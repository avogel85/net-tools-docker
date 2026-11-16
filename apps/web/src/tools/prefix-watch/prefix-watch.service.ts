import type { components } from '@/api/schema'
import { parseCidr } from '@/utils/ip'

export type PrefixStatus = components['schemas']['PrefixStatus']
export type RpkiResult = components['schemas']['RpkiResult']

export interface Watch { prefix: string, origins: number[] }
export interface Check {
  prefix: string
  checkedAt: string
  announced: boolean
  origins: number[]
  /** More-specifics with their origin. */
  more: { prefix: string, origin: number }[]
  /** RPKI status per expected origin. */
  rpki: Record<number, string>
  error: string
}
export interface Store { watches: Watch[], checks: Check[] }

export const MAX_WATCHES = 25
export const PER_RUN = 4
export const FRESH_MS = 6 * 3600 * 1000
const KEY = 'nettools.prefix-watch'

/** "192.0.2.0/24 AS64500 AS64501" or "2001:db8::/32 64500" per line. */
export function parseWatches(text: string): { watches: Watch[], invalid: string[] } {
  const watches: Watch[] = []
  const invalid: string[] = []
  for (const raw of text.split('\n').map(l => l.trim()).filter(Boolean)) {
    const [p, ...rest] = raw.split(/[\s,]+/)
    const c = parseCidr(p ?? '', { allowBare: false })
    const origins = rest.map(a => Number(a.replace(/^AS/i, ''))).filter(n => Number.isInteger(n) && n > 0 && n < 4294967296)
    if (!c || !origins.length || origins.length !== rest.length) {
      invalid.push(raw)
      continue
    }
    watches.push({ prefix: p!.toLowerCase(), origins: [...new Set(origins)] })
  }
  return { watches, invalid }
}

export function due(watches: Watch[], checks: Check[], now = Date.now()): Watch[] {
  const last = (p: string) => Date.parse(checks.find(c => c.prefix === p)?.checkedAt ?? '') || 0
  return watches.filter(w => now - last(w.prefix) > FRESH_MS).sort((a, b) => last(a.prefix) - last(b.prefix)).slice(0, PER_RUN)
}

export type Finding = 'not-announced' | 'unexpected-origin' | 'moas' | 'foreign-more-specific' | 'more-specific' | 'rpki-invalid' | 'rpki-unknown' | 'error'
const SEVERE: Finding[] = ['unexpected-origin', 'foreign-more-specific', 'rpki-invalid']

export function findings(w: Watch, c: Check): Finding[] {
  if (c.error)
    return ['error']
  const f: Finding[] = []
  if (!c.announced)
    f.push('not-announced')
  if (c.origins.some(o => !w.origins.includes(o)))
    f.push('unexpected-origin')
  if (c.origins.length > 1)
    f.push('moas')
  if (c.more.some(m => !w.origins.includes(m.origin)))
    f.push('foreign-more-specific')
  else if (c.more.length)
    f.push('more-specific')
  const statuses = Object.values(c.rpki)
  if (statuses.some(s => s.startsWith('invalid')))
    f.push('rpki-invalid')
  else if (statuses.includes('unknown'))
    f.push('rpki-unknown')
  return f
}

export function level(fs: Finding[]): 'alert' | 'warn' | 'ok' {
  if (fs.some(f => SEVERE.includes(f)))
    return 'alert'
  return fs.some(f => f !== 'more-specific') ? 'warn' : 'ok'
}

export function load(storage: Pick<Storage, 'getItem'> | undefined = globalThis.localStorage): Store {
  try {
    const s = JSON.parse(storage?.getItem(KEY) ?? 'null') as Partial<Store> | null
    return { watches: Array.isArray(s?.watches) ? s.watches.slice(0, MAX_WATCHES) : [], checks: Array.isArray(s?.checks) ? s.checks : [] }
  }
  catch {
    return { watches: [], checks: [] }
  }
}

export function save(s: Store, storage: Pick<Storage, 'setItem'> | undefined = globalThis.localStorage): boolean {
  try {
    storage?.setItem(KEY, JSON.stringify(s))
    return true
  }
  catch {
    return false
  }
}
