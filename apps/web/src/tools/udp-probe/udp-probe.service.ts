import type { components } from '@/api/schema'

export type UdpProbe = components['schemas']['UdpProbe']

export const SERVICES = ['all', 'dns', 'ntp', 'snmp', 'sip', 'stun', 'ike', 'quic', 'openvpn', 'netbios', 'tftp', 'custom'] as const

export const STATE_TYPE = { 'open': 'success', 'closed': 'default', 'open|filtered': 'warning', 'unexpected': 'info', 'error': 'error' } as const

/** Open first, then unexpected, then the rest. */
export function sortResults(rs: UdpProbe['results']): UdpProbe['results'] {
  const order: Record<string, number> = { 'open': 0, 'unexpected': 1, 'open|filtered': 2, 'closed': 3, 'error': 4 }
  return [...rs].sort((a, b) => (order[a.state] ?? 9) - (order[b.state] ?? 9))
}
