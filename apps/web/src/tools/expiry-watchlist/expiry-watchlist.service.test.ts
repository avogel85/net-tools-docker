import type { Check } from './expiry-watchlist.service'
import { describe, expect, it } from 'vitest'
import { domainsToCheck, level, load, mergeEntries, parseEntries, rows, save, upsertChecks } from './expiry-watchlist.service'

const check = (target: string, kind: 'cert' | 'domain', daysLeft: number | null, checkedAt = '2026-11-17T00:00:00Z'): Check => ({ target, kind, daysLeft, checkedAt, expires: '', by: '', error: '' })

describe('expiry-watchlist', () => {
  it('parses entries', () => {
    const r = parseEntries('example.com\nmail.example.com:465, https://shop.example.com/path\ndomain:example.org; cert:vpn.example.net\nnot valid')
    expect(r.entries).toEqual([
      { target: 'example.com', kinds: ['cert', 'domain'] },
      { target: 'mail.example.com:465', kinds: ['cert'] },
      { target: 'shop.example.com', kinds: ['cert'] },
      { target: 'example.org', kinds: ['domain'] },
      { target: 'vpn.example.net', kinds: ['cert'] },
    ])
    expect(r.invalid).toEqual(['not valid'])
    expect(parseEntries('domain:example.com:443').invalid).toHaveLength(1)
    expect(parseEntries('example.co.uk').entries[0]!.kinds).toEqual(['cert', 'domain'])
    expect(parseEntries('domain:sub.example.com').entries[0]!.kinds).toEqual(['domain'])
  })

  it('merges entries', () => {
    expect(mergeEntries([{ target: 'a.test', kinds: ['cert'] }], [{ target: 'a.test', kinds: ['domain'] }, { target: 'b.test', kinds: ['cert'] }]))
      .toEqual([{ target: 'a.test', kinds: ['cert', 'domain'] }, { target: 'b.test', kinds: ['cert'] }])
  })

  it('picks stale domains first and skips fresh ones', () => {
    const now = Date.parse('2026-11-17T12:00:00Z')
    const entries = ['a', 'b', 'c'].map(x => ({ target: `${x}.test`, kinds: ['domain' as const] }))
    const checks = [check('a.test', 'domain', 10, '2026-11-17T06:00:00Z'), check('b.test', 'domain', 10, '2026-11-10T00:00:00Z')]
    expect(domainsToCheck(entries, checks, now)).toEqual(['c.test', 'b.test'])
  })

  it('upserts and ranks checks', () => {
    const merged = upsertChecks([check('a.test', 'cert', 50)], [check('a.test', 'cert', 5), check('b.test', 'cert', -1), check('c.test', 'cert', null)])
    expect(merged).toHaveLength(3)
    const store = { entries: ['a', 'b', 'c'].map(x => ({ target: `${x}.test`, kinds: ['cert' as const] })), checks: merged, warnDays: 30 }
    expect(rows(store).map(c => c.target)).toEqual(['b.test', 'a.test', 'c.test'])
    expect([level(merged[1]!, 30), level(merged[0]!, 30), level(merged[2]!, 30)]).toEqual(['expired', 'soon', 'unknown'])
    expect(rows({ ...store, entries: [] })).toEqual([])
  })

  it('stores safely', () => {
    const mem = new Map<string, string>()
    const st = { getItem: (k: string) => mem.get(k) ?? null, setItem: (k: string, v: string) => void mem.set(k, v) }
    expect(save({ entries: [{ target: 'a.test', kinds: ['cert'] }], checks: [], warnDays: 14 }, st)).toBe(true)
    expect(load(st).warnDays).toBe(14)
    expect(load({ getItem: () => '{oops' })).toEqual({ entries: [], checks: [], warnDays: 30 })
  })
})
