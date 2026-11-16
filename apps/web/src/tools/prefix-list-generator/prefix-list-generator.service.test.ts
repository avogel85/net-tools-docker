import type { Prefix } from './prefix-list-generator.service'
import { describe, expect, it } from 'vitest'
import { isValidListName, range, render } from './prefix-list-generator.service'

const v4: Prefix[] = [
  { prefix: '192.0.2.0/24', exact: true },
  { prefix: '198.51.100.0/22', exact: false, greaterEqual: 23, lessEqual: 24 },
  { prefix: '203.0.113.0/24', exact: false, lessEqual: 24 },
]
const v6: Prefix[] = [{ prefix: '2001:db8::/32', exact: false, lessEqual: 48 }]

describe('prefix-list-generator', () => {
  it('computes ranges', () => {
    expect(v4.map(range)).toEqual([[24, 24], [23, 24], [24, 24]])
    expect(range(v6[0]!)).toEqual([32, 48])
  })

  it('renders IOS / FRR / Arista', () => {
    expect(render(v4, 'ios', 'AS-EXAMPLE', 4)).toBe('no ip prefix-list AS-EXAMPLE\nip prefix-list AS-EXAMPLE seq 5 permit 192.0.2.0/24\nip prefix-list AS-EXAMPLE seq 10 permit 198.51.100.0/22 ge 23 le 24\nip prefix-list AS-EXAMPLE seq 15 permit 203.0.113.0/24\n')
    expect(render(v6, 'frr', 'X', 6)).toContain('ipv6 prefix-list X-V6 seq 5 permit 2001:db8::/32 le 48')
    expect(render([], 'ios', 'EMPTY', 4)).toContain('deny 0.0.0.0/0 le 32')
  })

  it('renders other platforms', () => {
    expect(render(v4, 'iosxr', 'P', 4)).toBe('prefix-set P\n  192.0.2.0/24,\n  198.51.100.0/22 ge 23 le 24,\n  203.0.113.0/24\nend-set\n')
    const j = render(v4, 'junos', 'P', 4)
    expect(j).toContain('route-filter-list P 192.0.2.0/24 exact')
    expect(j).toContain('route-filter-list P 198.51.100.0/22 prefix-length-range /23-/24')
    expect(render(v6, 'junos', 'P', 6)).toContain('2001:db8::/32 upto /48')
    expect(render(v4, 'routeros', 'P', 4)).toContain('rule="if (dst in 198.51.100.0/22 && dst-len in 23-24) { accept }"')
    expect(render(v4, 'bird', 'AS-X', 4)).toBe('define AS_X = [\n  192.0.2.0/24,\n  198.51.100.0/22{23,24},\n  203.0.113.0/24\n];\n')
    expect(render(v6, 'openbgpd', 'P', 6)).toContain('2001:db8::/32 prefixlen 32 - 48')
  })

  it('validates names', () => {
    expect(isValidListName('AS65000-IN')).toBe(true)
    expect(isValidListName('1bad')).toBe(false)
  })
})
