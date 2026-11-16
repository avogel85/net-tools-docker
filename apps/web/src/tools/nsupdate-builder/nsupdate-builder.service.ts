export const TSIG_ALGORITHMS = ['hmac-sha256', 'hmac-sha512', 'hmac-sha384', 'hmac-sha224', 'hmac-sha1', 'hmac-md5'] as const
export type TsigAlgorithm = typeof TSIG_ALGORITHMS[number]

/** Secret length = the HMAC's output size (what tsig-keygen uses). */
const KEY_BYTES: Record<TsigAlgorithm, number> = { 'hmac-sha256': 32, 'hmac-sha512': 64, 'hmac-sha384': 48, 'hmac-sha224': 28, 'hmac-sha1': 20, 'hmac-md5': 16 }

export function generateSecret(alg: TsigAlgorithm): string {
  const bytes = globalThis.crypto.getRandomValues(new Uint8Array(KEY_BYTES[alg]))
  return btoa(String.fromCharCode(...bytes))
}

export function isValidSecret(s: string): boolean {
  if (!/^[A-Z0-9+/]+={0,2}$/i.test(s) || s.length % 4 !== 0)
    return false
  try {
    return atob(s).length >= 8
  }
  catch {
    return false
  }
}

const NAME = /^(?:\*\.)?(?:\w(?:[\w-]{0,61}[a-z0-9])?\.)*\w(?:[\w-]{0,61}[a-z0-9])?\.?$/i
export const isValidName = (s: string) => s === '@' || NAME.test(s)

export interface TsigKey { name: string, algorithm: TsigAlgorithm, secret: string }

export function keyConfigs(k: TsigKey): Record<'bind' | 'knot' | 'powerdns' | 'cli', string> {
  const name = k.name.replace(/\.$/, '')
  return {
    bind: `key "${name}" {\n    algorithm ${k.algorithm};\n    secret "${k.secret}";\n};\n`,
    knot: `key:\n  - id: ${name}\n    algorithm: ${k.algorithm}\n    secret: ${k.secret}\n`,
    powerdns: `pdnsutil import-tsig-key ${name} ${k.algorithm} ${k.secret}\n`,
    cli: `nsupdate -y ${k.algorithm}:${name}:${k.secret}\n`,
  }
}

export type Action = 'add' | 'delete' | 'replace'
export interface Operation { action: Action, name: string, ttl?: number, type?: string, data?: string }
export interface ParsedLine { line: number, op?: Operation, error?: string }

const TYPE = /^[A-Z][A-Z0-9]{0,9}$/

/**
 * Parses one update per line: `add <name> <ttl> <type> <data>`, `replace <name> <ttl> <type> <data>`,
 * `delete <name> [type [data]]`. Empty lines and `#` comments are skipped.
 */
export function parseOperations(text: string): ParsedLine[] {
  const out: ParsedLine[] = []
  text.split('\n').forEach((raw, i) => {
    const line = raw.trim()
    if (!line || line.startsWith('#'))
      return
    const [action, name, ...rest] = line.split(/\s+/)
    const a = action?.toLowerCase()
    if (a !== 'add' && a !== 'delete' && a !== 'replace')
      return out.push({ line: i + 1, error: 'action' })
    if (!name || !isValidName(name))
      return out.push({ line: i + 1, error: 'name' })
    if (a === 'delete') {
      const [type, ...data] = rest
      if (type && !TYPE.test(type.toUpperCase()))
        return out.push({ line: i + 1, error: 'type' })
      return out.push({ line: i + 1, op: { action: a, name, type: type?.toUpperCase(), data: data.join(' ') || undefined } })
    }
    const [ttl, type, ...data] = rest
    const n = Number(ttl)
    if (!Number.isInteger(n) || n < 0 || n > 2147483647)
      return out.push({ line: i + 1, error: 'ttl' })
    if (!type || !TYPE.test(type.toUpperCase()))
      return out.push({ line: i + 1, error: 'type' })
    if (!data.length)
      return out.push({ line: i + 1, error: 'data' })
    return out.push({ line: i + 1, op: { action: a, name, ttl: n, type: type.toUpperCase(), data: data.join(' ') } })
  })
  return out
}

/** Relative names get the zone appended; "@" is the zone itself. */
export function absolute(name: string, zone: string): string {
  const z = zone.replace(/\.$/, '')
  if (name === '@')
    return `${z}.`
  if (name.endsWith('.'))
    return name
  return z ? `${name}.${z}.` : `${name}.`
}

export interface ScriptInput { server: string, port: number, zone: string, ops: Operation[], showAnswer: boolean }

export function buildScript(i: ScriptInput): string {
  const lines: string[] = []
  if (i.server)
    lines.push(`server ${i.server}${i.port && i.port !== 53 ? ` ${i.port}` : ''}`)
  if (i.zone)
    lines.push(`zone ${i.zone.replace(/\.$/, '')}.`)
  for (const op of i.ops) {
    const name = absolute(op.name, i.zone)
    if (op.action === 'delete') {
      lines.push(['update delete', name, op.type, op.data].filter(Boolean).join(' '))
    }
    else {
      if (op.action === 'replace')
        lines.push(`update delete ${name} ${op.type}`)
      lines.push(`update add ${name} ${op.ttl} ${op.type} ${op.data}`)
    }
  }
  if (i.showAnswer)
    lines.push('show')
  lines.push('send')
  if (i.showAnswer)
    lines.push('answer')
  return `${lines.join('\n')}\n`
}
