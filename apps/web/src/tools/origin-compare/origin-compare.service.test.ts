import { describe, expect, it } from 'vitest'
import { isValidIp, sortDiffs } from './origin-compare.service'

describe('origin-compare', () => {
  it('validates IPs', () => {
    expect(isValidIp('203.0.113.5')).toBe(true)
    expect(isValidIp('[2001:db8::1]')).toBe(true)
    expect(isValidIp('origin.example.com')).toBe(false)
    expect(isValidIp('300.1.1.1')).toBe(false)
  })
  it('puts differences first', () => {
    expect(sortDiffs([{ field: 'a', cdn: '1', origin: '1', same: true }, { field: 'b', cdn: '1', origin: '2', same: false }]).map(d => d.field)).toEqual(['b', 'a'])
  })
})
