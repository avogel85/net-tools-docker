import { AlertTriangle } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.arp-conflicts.title'),
  path: '/arp-conflicts',
  description: translate('tools.arp-conflicts.description'),
  keywords: ['arp', 'duplicate ip', 'ip conflict', 'mac change', 'spoofing', 'rogue device', 'new device', 'baseline'],
  component: () => import('./arp-conflicts.vue'),
  icon: AlertTriangle,
  createdAt: new Date('2026-10-22'),
  runMode: 'server',
  capability: 'lan.arpscan',
  requiresLan: true,
})
