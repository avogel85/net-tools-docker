import { parseIPv4, parseIPv6 } from '@/utils/ip'

export interface ZoneRecord {
  line: number
  /** Absolute owner name, lowercase, with trailing dot. */
  name: string
  ttl: number
  /** True when the TTL came from the line itself. */
  explicitTtl: boolean
  type: string
  /** RDATA as written, whitespace-normalised. */
  data: string
  /** $ORIGIN in effect for this record (relative names in the data are relative to it). */
  origin: string
}

export type Severity = 'error' | 'warning' | 'info'
export interface Finding {
  severity: Severity
  code: string
  line?: number
  params?: Record<string, string | number>
}

export interface LintResult {
  origin: string
  records: ZoneRecord[]
  findings: Finding[]
}

const CLASSES = new Set(['IN', 'CH', 'HS', 'CS'])
const KNOWN_TYPES = new Set(['A', 'AAAA', 'NS', 'CNAME', 'SOA', 'PTR', 'MX', 'TXT', 'SRV', 'CAA', 'DS', 'DNSKEY', 'RRSIG', 'NSEC', 'NSEC3', 'NSEC3PARAM', 'TLSA', 'SSHFP', 'HTTPS', 'SVCB', 'NAPTR', 'DNAME', 'HINFO', 'LOC', 'SPF', 'CDS', 'CDNSKEY', 'OPENPGPKEY', 'SMIMEA', 'URI', 'ZONEMD', 'CERT', 'RP', 'AFSDB'])
const DNSSEC_TYPES = new Set(['RRSIG', 'NSEC', 'NSEC3'])

/** "1h30m", "2d", "3600" → seconds; undefined when not a TTL. */
export function parseTtl(s: string): number | undefined {
  if (/^\d+$/.test(s))
    return Number(s)
  const m = s.toLowerCase().match(/^(?:\d+[wdhms])+$/)
  if (!m)
    return undefined
  const mult: Record<string, number> = { w: 604800, d: 86400, h: 3600, m: 60, s: 1 }
  let total = 0
  for (const [, n, u] of s.toLowerCase().matchAll(/(\d+)([wdhms])/g))
    total += Number(n) * mult[u!]!
  return total
}

/** Splits a line into tokens, keeping quoted strings together and dropping ; comments. */
function tokenize(line: string): string[] {
  const out: string[] = []
  let cur = ''
  let quoted = false
  for (let i = 0; i < line.length; i++) {
    const c = line[i]!
    if (quoted) {
      cur += c
      if (c === '\\' && i + 1 < line.length)
        cur += line[++i]
      else if (c === '"')
        quoted = false
      continue
    }
    if (c === ';')
      break
    if (c === '"') {
      quoted = true
      cur += c
    }
    else if (c === ' ' || c === '\t') {
      if (cur)
        out.push(cur)
      cur = ''
    }
    else {
      cur += c
    }
  }
  if (cur)
    out.push(cur)
  return out
}

/** Joins records that span lines with ( … ); returns [start line, text, first char was whitespace]. */
function logicalLines(text: string): { line: number, text: string, indented: boolean }[] {
  const out: { line: number, text: string, indented: boolean }[] = []
  let buf: { line: number, text: string, indented: boolean } | undefined
  let depth = 0
  text.split(/\r?\n/).forEach((raw, i) => {
    const toks = tokenize(raw).join(' ')
    if (!buf) {
      if (!toks)
        return
      buf = { line: i + 1, text: '', indented: /^[ \t]/.test(raw) }
    }
    for (const c of toks.replace(/"(?:[^"\\]|\\.)*"/g, ''))
      depth += c === '(' ? 1 : c === ')' ? -1 : 0
    buf.text += ` ${toks}`
    if (depth <= 0) {
      out.push({ ...buf, text: buf.text.trim() })
      buf = undefined
      depth = 0
    }
  })
  if (buf)
    out.push(buf)
  return out
}

function stripParens(tokens: string[]): string[] {
  return tokens.flatMap((t) => {
    if (t.startsWith('"'))
      return [t]
    const s = t.replace(/[()]/g, '')
    return s ? [s] : []
  })
}

export function absoluteName(name: string, origin: string): string {
  if (name === '@')
    return origin
  if (name.endsWith('.'))
    return name.toLowerCase()
  return `${name}.${origin === '.' ? '' : origin}`.toLowerCase()
}

