import { Server } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.homelab-inventory.title'),
  path: '/homelab-inventory',
  description: translate('tools.homelab-inventory.description'),
  keywords: ['homelab', 'proxmox', 'inventory', 'vm', 'lxc', 'guests', 'ip', 'arp'],
  component: () => import('./homelab-inventory.vue'),
  icon: Server,
  createdAt: new Date('2026-10-22'),
  runMode: 'server',
  capability: 'homelab.inventory',
})
