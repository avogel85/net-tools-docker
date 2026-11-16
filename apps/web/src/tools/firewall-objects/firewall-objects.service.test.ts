import { describe, expect, it } from 'vitest'
import { isValidName, normalize, parseList, render } from './firewall-objects.service'

const list = parseList('192.0.2.10 # web server\n198.51.100.0/24, 198.51.101.0/24\n203.0.113.1-203.0.113.6\n2001:db8::/48\nnot-an-ip\n192.0.2.10')

describe('firewall-objects', () => {
  it('parses IPs, CIDRs, ranges and comments', () => {
    expect(list.invalid).toEqual([{ line: 5, text: 'not-an-ip' }])
    expect(list.entries).toHaveLength(9) // range 1-6 = /32, /31, /31, /32 → 4 networks
    expect(list.entries[0]!.comment).toBe('web server')
  })

  it('dedupes and merges', () => {
    expect(normalize(list.entries, false)).toHaveLength(8)
    const merged = normalize(list.entries, true)
    expect(merged.map(e => `${e.cidr.prefix}`)).toContain('23') // 198.51.100.0/23
  })

  it('renders platforms', () => {
    const e = normalize(list.entries, true)
    expect(render(e, 'asa', 'BLOCK')).toContain(' network-object 198.51.100.0 255.255.254.0')
    expect(render(e, 'ios', 'BLOCK')).toContain(' permit 198.51.100.0 0.0.1.255')
    expect(render(e, 'ios', 'BLOCK')).toContain('ipv6 access-list BLOCK_V6')
    const fg = render(e, 'fortigate', 'BLOCK')
    expect(fg).toContain('set subnet 192.0.2.10 255.255.255.255')
    expect(fg).toContain('set comment "web server"')
    expect(fg).toContain('config firewall address6')
    expect(render(e, 'paloalto', 'BLOCK')).toContain('set address-group BLOCK static [')
    expect(render(e, 'mikrotik', 'BLOCK')).toContain('/ipv6 firewall address-list add list=BLOCK address=2001:db8::/48')
    expect(render(e, 'ipset', 'BLOCK')).toContain('ipset create BLOCK_v6 hash:net family inet6 -exist')
    expect(render(e, 'nftables', 'BLOCK')).toContain('type ipv4_addr')
    expect(render(e, 'pfsense', 'BLOCK').split('\n')[0]).toBe('192.0.2.10 # web server')
    expect(render([], 'asa', 'X')).toBe('object-group network X\n')
  })

  it('validates names', () => {
    expect(isValidName('BLOCK_LIST-1')).toBe(true)
    expect(isValidName('1bad')).toBe(false)
    expect(isValidName('has space')).toBe(false)
  })
})
