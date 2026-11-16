import { describe, expect, it } from 'vitest'
import { isValidLabel, zoneLine, zoneText } from './spf-flattener.service'

describe('spf-flattener', () => {
  it('renders a short record as one string', () => {
    expect(zoneLine({ name: 'example.com', value: 'v=spf1 ip4:192.0.2.0/24 -all', length: 28 }))
      .toBe('example.com. 3600 IN TXT "v=spf1 ip4:192.0.2.0/24 -all"')
  })

  it('splits values longer than 255 bytes', () => {
    const value = `v=spf1 ${Array.from({ length: 20 }, (_, i) => `ip4:10.${i}.0.0/16`).join(' ')} -all`
    const line = zoneLine({ name: '_spf1.example.com', value, length: value.length }, 300)
    expect(line.startsWith('_spf1.example.com. 300 IN TXT "v=spf1 ')).toBe(true)
    const strings = line.slice(line.indexOf('"')).match(/"[^"]*"/g) ?? []
    expect(strings.length).toBe(2)
    expect(strings.map(s => s.slice(1, -1)).join('')).toBe(value)
  })

  it('joins all records', () => {
    expect(zoneText([
      { name: 'a.test', value: 'v=spf1 include:_spf1.a.test -all', length: 0 },
      { name: '_spf1.a.test', value: 'v=spf1 ip4:192.0.2.1', length: 0 },
    ]).split('\n')).toHaveLength(2)
  })

  it('validates the chain label', () => {
    expect(isValidLabel('_spf')).toBe(true)
    expect(isValidLabel('flat-1')).toBe(true)
    expect(isValidLabel('')).toBe(false)
    expect(isValidLabel('-x')).toBe(false)
    expect(isValidLabel('a.b')).toBe(false)
    expect(isValidLabel('x'.repeat(31))).toBe(false)
  })
})
