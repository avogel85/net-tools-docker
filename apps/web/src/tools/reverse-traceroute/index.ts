import { ArrowBackUp } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.reverse-traceroute.title'),
  path: '/reverse-traceroute',
  description: translate('tools.reverse-traceroute.description'),
  keywords: ['reverse traceroute', 'return path', 'asymmetric routing', 'globalping', 'inbound path', 'traceroute to me', 'peering'],
  component: () => import('./reverse-traceroute.vue'),
  icon: ArrowBackUp,
  createdAt: new Date('2026-11-16'),
  runMode: 'public-api',
  capability: 'remote.globalping',
})
