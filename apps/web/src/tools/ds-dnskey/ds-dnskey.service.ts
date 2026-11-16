export interface Dnskey {
  flags: number
  protocol: number
  algorithm: number
  publicKey: Uint8Array
  keyTag: number
}

export interface Ds {
  keyTag: number
  algorithm: number
  digestType: number
  digest: string
}

export const ALGORITHMS: Record<number, string> = {
  5: 'RSASHA1',
  7: 'RSASHA1-NSEC3-SHA1',
  8: 'RSASHA256',
  10: 'RSASHA512',
  13: 'ECDSAP256SHA256',
  14: 'ECDSAP384SHA384',
  15: 'ED25519',
  16: 'ED448',
}
export const DIGESTS: Record<number, { name: string, hash: string }> = {
  1: { name: 'SHA-1', hash: 'SHA-1' },
  2: { name: 'SHA-256', hash: 'SHA-256' },
  4: { name: 'SHA-384', hash: 'SHA-384' },
}

function fromBase64(s: string): Uint8Array | undefined {
  try {
    return Uint8Array.from(atob(s.replace(/\s+/g, '')), c => c.charCodeAt(0))
  }
  catch {
    return undefined
  }
}

/** DNSKEY RDATA in wire format: flags (2), protocol (1), algorithm (1), public key. */
export function dnskeyRdata(k: Omit<Dnskey, 'keyTag'>): Uint8Array {
  const out = new Uint8Array(4 + k.publicKey.length)
  out[0] = k.flags >> 8
  out[1] = k.flags & 0xFF
  out[2] = k.protocol
  out[3] = k.algorithm
  out.set(k.publicKey, 4)
  return out
}

/** RFC 4034 Appendix B key tag (algorithm 1 is obsolete and not handled specially). */
export function keyTag(rdata: Uint8Array): number {
  let ac = 0
  for (let i = 0; i < rdata.length; i++)
    ac += i & 1 ? rdata[i]! : rdata[i]! << 8
  ac += (ac >> 16) & 0xFFFF
  return ac & 0xFFFF
}

/**
 * Parses DNSKEY records: zone-file lines ("example.com. 3600 IN DNSKEY 257 3 13 …") or bare RDATA
 * ("257 3 13 …"). Returns the owner name found in the text (if any) with each key.
 */
export function parseDnskeys(text: string): { owner?: string, key: Dnskey }[] | undefined {
  const out: { owner?: string, key: Dnskey }[] = []
  // Join zone-file parentheses so a multi-line record becomes one line.
  const flat = text.replace(/;[^\n]*/g, '').replace(/\(([^)]*)\)/g, (_, inner: string) => inner.replace(/\s+/g, ' '))
  for (const line of flat.split('\n').map(l => l.trim()).filter(Boolean)) {
    const m = /^(?:(\S+)\s+(?:\d+\s+)?(?:IN\s+)?DNSKEY\s+)?(\d+)\s+(\d+)\s+(\d+)\s+([A-Z0-9+/=][A-Z0-9+/= ]*)$/i.exec(line)
    if (!m)
      return undefined
    const publicKey = fromBase64(m[5]!)
    const flags = Number(m[2])
    if (!publicKey || !publicKey.length || flags > 0xFFFF)
      return undefined
    const k = { flags, protocol: Number(m[3]), algorithm: Number(m[4]), publicKey }
    out.push({ owner: m[1], key: { ...k, keyTag: keyTag(dnskeyRdata(k)) } })
  }
  return out.length ? out : undefined
}

export function parseDs(text: string): Ds[] {
  const out: Ds[] = []
  for (const line of text.split('\n')) {
    const m = /(?:^|\s)(\d{1,5})\s+(\d{1,3})\s+(\d)\s+([0-9a-f][0-9a-f ]{39,})$/i.exec(line.trim())
    if (m)
      out.push({ keyTag: Number(m[1]), algorithm: Number(m[2]), digestType: Number(m[3]), digest: m[4]!.replace(/\s+/g, '').toUpperCase() })
  }
  return out
}

/** Owner name in canonical wire format (lowercase labels, RFC 4034 §6.2). */
export function nameWire(name: string): Uint8Array {
  const labels = name.toLowerCase().replace(/\.$/, '').split('.').filter(Boolean)
  const enc = new TextEncoder()
  const parts = labels.map(l => enc.encode(l))
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length + 1, 1))
  let i = 0
  for (const p of parts) {
    out[i++] = p.length
    out.set(p, i)
    i += p.length
  }
  return out
}

function hex(buf: ArrayBuffer): string {
  return [...new Uint8Array(buf)].map(b => b.toString(16).padStart(2, '0')).join('').toUpperCase()
}

/** DS digest = hash(owner wire name | DNSKEY RDATA) (RFC 4034 §5.1.4). */
export async function computeDs(owner: string, key: Dnskey, digestType: number, subtle: SubtleCrypto = globalThis.crypto.subtle): Promise<Ds> {
  const wire = nameWire(owner)
  const rdata = dnskeyRdata(key)
  const data = new Uint8Array(wire.length + rdata.length)
  data.set(wire)
  data.set(rdata, wire.length)
  const digest = hex(await subtle.digest(DIGESTS[digestType]!.hash, data))
  return { keyTag: key.keyTag, algorithm: key.algorithm, digestType, digest }
}

export const dsText = (d: Ds) => `${d.keyTag} ${d.algorithm} ${d.digestType} ${d.digest}`

export type KeyRole = 'ksk' | 'zsk' | 'revoked'
export function keyRole(k: Dnskey): KeyRole {
  if (k.flags & 0x80)
    return 'revoked'
  return k.flags & 1 ? 'ksk' : 'zsk'
}

export interface Comparison {
  /** Each published DS with the key it points to (undefined: no such key in the zone). */
  ds: { ds: Ds, key?: Dnskey, match: boolean }[]
  /** KSKs that no DS points to (not yet linked, or being rolled). */
  unlinked: Dnskey[]
}

/** Matches published DS records against the zone's keys by recomputing each digest. */
export async function compare(owner: string, keys: Dnskey[], published: Ds[], subtle?: SubtleCrypto): Promise<Comparison> {
  const ds: Comparison['ds'] = []
  const linked = new Set<Dnskey>()
  for (const d of published) {
    let found: Dnskey | undefined
    let match = false
    for (const k of keys.filter(k => k.keyTag === d.keyTag && k.algorithm === d.algorithm)) {
      found = k
      if (DIGESTS[d.digestType] && (await computeDs(owner, k, d.digestType, subtle)).digest === d.digest) {
        match = true
        linked.add(k)
        break
      }
    }
    ds.push({ ds: d, key: found, match })
  }
  return { ds, unlinked: keys.filter(k => keyRole(k) === 'ksk' && !linked.has(k)) }
}
