import { Route } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.homelab-reach.title'),
  path: '/homelab-reach',
  description: translate('tools.homelab-reach.description'),
  keywords: ['homelab', 'vlan', 'reachability', 'firewall', 'port', 'matrix', 'nas', 'proxmox'],
  component: () => import('./homelab-reach.vue'),
  icon: Route,
  createdAt: new Date('2026-10-22'),
  runMode: 'server',
  capability: 'homelab.reach',
})
