import { Wall } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.firewall-objects.title'),
  path: '/firewall-objects',
  description: translate('tools.firewall-objects.description'),
  keywords: ['firewall', 'object group', 'address list', 'fortigate', 'cisco asa', 'palo alto', 'pfsense', 'mikrotik', 'ipset', 'nftables', 'juniper', 'acl', 'ip list'],
  component: () => import('./firewall-objects.vue'),
  icon: Wall,
  createdAt: new Date('2026-11-17'),
  runMode: 'browser',
})
