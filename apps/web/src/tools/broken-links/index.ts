import { Unlink } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.broken-links.title'),
  path: '/broken-links',
  description: translate('tools.broken-links.description'),
  keywords: ['broken links', 'link checker', '404', 'dead links', 'crawl', 'redirect loop', 'assets', 'site audit'],
  component: () => import('./broken-links.vue'),
  icon: Unlink,
  createdAt: new Date('2026-11-17'),
  runMode: 'server',
  capability: 'web.links',
  accepts: { url: 'url' },
})
