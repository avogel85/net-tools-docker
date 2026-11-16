import type { Check, Watch } from './prefix-watch.service'
import { describe, expect, it } from 'vitest'
import { due, findings, level, load, parseWatches, save } from './prefix-watch.service'

const w: Watch = { prefix: '192.0.2.0/24', origins: [64500] }
const c = (over: Partial<Check>): Check => ({ prefix: w.prefix, checkedAt: '', announced: true, origins: [64500], more: [], rpki: { 64500: 'valid' }, error: '', ...over })

describe('prefix-watch', () => {
  it('parses watches', () => {
    const r = parseWatches('192.0.2.0/24 AS64500\n2001:db8::/32 64500, 64501\n198.51.100.0/24\nbad AS1\n10.0.0.0/8 ASX')
    expect(r.watches).toEqual([{ prefix: '192.0.2.0/24', origins: [64500] }, { prefix: '2001:db8::/32', origins: [64500, 64501] }])
    expect(r.invalid).toEqual(['198.51.100.0/24', 'bad AS1', '10.0.0.0/8 ASX'])
  })

  it('derives findings and levels', () => {
    expect(findings(w, c({}))).toEqual([])
    expect(level(findings(w, c({})))).toBe('ok')
    expect(findings(w, c({ origins: [64500, 666] }))).toEqual(['unexpected-origin', 'moas'])
    expect(findings(w, c({ more: [{ prefix: '192.0.2.0/25', origin: 666 }] }))).toEqual(['foreign-more-specific'])
    expect(level(findings(w, c({ more: [{ prefix: '192.0.2.0/25', origin: 64500 }] })))).toBe('ok')
    expect(findings(w, c({ rpki: { 64500: 'invalid_length' } }))).toEqual(['rpki-invalid'])
    expect(level(findings(w, c({ announced: false, origins: [], rpki: { 64500: 'unknown' } })))).toBe('warn')
  })

  it('picks stale prefixes first', () => {
    const now = Date.parse('2026-11-17T12:00:00Z')
    const ws = ['a', 'b', 'c'].map(x => ({ prefix: x, origins: [1] }))
    const checks = [c({ prefix: 'a', checkedAt: '2026-11-17T11:00:00Z' }), c({ prefix: 'b', checkedAt: '2026-11-16T00:00:00Z' })]
    expect(due(ws, checks, now).map(x => x.prefix)).toEqual(['c', 'b'])
  })

  it('stores safely', () => {
    const mem = new Map<string, string>()
    const st = { getItem: (k: string) => mem.get(k) ?? null, setItem: (k: string, v: string) => void mem.set(k, v) }
    expect(save({ watches: [w], checks: [] }, st)).toBe(true)
    expect(load(st).watches).toEqual([w])
    expect(load({ getItem: () => '{' })).toEqual({ watches: [], checks: [] })
  })
})
