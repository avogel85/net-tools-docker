import { Eye } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.prefix-watch.title'),
  path: '/prefix-watch',
  description: translate('tools.prefix-watch.description'),
  keywords: ['bgp', 'hijack', 'prefix monitoring', 'origin', 'moas', 'more-specific', 'rpki', 'watchlist', 'route leak'],
  component: () => import('./prefix-watch.vue'),
  icon: Eye,
  createdAt: new Date('2026-11-17'),
  runMode: 'public-api',
  capability: 'bgp.prefix',
})
