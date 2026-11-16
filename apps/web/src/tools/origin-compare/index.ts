import { ArrowsSplit } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.origin-compare.title'),
  path: '/origin-compare',
  description: translate('tools.origin-compare.description'),
  keywords: ['origin', 'cdn', 'cloudflare', 'bypass', 'waf', 'compare', 'direct', 'cache', 'headers', 'resolve'],
  component: () => import('./origin-compare.vue'),
  icon: ArrowsSplit,
  createdAt: new Date('2026-11-17'),
  runMode: 'server',
  capability: 'web.origin',
  accepts: { url: 'url' },
})
