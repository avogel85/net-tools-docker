import { describe, expect, it } from 'vitest'
import { nameIndex, parseKnown, unknownHosts } from './homelab-inventory.service'

describe('homelab-inventory', () => {
  const guests = [{ node: 'pve01', vmid: 240, type: 'lxc', name: 'homepage', status: 'running', ip: '192.168.20.185' }]
  it('lists ARP hosts that no guest owns, numerically sorted', () => {
    const arp = [{ ip: '192.168.20.185', mac: 'a' }, { ip: '192.168.20.20', mac: 'b' }, { ip: '192.168.20.9', mac: 'c' }]
    expect(unknownHosts(arp, guests).map(h => h.ip)).toEqual(['192.168.20.9', '192.168.20.20'])
  })
  it('skips explicitly known addresses', () => {
    expect(unknownHosts([{ ip: '192.168.20.20', mac: 'b' }], guests, ['192.168.20.20'])).toEqual([])
  })
  it('names unknown hosts from UniFi by MAC, case-insensitively', () => {
    const names = nameIndex([{ mac: 'AA:BB:CC:00:00:01', name: 'printer' }])
    expect(unknownHosts([{ ip: '10.0.0.7', mac: 'aa:bb:cc:00:00:01' }], [], [], names)[0]?.name).toBe('printer')
  })
  it('parses a known-address list', () => {
    expect(parseKnown('192.168.20.20, 192.168.20.21\n192.168.20.22')).toEqual(['192.168.20.20', '192.168.20.21', '192.168.20.22'])
  })
})
