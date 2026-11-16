export type Kind = 'cert' | 'domain'

export interface Entry { target: string, kinds: Kind[] }

export interface Check {
  target: string
  kind: Kind
  checkedAt: string
  /** ISO date of expiry, '' when unknown. */
  expires: string
  daysLeft: number | null
  /** Issuer (certificate) or registrar (domain). */
  by: string
  error: string
}

export interface Store { entries: Entry[], checks: Check[], warnDays: number }

const KEY = 'nettools.expiry-watchlist'
export const MAX_ENTRIES = 50
export const DOMAINS_PER_RUN = 8
export const DOMAIN_FRESH_MS = 12 * 3600 * 1000

const HOSTPORT = /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]*(?::\d{1,5})?$/i

/** example.com or example.co.uk: registration data exists only for registrable domains. */
export function looksRegistrable(name: string): boolean {
  const l = name.split('.')
  return l.length === 2 || (l.length === 3 && /^(?:co|com|net|org|ac|gov|or|ne|edu)$/.test(l[1]!))
}

/**
 * Parses "host", "host:port", "domain:example.com" or "cert:host" lines. A bare registrable name
 * is watched for both; a subdomain or host:port only for the certificate.
 */
export function parseEntries(text: string): { entries: Entry[], invalid: string[] } {
  const entries: Entry[] = []
  const invalid: string[] = []
  for (const raw of text.split(/[\n,;]+/).map(s => s.trim().toLowerCase()).filter(Boolean)) {
    let kinds: Kind[] = ['cert', 'domain']
    let t = raw.replace(/^https?:\/\//, '').replace(/\/.*$/, '')
    const m = /^(cert|domain):(.+)$/.exec(t)
    if (m) {
      kinds = [m[1] as Kind]
      t = m[2]!
    }
    if (t.includes(':') || (!m && !looksRegistrable(t)))
      kinds = kinds.filter(k => k === 'cert')
    if (!HOSTPORT.test(t) || !kinds.length) {
      invalid.push(raw)
      continue
    }
    const existing = entries.find(e => e.target === t)
    if (existing)
      existing.kinds = [...new Set([...existing.kinds, ...kinds])]
    else
      entries.push({ target: t, kinds })
  }
  return { entries, invalid }
}

export function mergeEntries(a: Entry[], b: Entry[]): Entry[] {
  const out = a.map(e => ({ ...e, kinds: [...e.kinds] }))
  for (const e of b) {
    const x = out.find(o => o.target === e.target)
    if (x)
      x.kinds = [...new Set([...x.kinds, ...e.kinds])]
    else
      out.push(e)
  }
  return out.slice(0, MAX_ENTRIES)
}

/** Domains to refresh this run: never or longest ago checked first, skipping fresh ones. */
export function domainsToCheck(entries: Entry[], checks: Check[], now = Date.now()): string[] {
  const last = (t: string) => Date.parse(checks.find(c => c.kind === 'domain' && c.target === t)?.checkedAt ?? '') || 0
  return entries.filter(e => e.kinds.includes('domain'))
    .map(e => e.target)
    .filter(t => now - last(t) > DOMAIN_FRESH_MS)
    .sort((a, b) => last(a) - last(b))
    .slice(0, DOMAINS_PER_RUN)
}

/** Replaces checks of the same target and kind. */
export function upsertChecks(checks: Check[], fresh: Check[]): Check[] {
  const key = (c: Check) => `${c.kind} ${c.target}`
  const keys = new Set(fresh.map(key))
  return [...checks.filter(c => !keys.has(key(c))), ...fresh]
}

export type Level = 'expired' | 'soon' | 'ok' | 'unknown'
export function level(c: Check, warnDays: number): Level {
  if (c.error || c.daysLeft === null)
    return 'unknown'
  if (c.daysLeft < 0)
    return 'expired'
  return c.daysLeft <= warnDays ? 'soon' : 'ok'
}

/** Rows for the entries that still exist, most urgent first. */
export function rows(store: Store): Check[] {
  const watched = new Set(store.entries.flatMap(e => e.kinds.map(k => `${k} ${e.target}`)))
  const order: Record<Level, number> = { expired: 0, soon: 1, unknown: 2, ok: 3 }
  return store.checks.filter(c => watched.has(`${c.kind} ${c.target}`))
    .sort((a, b) => order[level(a, store.warnDays)] - order[level(b, store.warnDays)] || (a.daysLeft ?? 1e9) - (b.daysLeft ?? 1e9))
}

export function load(storage: Pick<Storage, 'getItem'> | undefined = globalThis.localStorage): Store {
  try {
    const s = JSON.parse(storage?.getItem(KEY) ?? 'null') as Partial<Store> | null
    return { entries: Array.isArray(s?.entries) ? s.entries.slice(0, MAX_ENTRIES) : [], checks: Array.isArray(s?.checks) ? s.checks : [], warnDays: typeof s?.warnDays === 'number' ? s.warnDays : 30 }
  }
  catch {
    return { entries: [], checks: [], warnDays: 30 }
  }
}

export function save(store: Store, storage: Pick<Storage, 'setItem'> | undefined = globalThis.localStorage): boolean {
  try {
    storage?.setItem(KEY, JSON.stringify(store))
    return true
  }
  catch {
    return false
  }
}