function validName(name: string): boolean {
  if (name === '.')
    return true
  const labels = name.replace(/\.$/, '').split('.')
  return name.length <= 254 && labels.every(l => l.length > 0 && l.length <= 63)
}

/** Byte lengths of the quoted strings (or bare words) of TXT data. */
function txtStrings(data: string): number[] {
  const out: number[] = []
  for (const m of data.matchAll(/"((?:[^"\\]|\\.)*)"|(\S+)/g)) {
    const s = (m[1] ?? m[2] ?? '').replace(/\\(\d{3}|.)/g, 'x')
    out.push(new TextEncoder().encode(s).length)
  }
  return out
}

/** Parses a zone file into records; parse problems become findings. */
export function parseZone(text: string, defaultOrigin: string): LintResult {
  const findings: Finding[] = []
  const records: ZoneRecord[] = []
  let origin = defaultOrigin ? `${defaultOrigin.toLowerCase().replace(/\.$/, '')}.` : ''
  let defaultTtl: number | undefined
  let lastName: string | undefined
  let lastTtl: number | undefined
  let soaMinimum: number | undefined
  const pendingNoTtl: ZoneRecord[] = []

  for (const ll of logicalLines(text)) {
    const tokens = stripParens(tokenize(ll.text))
    if (!tokens.length)
      continue
    const head = tokens[0]!.toUpperCase()
    if (head === '$ORIGIN') {
      if (!tokens[1])
        findings.push({ severity: 'error', code: 'directive', line: ll.line })
      else
        origin = absoluteName(tokens[1], origin || '.')
      continue
    }
    if (head === '$TTL') {
      const t = tokens[1] ? parseTtl(tokens[1]) : undefined
      if (t === undefined)
        findings.push({ severity: 'error', code: 'bad-ttl', line: ll.line, params: { value: tokens[1] ?? '' } })
      else
        defaultTtl = t
      continue
    }
    if (head === '$INCLUDE' || head === '$GENERATE') {
      findings.push({ severity: 'info', code: 'directive-skipped', line: ll.line, params: { directive: head } })
      continue
    }
    if (head.startsWith('$')) {
      findings.push({ severity: 'error', code: 'directive', line: ll.line })
      continue
    }
    if (!origin) {
      findings.push({ severity: 'error', code: 'no-origin', line: ll.line })
      return { origin: '', records, findings }
    }

    let i = 0
    let name: string
    if (ll.indented) {
      if (!lastName) {
        findings.push({ severity: 'error', code: 'no-owner', line: ll.line })
        continue
      }
      name = lastName
    }
    else {
      name = absoluteName(tokens[0]!, origin)
      i = 1
    }
    let ttl: number | undefined
    // [ttl] [class] or [class] [ttl], in either order.
    for (let k = 0; k < 2 && i < tokens.length; k++) {
      const tok = tokens[i]!
      if (ttl === undefined && parseTtl(tok) !== undefined && !KNOWN_TYPES.has(tok.toUpperCase())) {
        ttl = parseTtl(tok)
        i++
      }
      else if (CLASSES.has(tok.toUpperCase())) {
        if (tok.toUpperCase() !== 'IN')
          findings.push({ severity: 'warning', code: 'class', line: ll.line, params: { class: tok.toUpperCase() } })
        i++
      }
    }
    const type = tokens[i]?.toUpperCase()
    const data = tokens.slice(i + 1).join(' ')
    if (!type || (!KNOWN_TYPES.has(type) && !/^TYPE\d+$/.test(type))) {
      findings.push({ severity: 'error', code: 'unknown-type', line: ll.line, params: { type: tokens[i] ?? '' } })
      continue
    }
    if (!data) {
      findings.push({ severity: 'error', code: 'no-data', line: ll.line, params: { type } })
      continue
    }
    if (!validName(name))
      findings.push({ severity: 'error', code: 'bad-name', line: ll.line, params: { name } })
    lastName = name
    const rec: ZoneRecord = { line: ll.line, name, ttl: ttl ?? defaultTtl ?? lastTtl ?? 0, explicitTtl: ttl !== undefined, type, data, origin }
    if (ttl !== undefined)
      lastTtl = ttl
    else if (defaultTtl === undefined && lastTtl === undefined)
      pendingNoTtl.push(rec)
    if (type === 'SOA') {
      const min = parseTtl(data.split(/\s+/)[6] ?? '')
      if (min !== undefined)
        soaMinimum = min
    }
    records.push(rec)
  }
  if (pendingNoTtl.length) {
    findings.push({ severity: 'warning', code: 'no-default-ttl', line: pendingNoTtl[0]!.line })
    for (const r of pendingNoTtl) r.ttl = soaMinimum ?? 0
  }
  return { origin, records, findings }
}

