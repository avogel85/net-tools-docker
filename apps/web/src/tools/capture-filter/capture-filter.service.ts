import { parseCidr, parseIP } from '@/utils/ip'

export type Direction = 'any' | 'src' | 'dst'
export type Proto = '' | 'tcp' | 'udp' | 'icmp' | 'icmp6' | 'arp'
export type TcpFlags = '' | 'syn' | 'rst' | 'synack'

export interface FilterInput {
  hosts: string[]
  direction: Direction
  ports: string[]
  proto: Proto
  excludePorts: string[]
  excludeHosts: string[]
  vlan: string
  tcpFlags: TcpFlags
  minSize: number
  iface: string
  file: string
}

export interface Built { bpf: string, display: string, tcpdump: string, tshark: string, errors: string[] }

const portOk = (p: string) => /^\d{1,5}(?:-\d{1,5})?$/.test(p) && p.split('-').every(x => Number(x) >= 0 && Number(x) <= 65535)
const hostOk = (h: string) => !!parseIP(h) || (!!parseCidr(h, { allowBare: false })) || /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$/i.test(h)
const isNet = (h: string) => h.includes('/')
const isV6 = (h: string) => h.includes(':')
const or = (xs: string[]) => (xs.length > 1 ? `(${xs.join(' or ')})` : xs[0]!)
const orD = (xs: string[]) => (xs.length > 1 ? `(${xs.join(' || ')})` : xs[0]!)
const shellQuote = (s: string) => `'${s.replace(/'/g, `'\\''`)}'`

function bpfHost(h: string, dir: Direction): string {
  const kw = isNet(h) ? 'net' : 'host'
  return `${dir === 'any' ? '' : `${dir} `}${kw} ${h}`
}
function dispHost(h: string, dir: Direction): string {
  const field = isV6(h) ? 'ipv6' : 'ip'
  const name = /^[a-z]/i.test(h) && !isNet(h) && !parseIP(h)
  if (name)
    return dir === 'any' ? `ip.host == "${h}"` : `ip.${dir}_host == "${h}"`
  return `${field}.${dir === 'any' ? 'addr' : dir} == ${h}`
}
function bpfPort(p: string, dir: Direction, proto: Proto): string {
  const pre = `${proto === 'tcp' || proto === 'udp' ? `${proto} ` : ''}${dir === 'any' ? '' : `${dir} `}`
  return p.includes('-') ? `${pre}portrange ${p}` : `${pre}port ${p}`
}
function dispPort(p: string, dir: Direction, proto: Proto): string {
  const protos = proto === 'tcp' || proto === 'udp' ? [proto] : ['tcp', 'udp']
  const field = dir === 'any' ? 'port' : `${dir === 'src' ? 'srcport' : 'dstport'}`
  const one = (pr: string) => (p.includes('-') ? `(${pr}.${field} >= ${p.split('-')[0]} && ${pr}.${field} <= ${p.split('-')[1]})` : `${pr}.${field} == ${p}`)
  return orD(protos.map(one))
}

/** The address family the filter is limited to by its hosts (0 = both or unknown). */
function family(hosts: string[], proto: Proto): 0 | 4 | 6 {
  if (proto === 'icmp6')
    return 6
  const fams = new Set(hosts.filter(h => parseIP(h.split('/')[0]!)).map(h => (isV6(h) ? 6 : 4)))
  const names = hosts.some(h => !parseIP(h.split('/')[0]!))
  return fams.size === 1 && !names ? [...fams][0] as 4 | 6 : 0
}

export function build(i: FilterInput): Built {
  const errors: string[] = []
  const bad = (list: string[], ok: (s: string) => boolean, code: string) => {
    if (list.some(x => !ok(x)))
      errors.push(code)
  }
  bad(i.hosts, hostOk, 'hosts')
  bad(i.excludeHosts, hostOk, 'excludeHosts')
  bad(i.ports, portOk, 'ports')
  bad(i.excludePorts, portOk, 'excludePorts')
  if (i.vlan && !/^\d{1,4}$/.test(i.vlan))
    errors.push('vlan')
  if (i.iface && !/^[\w.:-]{1,32}$/.test(i.iface))
    errors.push('iface')
  if (i.file && !/^[\w./-]{1,128}$/.test(i.file))
    errors.push('file')

  const b: string[] = []
  const d: string[] = []
  if (i.vlan) {
    b.push(`vlan ${i.vlan}`)
    d.push(`vlan.id == ${i.vlan}`)
  }
  if (i.proto) {
    b.push(i.proto === 'icmp6' ? 'icmp6' : i.proto)
    d.push(i.proto === 'icmp6' ? 'icmpv6' : i.proto)
  }
  if (i.hosts.length) {
    b.push(or(i.hosts.map(h => bpfHost(h, i.direction))))
    d.push(orD(i.hosts.map(h => dispHost(h, i.direction))))
  }
  if (i.ports.length) {
    b.push(or(i.ports.map(p => bpfPort(p, i.direction, i.proto))))
    d.push(orD(i.ports.map(p => dispPort(p, i.direction, i.proto))))
  }
  if (i.tcpFlags) {
    // BPF tcp[] only reaches into IPv4; for IPv6 read the flags byte after the fixed 40-byte header
    // (no extension headers). Mask/value per flag set: SYN without ACK, SYN+ACK, RST.
    const [m, v] = { syn: ['0x12', '0x02'], synack: ['0x12', '0x12'], rst: ['0x04', '0x04'] }[i.tcpFlags]
    const v4 = `tcp[13] & ${m} == ${v}`
    const v6 = `(ip6 and ip6[6] == 6 and ip6[53] & ${m} == ${v})`
    const fam = family(i.hosts, i.proto)
    b.push(fam === 4 ? v4 : fam === 6 ? v6 : `(${v4} or ${v6})`)
    d.push({ syn: 'tcp.flags.syn == 1 && tcp.flags.ack == 0', synack: 'tcp.flags.syn == 1 && tcp.flags.ack == 1', rst: 'tcp.flags.reset == 1' }[i.tcpFlags])
  }
  if (i.minSize > 0) {
    b.push(`greater ${Math.round(i.minSize)}`)
    d.push(`frame.len >= ${Math.round(i.minSize)}`)
  }
  for (const h of i.excludeHosts) {
    b.push(`not ${bpfHost(h, 'any')}`)
    d.push(`!(${dispHost(h, 'any')})`)
  }
  for (const p of i.excludePorts) {
    b.push(`not ${bpfPort(p, 'any', '')}`)
    d.push(`!${dispPort(p, 'any', '')}`)
  }
  const bpf = b.join(' and ')
  const display = d.join(' && ')
  const iface = i.iface || 'any'
  const tcpdump = ['tcpdump -i', iface, '-nn', i.file ? `-w ${i.file}` : '', bpf ? shellQuote(bpf) : ''].filter(Boolean).join(' ')
  const tshark = ['tshark -i', iface, bpf ? `-f ${shellQuote(bpf)}` : '', i.file ? `-w ${i.file}` : (display ? `-Y ${shellQuote(display)}` : '')].filter(Boolean).join(' ')
  return { bpf, display, tcpdump, tshark, errors }
}
