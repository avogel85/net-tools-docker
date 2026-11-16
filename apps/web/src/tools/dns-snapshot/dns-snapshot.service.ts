export interface SnapRecord { name: string, type: string, ttl: number, data: string }
export interface Snapshot {
  id: string
  domain: string
  takenAt: string
  resolver: string
  records: SnapRecord[]
  /** Lookups that failed or returned an error code other than NOERROR/NXDOMAIN. */
  errors: { name: string, type: string, rcode: string }[]
}

export interface Query { name: string, type: string }

const APEX_TYPES = ['A', 'AAAA', 'MX', 'NS', 'TXT', 'SOA', 'CAA', 'HTTPS']
const SUBNAMES: [string, string[]][] = [
  ['www', ['A', 'AAAA', 'CNAME']],
  ['mail', ['A', 'AAAA', 'CNAME']],
  ['autodiscover', ['CNAME', 'A']],
  ['_dmarc', ['TXT']],
  ['_mta-sts', ['TXT']],
]
export const MAX_EXTRA = 10

/** The lookups a snapshot makes: apex types, common names, then extra names (A, AAAA, CNAME, TXT). */
export function plan(domain: string, extra: string[]): Query[] {
  const d = domain.toLowerCase().replace(/\.$/, '')
  const q: Query[] = APEX_TYPES.map(type => ({ name: d, type }))
  for (const [sub, types] of SUBNAMES) q.push(...types.map(type => ({ name: `${sub}.${d}`, type })))
  for (const raw of extra.slice(0, MAX_EXTRA)) {
    const n = raw.toLowerCase().replace(/\.$/, '')
    const name = n === d || n.endsWith(`.${d}`) ? n : `${n}.${d}`
    if (!q.some(x => x.name === name))
      q.push(...['A', 'AAAA', 'CNAME', 'TXT'].map(type => ({ name, type })))
  }
  return q
}

const norm = (name: string) => name.toLowerCase().replace(/\.$/, '')

/** Keeps answer records once (A queries also return the CNAME chain) in a stable order. */
export function collect(answers: SnapRecord[]): SnapRecord[] {
  const seen = new Set<string>()
  const out: SnapRecord[] = []
  for (const a of answers) {
    const r = { ...a, name: norm(a.name) }
    const k = `${r.name} ${r.type} ${r.data}`
    if (!seen.has(k)) {
      seen.add(k)
      out.push(r)
    }
  }
  return out.sort((a, b) => a.name.localeCompare(b.name) || a.type.localeCompare(b.type) || a.data.localeCompare(b.data))
}

export interface DiffOptions { ttl: boolean, serial: boolean }
export interface RRsetChange { name: string, type: string, before: string[], after: string[] }
export interface Diff { added: RRsetChange[], removed: RRsetChange[], changed: RRsetChange[], same: number }

function key(r: SnapRecord, o: DiffOptions): string {
  let data = r.data
  if (r.type === 'SOA' && !o.serial) {
    const f = data.split(/\s+/)
    f[2] = '*'
    data = f.join(' ')
  }
  return o.ttl ? `${r.ttl} ${data}` : data
}

/** Compares two snapshots per RRset (name + type). */
export function diff(a: Snapshot, b: Snapshot, o: DiffOptions = { ttl: false, serial: false }): Diff {
  const group = (s: Snapshot) => {
    const m = new Map<string, string[]>()
    for (const r of s.records) {
      const k = `${r.name}\t${r.type}`
      m.set(k, [...(m.get(k) ?? []), key(r, o)])
    }
    for (const [k, v] of m) m.set(k, [...new Set(v)].sort())
    return m
  }
  const ga = group(a)
  const gb = group(b)
  const out: Diff = { added: [], removed: [], changed: [], same: 0 }
  for (const k of [...new Set([...ga.keys(), ...gb.keys()])].sort()) {
    const [name, type] = k.split('\t') as [string, string]
    const before = ga.get(k) ?? []
    const after = gb.get(k) ?? []
    if (!before.length)
      out.added.push({ name, type, before, after })
    else if (!after.length)
      out.removed.push({ name, type, before, after })
    else if (before.join('\n') !== after.join('\n'))
      out.changed.push({ name, type, before, after })
    else
      out.same++
  }
  return out
}

// Browser storage, per viewer; failures (private mode, quota) are ignored.
const KEY = 'nettools.dns-snapshots'
export const MAX_SNAPSHOTS = 30

export function loadSnapshots(storage: Pick<Storage, 'getItem'> | undefined = globalThis.localStorage): Snapshot[] {
  try {
    const v = JSON.parse(storage?.getItem(KEY) ?? '[]') as unknown
    return Array.isArray(v) ? (v as Snapshot[]).filter(s => s && typeof s.domain === 'string' && Array.isArray(s.records)) : []
  }
  catch {
    return []
  }
}

export function saveSnapshots(list: Snapshot[], storage: Pick<Storage, 'setItem'> | undefined = globalThis.localStorage): boolean {
  try {
    storage?.setItem(KEY, JSON.stringify(list.slice(0, MAX_SNAPSHOTS)))
    return true
  }
  catch {
    return false
  }
}

/** Parses an exported snapshot file. */
export function parseSnapshot(text: string): Snapshot | undefined {
  try {
    const s = JSON.parse(text) as Snapshot
    if (typeof s.domain !== 'string' || !Array.isArray(s.records) || !s.records.every(r => typeof r.name === 'string' && typeof r.type === 'string' && typeof r.data === 'string'))
      return undefined
    return { id: s.id || `import-${Date.now()}`, domain: s.domain, takenAt: s.takenAt || new Date().toISOString(), resolver: s.resolver || '', records: collect(s.records.map(r => ({ ...r, ttl: Number(r.ttl) || 0 }))), errors: Array.isArray(s.errors) ? s.errors : [] }
  }
  catch {
    return undefined
  }
}
