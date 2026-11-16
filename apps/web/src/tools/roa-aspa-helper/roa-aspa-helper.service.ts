import type { RoaCoverage } from '../roa-coverage/roa-coverage.service'
import { cidrContains, parseCidr } from '@/utils/ip'

// ---------- ROA plan ----------

export type RoaAction = 'add' | 'remove'
export type RoaWhy = 'not-found' | 'invalid-length' | 'invalid-asn' | 'loose' | 'unused'
export interface RoaChange { action: RoaAction, prefix: string, maxLength: number, why: RoaWhy }

const len = (p: string) => Number(p.split('/')[1])

/** True when prefix a lies inside (or equals) b. */
export function within(a: string, b: string): boolean {
  const x = parseCidr(a, { allowBare: false })
  const y = parseCidr(b, { allowBare: false })
  return !!x && !!y && x.version === y.version && x.prefix >= y.prefix && cidrContains(y, { version: x.version, value: x.network })
}

/** Minimal ROA changes (RFC 9319): one ROA per announced prefix, maxLength = its length. */
export function roaPlan(r: RoaCoverage): RoaChange[] {
  const out: RoaChange[] = []
  const added = new Set<string>()
  const add = (prefix: string, why: RoaWhy) => {
    if (!added.has(prefix)) {
      added.add(prefix)
      out.push({ action: 'add', prefix, maxLength: len(prefix), why })
    }
  }
  for (const p of r.prefixes) {
    if (p.status === 'not-found')
      add(p.prefix, 'not-found')
    else if (p.status === 'invalid')
      add(p.prefix, p.reason === 'length' ? 'invalid-length' : 'invalid-asn')
  }
  const announced = r.prefixes.filter(p => p.status !== 'unchecked').map(p => p.prefix)
  for (const roa of r.roas) {
    if (roa.loose) {
      // Replace by exact ROAs for what is announced inside it.
      out.push({ action: 'remove', prefix: roa.prefix, maxLength: roa.maxLength, why: 'loose' })
      for (const a of announced.filter(a => within(a, roa.prefix) && len(a) <= roa.maxLength))
        add(a, 'loose')
    }
    else if (!roa.used) {
      out.push({ action: 'remove', prefix: roa.prefix, maxLength: roa.maxLength, why: 'unused' })
    }
  }
  return out
}

const roaText = (c: RoaChange) => (c.maxLength > len(c.prefix) ? `${c.prefix}-${c.maxLength}` : c.prefix)

/** Krill delta file (`krillc roas update --delta`). Unused ROAs are commented out: they may be planned announcements. */
export function krillDelta(asn: number, changes: RoaChange[]): string {
  return `${changes.map(c => `${c.why === 'unused' ? '# ' : ''}${c.action === 'add' ? 'A' : 'R'}: ${roaText(c)} => ${asn}`).join('\n')}\n`
}

/** Generic CSV for portals (RIPE NCC, ARIN, APNIC bulk import differ slightly). */
export function roaCsv(asn: number, changes: RoaChange[]): string {
  return `action,asn,prefix,maxLength\n${changes.map(c => `${c.action},AS${asn},${c.prefix},${c.maxLength}`).join('\n')}\n`
}

// ---------- ASPA ----------

export type Aspa = Map<number, Set<number>>
function asn(s: string) {
  const n = Number(s.trim().replace(/^AS/i, ''))
  return Number.isInteger(n) && n >= 0 && n < 4294967296 && s.trim() !== '' ? n : undefined
}

/** "AS64500: AS64501, AS64502" (or "=>") per line → customer → providers. */
export function parseAspas(text: string): { aspas: Aspa, invalid: string[] } {
  const aspas: Aspa = new Map()
  const invalid: string[] = []
  for (const line of text.split('\n').map(l => l.trim()).filter(l => l && !l.startsWith('#'))) {
    const m = /^([^\s:=]+)\s*(?::|=>)(.*)$/.exec(line)
    const c = m ? asn(m[1]!) : undefined
    const ps = m ? m[2]!.split(/[\s,]+/).filter(Boolean).map(asn) : []
    if (c === undefined || ps.includes(undefined)) {
      invalid.push(line)
      continue
    }
    // AS0 alone means "no providers" (a provider-free AS).
    aspas.set(c, new Set(ps as number[]))
  }
  return { aspas, invalid }
}

export function parsePath(text: string): number[] | undefined {
  const parts = text.trim().split(/[\s,]+/).filter(Boolean)
  const path = parts.map(asn)
  if (!parts.length || path.includes(undefined))
    return undefined
  // Collapse prepends.
  return (path as number[]).filter((a, i, all) => i === 0 || all[i - 1] !== a)
}

export type Hop = 'provider' | 'not-provider' | 'no-attestation'

/** Is b an attested provider of a? */
export function hop(aspas: Aspa, a: number, b: number): Hop {
  const ps = aspas.get(a)
  if (!ps)
    return 'no-attestation'
  return ps.has(b) ? 'provider' : 'not-provider'
}

export type Verdict = 'valid' | 'invalid' | 'unknown'
export interface Verification { verdict: Verdict, hops: { from: number, to: number, hop: Hop }[] }

/**
 * ASPA path verification (draft-ietf-sidrops-aspa-verification).
 * `path` is in BGP order: neighbour first, origin last.
 * upstream: received from a customer, peer or RS client; downstream: received from a provider.
 */
export function verify(aspas: Aspa, path: number[], direction: 'upstream' | 'downstream'): Verification {
  const a = [...path].reverse() // a[0] = origin
  const n = a.length
  const up = Array.from({ length: n - 1 }, (_, i) => hop(aspas, a[i]!, a[i + 1]!)) // a[i] → a[i+1]
  const down = Array.from({ length: n - 1 }, (_, i) => hop(aspas, a[i + 1]!, a[i]!)) // a[i+1] → a[i]
  const ramp = (hops: Hop[], ok: (h: Hop) => boolean) => {
    let k = 1
    while (k <= hops.length && ok(hops[k - 1]!))
      k++
    return k
  }
  const maxUp = ramp(up, h => h !== 'not-provider')
  const minUp = ramp(up, h => h === 'provider')
  const downRev = [...down].reverse() // from the neighbour side
  const maxDown = ramp(downRev, h => h !== 'not-provider')
  const minDown = ramp(downRev, h => h === 'provider')
  let verdict: Verdict
  if (direction === 'upstream')
    verdict = maxUp < n ? 'invalid' : minUp < n ? 'unknown' : 'valid'
  else if (n <= 2)
    verdict = 'valid'
  else
    verdict = maxUp + maxDown < n ? 'invalid' : minUp + minDown < n ? 'unknown' : 'valid'
  return { verdict, hops: up.map((h, i) => ({ from: a[i]!, to: a[i + 1]!, hop: h })) }
}

/** Krill command to publish one customer's ASPA. */
export function krillAspa(customer: number, providers: number[]): string {
  return `krillc aspas add --aspa "AS${customer} => ${providers.length ? providers.map(p => `AS${p}`).join(', ') : '<none>'}"\n`
}
