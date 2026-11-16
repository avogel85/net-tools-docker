import { ListDetails } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.prefix-list-generator.title'),
  path: '/prefix-list-generator',
  description: translate('tools.prefix-list-generator.description'),
  keywords: ['prefix-list', 'bgpq4', 'as-set', 'irr', 'route filter', 'cisco', 'juniper', 'mikrotik', 'bird', 'frr', 'openbgpd', 'arista', 'peering filter'],
  component: () => import('./prefix-list-generator.vue'),
  icon: ListDetails,
  createdAt: new Date('2026-11-17'),
  runMode: 'server',
  capability: 'bgp.irr',
})