const inZone = (name: string, origin: string) => origin === '.' || name === origin || name.endsWith(`.${origin}`)

const order: Record<Severity, number> = { error: 0, warning: 1, info: 2 }

/** Runs the checks on a parsed zone. */
export function lintZone(text: string, defaultOrigin: string): LintResult {
  const res = parseZone(text, defaultOrigin)
  const { records, findings } = res
  if (!res.origin)
    return res
  // Use the apex as the zone: the SOA owner if present, else the origin.
  const soas = records.filter(r => r.type === 'SOA')
  const apex = soas[0]?.name ?? res.origin
  const byName = new Map<string, ZoneRecord[]>()
  for (const r of records) byName.set(r.name, [...(byName.get(r.name) ?? []), r])
  const has = (name: string, ...types: string[]) => (byName.get(name) ?? []).some(r => types.includes(r.type))
  const add = (f: Finding) => findings.push(f)

  // SOA
  if (!soas.length)
    add({ severity: 'error', code: 'no-soa' })
  if (soas.length > 1)
    add({ severity: 'error', code: 'multiple-soa', line: soas[1]!.line })
  for (const s of soas) {
    if (s.name !== apex)
      add({ severity: 'error', code: 'soa-not-apex', line: s.line })
    const f = s.data.split(/\s+/)
    const [, , serial, refresh, retry, expire, minimum] = [f[0], f[1], Number(f[2]), parseTtl(f[3] ?? ''), parseTtl(f[4] ?? ''), parseTtl(f[5] ?? ''), parseTtl(f[6] ?? '')]
    if (f.length !== 7 || [refresh, retry, expire, minimum].includes(undefined) || !Number.isInteger(serial)) {
      add({ severity: 'error', code: 'soa-syntax', line: s.line })
      continue
    }
    if (serial > 4294967295)
      add({ severity: 'error', code: 'soa-serial', line: s.line })
    if (retry! >= refresh!)
      add({ severity: 'warning', code: 'soa-retry', line: s.line })
    if (expire! < refresh! * 7 || expire! < 604800)
      add({ severity: 'warning', code: 'soa-expire', line: s.line, params: { value: expire! } })
    if (minimum! > 86400)
      add({ severity: 'warning', code: 'soa-minimum', line: s.line, params: { value: minimum! } })
    if (refresh! < 1200)
      add({ severity: 'info', code: 'soa-refresh', line: s.line, params: { value: refresh! } })
  }

  // Apex NS
  const apexNs = (byName.get(apex) ?? []).filter(r => r.type === 'NS')
  if (!apexNs.length)
    add({ severity: 'error', code: 'no-ns' })
  else if (apexNs.length < 2)
    add({ severity: 'warning', code: 'one-ns', line: apexNs[0]!.line })

  // Delegations (NS below the apex) and occluded data beneath them.
  const cuts = [...byName.keys()].filter(n => n !== apex && has(n, 'NS'))
  const underCut = (name: string) => cuts.find(c => name !== c && name.endsWith(`.${c}`))

  const seen = new Map<string, number>()
  for (const r of records) {
    if (!inZone(r.name, apex)) {
      add({ severity: 'error', code: 'out-of-zone', line: r.line, params: { name: r.name } })
      continue
    }
    const key = `${r.name} ${r.type} ${r.data.toLowerCase()}`
    if (seen.has(key))
      add({ severity: 'warning', code: 'duplicate', line: r.line, params: { first: seen.get(key)! } })
    else
      seen.set(key, r.line)
    const cut = underCut(r.name)
    if (cut && !['A', 'AAAA'].includes(r.type))
      add({ severity: 'warning', code: 'occluded', line: r.line, params: { cut } })
    else if (r.name !== apex && cuts.includes(r.name) && !['NS', 'DS', ...DNSSEC_TYPES].includes(r.type))
      add({ severity: 'warning', code: 'occluded', line: r.line, params: { cut: r.name } })

    switch (r.type) {
      case 'A':
        if (parseIPv4(r.data) === undefined)
          add({ severity: 'error', code: 'bad-address', line: r.line, params: { value: r.data } })
        break
      case 'AAAA':
        if (parseIPv6(r.data) === undefined)
          add({ severity: 'error', code: 'bad-address', line: r.line, params: { value: r.data } })
        break
      case 'TXT':
      case 'SPF':
        if (txtStrings(r.data).some(n => n > 255))
          add({ severity: 'error', code: 'txt-too-long', line: r.line })
        break
      case 'CNAME':
        if (r.name === apex)
          add({ severity: 'error', code: 'cname-apex', line: r.line })
        else if ((byName.get(r.name) ?? []).some(o => o.type !== 'CNAME' && !DNSSEC_TYPES.has(o.type)))
          add({ severity: 'error', code: 'cname-other-data', line: r.line, params: { name: r.name } })
        if ((byName.get(r.name) ?? []).filter(o => o.type === 'CNAME').length > 1)
          add({ severity: 'error', code: 'multiple-cname', line: r.line })
        break
    }
    // Targets of NS, MX, CNAME, SRV, PTR.
    const tokens = r.data.split(/\s+/)
    const targetIdx: Record<string, number> = { NS: 0, CNAME: 0, PTR: 0, DNAME: 0, MX: 1, SRV: 3 }
    const ti = targetIdx[r.type]
    if (ti !== undefined && tokens[ti]) {
      const raw = tokens[ti]!
      if (parseIPv4(raw.replace(/\.$/, '')) !== undefined || parseIPv6(raw) !== undefined) {
        add({ severity: 'error', code: 'target-is-ip', line: r.line, params: { type: r.type, value: raw } })
        continue
      }
      const target = absoluteName(raw, r.origin)
      // "mail.example.com" without the final dot in zone example.com becomes mail.example.com.example.com.
      const bareApex = apex.replace(/\.$/, '')
      if (!raw.endsWith('.') && apex !== '.' && (raw.toLowerCase() === bareApex || raw.toLowerCase().endsWith(`.${bareApex}`)))
        add({ severity: 'warning', code: 'missing-dot', line: r.line, params: { value: raw, target } })
      if (r.type === 'MX' || r.type === 'NS') {
        if (has(target, 'CNAME'))
          add({ severity: 'error', code: 'target-cname', line: r.line, params: { type: r.type, target } })
        else if (inZone(target, apex) && !has(target, 'A', 'AAAA') && !(r.type === 'MX' && raw === '.'))
          add({ severity: r.type === 'NS' ? 'error' : 'warning', code: r.type === 'NS' ? 'missing-glue' : 'target-no-address', line: r.line, params: { target } })
      }
    }
  }

  // TTLs inside one RRset must match (RFC 2181 §5.2).
  const rrsets = new Map<string, ZoneRecord[]>()
  for (const r of records) rrsets.set(`${r.name} ${r.type}`, [...(rrsets.get(`${r.name} ${r.type}`) ?? []), r])
  for (const [k, rs] of rrsets) {
    if (new Set(rs.map(r => r.ttl)).size > 1)
      add({ severity: 'warning', code: 'ttl-mismatch', line: rs[1]!.line, params: { rrset: k } })
    if (rs[0]!.type === 'TXT' && rs.filter(r => /^"?v=spf1(?:\s|"|$)/i.test(r.data)).length > 1)
      add({ severity: 'error', code: 'multiple-spf', line: rs[1]!.line, params: { name: rs[0]!.name } })
  }
  for (const r of records.filter(r => r.ttl > 0 && r.ttl < 60))
    add({ severity: 'info', code: 'low-ttl', line: r.line, params: { ttl: r.ttl } })

  findings.sort((a, b) => (a.line ?? 0) - (b.line ?? 0) || order[a.severity] - order[b.severity])
  // Unique per line + code.
  const uniq = new Set<string>()
  res.findings = findings.filter((f) => {
    const k = `${f.line}:${f.code}`
    return uniq.has(k) ? false : (uniq.add(k), true)
  })
  return res
}
