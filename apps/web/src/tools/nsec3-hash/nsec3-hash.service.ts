import { nameWire } from '../ds-dnskey/ds-dnskey.service'

/** RFC 9276 caps validators at about 100 iterations; we refuse more to keep the browser responsive. */
export const MAX_ITERATIONS = 2500

const B32HEX = '0123456789abcdefghijklmnopqrstuv'

/** Base32 with the extended hex alphabet, no padding (RFC 4648 §7, as NSEC3 owner names use). */
export function base32hex(bytes: Uint8Array): string {
  let out = ''
  let bits = 0
  let value = 0
  for (const b of bytes) {
    value = (value << 8) | b
    bits += 8
    while (bits >= 5) {
      out += B32HEX[(value >>> (bits - 5)) & 31]
      bits -= 5
    }
  }
  if (bits > 0)
    out += B32HEX[(value << (5 - bits)) & 31]
  return out
}

/** Parses the salt as NSEC3PARAM shows it: hex, or "-" for none. */
export function parseSalt(s: string): Uint8Array | undefined {
  const v = s.trim()
  if (v === '' || v === '-')
    return new Uint8Array()
  if (!/^(?:[0-9a-f]{2})+$/i.test(v))
    return undefined
  return Uint8Array.from(v.match(/../g)!.map(h => Number.parseInt(h, 16)))
}

/** NSEC3 hash: IH(0) = SHA-1(name | salt), IH(k) = SHA-1(IH(k-1) | salt) (RFC 5155 §5). */
export async function nsec3Hash(name: string, salt: Uint8Array, iterations: number, subtle: SubtleCrypto = globalThis.crypto.subtle): Promise<string> {
  let x = nameWire(name)
  for (let i = 0; i <= iterations; i++) {
    const data = new Uint8Array(x.length + salt.length)
    data.set(x)
    data.set(salt, x.length)
    x = new Uint8Array(await subtle.digest('SHA-1', data))
  }
  return base32hex(x)
}

export interface Nsec3Param { algorithm: number, flags: number, iterations: number, salt: string }

/** Reads "1 0 10 AABBCCDD" (NSEC3PARAM RDATA). */
export function parseNsec3Param(data: string): Nsec3Param | undefined {
  const m = /^(\d+)\s+(\d+)\s+(\d+)\s+(\S+)$/.exec(data.trim())
  return m ? { algorithm: Number(m[1]), flags: Number(m[2]), iterations: Number(m[3]), salt: m[4]! } : undefined
}

/** RFC 9276 advice: 0 extra iterations and no salt. */
export function paramWarnings(iterations: number, salt: Uint8Array): string[] {
  const w: string[] = []
  if (iterations > 0)
    w.push(iterations > 100 ? 'iterations-high' : 'iterations')
  if (salt.length)
    w.push('salt')
  return w
}
