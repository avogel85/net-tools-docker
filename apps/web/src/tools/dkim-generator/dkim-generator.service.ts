export type KeyKind = 'rsa-2048' | 'rsa-1024' | 'rsa-4096' | 'ed25519'

export interface DkimKeys {
  kind: KeyKind
  /** The p= value: base64 SubjectPublicKeyInfo for RSA, the raw 32-byte key for Ed25519 (RFC 8463). */
  publicKey: string
  privatePem: string
}

export interface DkimRecordInput {
  selector: string
  domain: string
  kind: KeyKind
  publicKey: string
  /** t=y: receivers treat failures as if unsigned. */
  testing: boolean
  /** t=s: the i= domain must equal d= (no subdomains). */
  strict: boolean
}

export function toBase64(buf: ArrayBuffer): string {
  let s = ''
  for (const b of new Uint8Array(buf))
    s += String.fromCharCode(b)
  return btoa(s)
}

export function pem(der: ArrayBuffer, label: string): string {
  const lines = toBase64(der).match(/.{1,64}/g) ?? []
  return `-----BEGIN ${label}-----\n${lines.join('\n')}\n-----END ${label}-----\n`
}

/** Generates a DKIM key pair in the browser; the private key never leaves it. */
export async function generateKeys(kind: KeyKind, subtle: SubtleCrypto = globalThis.crypto.subtle): Promise<DkimKeys> {
  if (kind === 'ed25519') {
    const pair = await subtle.generateKey({ name: 'Ed25519' }, true, ['sign', 'verify']) as CryptoKeyPair
    const raw = await subtle.exportKey('raw', pair.publicKey)
    return { kind, publicKey: toBase64(raw), privatePem: pem(await subtle.exportKey('pkcs8', pair.privateKey), 'PRIVATE KEY') }
  }
  const bits = Number(kind.slice(4))
  const pair = await subtle.generateKey({ name: 'RSASSA-PKCS1-v1_5', modulusLength: bits, publicExponent: new Uint8Array([1, 0, 1]), hash: 'SHA-256' }, true, ['sign', 'verify'])
  return {
    kind,
    publicKey: toBase64(await subtle.exportKey('spki', pair.publicKey)),
    privatePem: pem(await subtle.exportKey('pkcs8', pair.privateKey), 'PRIVATE KEY'),
  }
}

/** A DNS label (selectors may contain dots, e.g. "s1.mail", per RFC 6376 §3.1). */
export function isValidSelector(s: string): boolean {
  return /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$/i.test(s)
}

export function isValidDomain(d: string): boolean {
  return /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{0,62}$/i.test(d.replace(/\.$/, ''))
}

export function buildDkimRecord(i: DkimRecordInput): { name: string, value: string } {
  const flags = [i.testing && 'y', i.strict && 's'].filter(Boolean).join(':')
  const tags = ['v=DKIM1', `k=${i.kind === 'ed25519' ? 'ed25519' : 'rsa'}`]
  if (flags)
    tags.push(`t=${flags}`)
  tags.push(`p=${i.publicKey}`)
  return { name: `${i.selector.toLowerCase()}._domainkey.${i.domain.toLowerCase().replace(/\.$/, '')}`, value: tags.join('; ') }
}
