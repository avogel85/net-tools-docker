import { LockAccess } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.ike-probe.title'),
  path: '/ike-probe',
  description: translate('tools.ike-probe.description'),
  keywords: ['ipsec', 'ike', 'ikev2', 'ikev1', 'vpn', 'site-to-site', 'proposal', 'dh group', 'nat-t', '500', '4500', 'strongswan'],
  component: () => import('./ike-probe.vue'),
  icon: LockAccess,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'con.ike',
  accepts: { domain: 'host', ip: 'host' },
})
