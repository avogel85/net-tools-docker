import { Wifi } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.wifi-throughput.title'),
  path: '/wifi-throughput',
  description: translate('tools.wifi-throughput.description'),
  keywords: ['wifi', 'wi-fi', 'mcs', 'mcs index', '802.11ax', 'wifi 6', 'wifi 7', '802.11ac', 'phy rate', 'throughput', 'spatial streams', 'channel width', 'guard interval'],
  component: () => import('./wifi-throughput.vue'),
  icon: Wifi,
  createdAt: new Date('2026-11-17'),
  runMode: 'browser',
})
