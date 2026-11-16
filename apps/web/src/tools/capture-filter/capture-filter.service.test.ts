import type { FilterInput } from './capture-filter.service'
import { describe, expect, it } from 'vitest'
import { build } from './capture-filter.service'

const base: FilterInput = { hosts: [], direction: 'any', ports: [], proto: '', excludePorts: [], excludeHosts: [], vlan: '', tcpFlags: '', minSize: 0, iface: '', file: '' }

describe('capture-filter', () => {
  it('builds host, port and protocol filters', () => {
    const r = build({ ...base, hosts: ['192.0.2.10', '198.51.100.0/24'], ports: ['443', '8000-8080'], proto: 'tcp' })
    expect(r.bpf).toBe('tcp and (host 192.0.2.10 or net 198.51.100.0/24) and (tcp port 443 or tcp portrange 8000-8080)')
    expect(r.display).toBe('tcp && (ip.addr == 192.0.2.10 || ip.addr == 198.51.100.0/24) && (tcp.port == 443 || (tcp.port >= 8000 && tcp.port <= 8080))')
    expect(r.errors).toEqual([])
  })

  it('handles direction, exclusions, flags, size and VLAN', () => {
    const r = build({ ...base, hosts: ['2001:db8::1'], direction: 'src', excludePorts: ['22'], tcpFlags: 'syn', minSize: 1000, vlan: '20' })
    expect(r.bpf).toBe('vlan 20 and src host 2001:db8::1 and (ip6 and ip6[6] == 6 and ip6[53] & 0x12 == 0x02) and greater 1000 and not port 22')
    expect(build({ ...base, hosts: ['192.0.2.1'], tcpFlags: 'rst' }).bpf).toBe('host 192.0.2.1 and tcp[13] & 0x04 == 0x04')
    expect(build({ ...base, tcpFlags: 'synack' }).bpf).toBe('(tcp[13] & 0x12 == 0x12 or (ip6 and ip6[6] == 6 and ip6[53] & 0x12 == 0x12))')
    expect(r.display).toBe('vlan.id == 20 && ipv6.src == 2001:db8::1 && tcp.flags.syn == 1 && tcp.flags.ack == 0 && frame.len >= 1000 && !(tcp.port == 22 || udp.port == 22)')
  })

  it('builds commands with quoting', () => {
    const r = build({ ...base, ports: ['53'], proto: 'udp', iface: 'eth0', file: 'dns.pcap' })
    expect(r.tcpdump).toBe('tcpdump -i eth0 -nn -w dns.pcap \'udp and udp port 53\'')
    expect(r.tshark).toBe('tshark -i eth0 -f \'udp and udp port 53\' -w dns.pcap')
    expect(build(base).tcpdump).toBe('tcpdump -i any -nn')
  })

  it('reports invalid input', () => {
    expect(build({ ...base, hosts: ['not a host'], ports: ['99999'], vlan: 'x', iface: 'eth0; rm', file: '../x y' }).errors).toEqual(['hosts', 'ports', 'vlan', 'iface', 'file'])
  })
})
