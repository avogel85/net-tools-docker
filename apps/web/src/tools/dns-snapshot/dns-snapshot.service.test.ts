import type { Snapshot } from './dns-snapshot.service'
import { describe, expect, it } from 'vitest'
import { collect, diff, loadSnapshots, parseSnapshot, plan, saveSnapshots } from './dns-snapshot.service'

const snap = (records: [string, string, number, string][]): Snapshot => ({ id: 'x', domain: 'example.com', takenAt: '', resolver: '', errors: [], records: collect(records.map(([name, type, ttl, data]) => ({ name, type, ttl, data }))) })

describe('dns-snapshot', () => {
  it('plans apex, common and extra names', () => {
    const q = plan('Example.com.', ['shop', 'api.example.com', 'www'])
    expect(q.filter(x => x.name === 'example.com').map(x => x.type)).toEqual(['A', 'AAAA', 'MX', 'NS', 'TXT', 'SOA', 'CAA', 'HTTPS'])
    expect(q.some(x => x.name === 'shop.example.com' && x.type === 'TXT')).toBe(true)
    expect(q.some(x => x.name === 'api.example.com')).toBe(true)
    expect(q.filter(x => x.name === 'www.example.com')).toHaveLength(3) // not added twice
  })

  it('dedupes and sorts answers', () => {
    const r = collect([
      { name: 'www.example.com.', type: 'CNAME', ttl: 300, data: 'web.example.net.' },
      { name: 'WWW.example.com', type: 'CNAME', ttl: 300, data: 'web.example.net.' },
      { name: 'a.example.com.', type: 'A', ttl: 60, data: '192.0.2.1' },
    ])
    expect(r.map(x => x.name)).toEqual(['a.example.com', 'www.example.com'])
  })

  it('diffs per RRset, ignoring TTL and SOA serial unless asked', () => {
    const a = snap([
      ['example.com', 'A', 300, '192.0.2.1'],
      ['example.com', 'SOA', 3600, 'ns1. host. 2026010101 7200 900 1209600 300'],
      ['example.com', 'MX', 300, '10 mx1.example.com.'],
      ['old.example.com', 'A', 300, '192.0.2.9'],
      ['example.com', 'TXT', 300, '"v=spf1 -all"'],
    ])
    const b = snap([
      ['example.com', 'A', 60, '192.0.2.1'],
      ['example.com', 'SOA', 3600, 'ns1. host. 2026111601 7200 900 1209600 300'],
      ['example.com', 'MX', 300, '10 mx2.example.com.'],
      ['new.example.com', 'A', 300, '192.0.2.10'],
      ['example.com', 'TXT', 300, '"v=spf1 -all"'],
    ])
    const d = diff(a, b)
    expect(d.changed.map(c => c.type)).toEqual(['MX'])
    expect(d.added.map(c => c.name)).toEqual(['new.example.com'])
    expect(d.removed.map(c => c.name)).toEqual(['old.example.com'])
    expect(d.same).toBe(3)
    expect(diff(a, b, { ttl: true, serial: true }).changed.map(c => c.type).sort()).toEqual(['A', 'MX', 'SOA'])
  })

  it('stores snapshots and survives broken storage', () => {
    const mem = new Map<string, string>()
    const storage = { getItem: (k: string) => mem.get(k) ?? null, setItem: (k: string, v: string) => void mem.set(k, v) }
    expect(saveSnapshots([snap([])], storage)).toBe(true)
    expect(loadSnapshots(storage)).toHaveLength(1)
    expect(loadSnapshots({ getItem: () => '{bad' })).toEqual([])
    const full = {
      setItem: () => {
        throw new Error('quota')
      },
    }
    expect(saveSnapshots([], full)).toBe(false)
  })

  it('imports exported files', () => {
    expect(parseSnapshot(JSON.stringify(snap([['example.com', 'A', 300, '192.0.2.1']])))?.records).toHaveLength(1)
    expect(parseSnapshot('{"domain": 1}')).toBeUndefined()
    expect(parseSnapshot('nope')).toBeUndefined()
  })
})
