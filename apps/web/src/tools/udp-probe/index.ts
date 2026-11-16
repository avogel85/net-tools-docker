import { AccessPoint } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.udp-probe.title'),
  path: '/udp-probe',
  description: translate('tools.udp-probe.description'),
  keywords: ['udp', 'port', 'open filtered', 'dns', 'ntp', 'snmp', 'stun', 'quic', 'openvpn', 'tftp', 'netbios', 'ike', 'sip'],
  component: () => import('./udp-probe.vue'),
  icon: AccessPoint,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'con.udpprobe',
  accepts: { domain: 'host', ip: 'host' },
})
