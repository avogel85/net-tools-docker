import { describe, expect, it } from 'vitest'
import { CONNECTORS, COPPER, FIBER, filterRows, OPTICS } from './cabling-reference.service'

describe('cabling-reference', () => {
  it('has consistent tables', () => {
    for (const t of [COPPER, FIBER, OPTICS, CONNECTORS]) {
      const keys = Object.keys(t[0]!).join()
      expect(t.every(r => Object.keys(r).join() === keys)).toBe(true)
    }
    expect(new Set(OPTICS.map(o => o.standard)).size).toBe(OPTICS.length)
  })
  it('filters case-insensitively across columns', () => {
    expect(filterRows(OPTICS, 'qsfp28').length).toBe(4)
    expect(filterRows(FIBER, 'AQUA').map(r => r.name)).toEqual(['OM3', 'OM4'])
    expect(filterRows(COPPER, '')).toHaveLength(COPPER.length)
  })
})
