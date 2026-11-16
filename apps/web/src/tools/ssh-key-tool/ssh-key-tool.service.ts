import { md5 } from './md5'

export interface SshKey {
  line: number
  type: string
  /** Bits for RSA/DSA, curve size for ECDSA, 256 for Ed25519. */
  bits: number
  comment: string
  /** Host names from a known_hosts line (hashed entries show as "(hashed)"). */
  hosts: string[]
  blob: Uint8Array<ArrayBuffer>
}

export interface Fingerprints {
  sha256: string
  md5: string
  /** SSHFP records (algorithm, then SHA-1 and SHA-256 fingerprints), undefined when no SSHFP algorithm exists. */
  sshfp?: { algorithm: number, sha1: string, sha256: string }
}

const TYPES: Record<string, { sshfp?: number }> = {
  'ssh-rsa': { sshfp: 1 },
  'ssh-dss': { sshfp: 2 },
  'ecdsa-sha2-nistp256': { sshfp: 3 },
  'ecdsa-sha2-nistp384': { sshfp: 3 },
  'ecdsa-sha2-nistp521': { sshfp: 3 },
  'ssh-ed25519': { sshfp: 4 },
  'ssh-ed448': { sshfp: 6 },
  'sk-ssh-ed25519@openssh.com': {},
  'sk-ecdsa-sha2-nistp256@openssh.com': {},
}

function b64decode(s: string): Uint8Array<ArrayBuffer> | undefined {
  try {
    return Uint8Array.from(atob(s), c => c.charCodeAt(0))
  }
  catch {
    return undefined
  }
}

/** Reads the SSH wire strings (uint32 length + bytes) of a key blob. */
function readStrings(blob: Uint8Array, max = 6): Uint8Array[] | undefined {
  const out: Uint8Array[] = []
  const view = new DataView(blob.buffer, blob.byteOffset, blob.byteLength)
  let i = 0
  while (i < blob.length && out.length < max) {
    if (i + 4 > blob.length)
      return undefined
    const n = view.getUint32(i)
    if (i + 4 + n > blob.length)
      return undefined
    out.push(blob.subarray(i + 4, i + 4 + n))
    i += 4 + n
  }
  return out
}

/** Bit length of an SSH mpint (big-endian, possibly with a leading 0 byte). */
function mpintBits(b: Uint8Array): number {
  let i = 0
  while (i < b.length && b[i] === 0) i++
  if (i === b.length)
    return 0
  return (b.length - i - 1) * 8 + (32 - Math.clz32(b[i]!))
}

function keyBits(type: string, parts: Uint8Array[]): number {
  if (type === 'ssh-rsa')
    return mpintBits(parts[2] ?? new Uint8Array())
  if (type === 'ssh-dss')
    return mpintBits(parts[1] ?? new Uint8Array())
  const curve = /nistp(\d+)/.exec(type)
  if (curve)
    return Number(curve[1])
  return type.includes('ed448') ? 448 : 256
}

// authorized_keys options (sshd(8) "AUTHORIZED_KEYS FILE FORMAT").
const OPTIONS = new Set(['agent-forwarding', 'cert-authority', 'command', 'environment', 'expiry-time', 'from', 'no-agent-forwarding', 'no-port-forwarding', 'no-pty', 'no-user-rc', 'no-x11-forwarding', 'permitlisten', 'permitopen', 'port-forwarding', 'principals', 'pty', 'no-touch-required', 'restrict', 'tunnel', 'user-rc', 'verify-required', 'x11-forwarding'])
function isOptions(token: string): boolean {
  const items = token.match(/(?:[^,"]|"[^"]*")+/g) ?? []
  return items.length > 0 && items.every(i => OPTIONS.has(i.split('=')[0]!.toLowerCase()))
}

/** Parses .pub, authorized_keys (with options) and known_hosts lines. */
export function parseKeys(text: string): { keys: SshKey[], errors: number[] } {
  const keys: SshKey[] = []
  const errors: number[] = []
  text.split('\n').forEach((raw, idx) => {
    const line = raw.trim()
    if (!line || line.startsWith('#'))
      return
    const tokens = line.match(/(?:[^\s"]|"[^"]*")+/g) ?? []
    const at = tokens.findIndex(t => t in TYPES)
    const blob = at >= 0 ? b64decode(tokens[at + 1] ?? '') : undefined
    const parts = blob && readStrings(blob)
    const type = tokens[at]
    if (!blob || !parts || !type || new TextDecoder().decode(parts[0]) !== type) {
      errors.push(idx + 1)
      return
    }
    // Before the type: authorized_keys options, or known_hosts host patterns.
    // known_hosts may start with a marker (@cert-authority, @revoked).
    const before = tokens.slice(0, at).filter(t => !t.startsWith('@'))
    const hostField = before.find(t => !isOptions(t))
    const hosts = hostField ? hostField.split(',').map(h => (h.startsWith('|1|') ? '(hashed)' : h)) : []
    keys.push({ line: idx + 1, type, bits: keyBits(type, parts), comment: tokens.slice(at + 2).join(' '), hosts, blob })
  })
  return { keys, errors }
}

const hex = (b: ArrayBuffer | Uint8Array) => [...new Uint8Array(b)].map(x => x.toString(16).padStart(2, '0')).join('')

export async function fingerprints(key: SshKey, subtle: SubtleCrypto = globalThis.crypto.subtle): Promise<Fingerprints> {
  const sha256 = await subtle.digest('SHA-256', key.blob)
  const out: Fingerprints = {
    sha256: `SHA256:${btoa(String.fromCharCode(...new Uint8Array(sha256))).replace(/=+$/, '')}`,
    md5: `MD5:${hex(md5(key.blob)).match(/../g)!.join(':')}`,
  }
  const alg = TYPES[key.type]?.sshfp
  if (alg)
    out.sshfp = { algorithm: alg, sha1: hex(await subtle.digest('SHA-1', key.blob)), sha256: hex(sha256) }
  return out
}

export function keyWarnings(k: SshKey): string[] {
  const w: string[] = []
  if (k.type === 'ssh-dss')
    w.push('dsa')
  else if (k.type === 'ssh-rsa' && k.bits < 2048)
    w.push('rsa-short')
  else if (k.type === 'ssh-rsa' && k.bits < 3072)
    w.push('rsa-2048')
  return w
}
