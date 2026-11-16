import { Hourglass } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.expiry-watchlist.title'),
  path: '/expiry-watchlist',
  description: translate('tools.expiry-watchlist.description'),
  keywords: ['expiry', 'watchlist', 'certificate', 'domain', 'renewal', 'monitor', 'ssl expiry', 'registration', 'dashboard'],
  component: () => import('./expiry-watchlist.vue'),
  icon: Hourglass,
  createdAt: new Date('2026-11-17'),
  runMode: 'server',
  capability: 'tls.expiry',
})
