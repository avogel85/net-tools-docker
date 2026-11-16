import { ArrowsMinimize } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.spf-flattener.title'),
  path: '/spf-flattener',
  description: translate('tools.spf-flattener.description'),
  keywords: ['spf', 'flatten', 'flattening', 'include', 'dns lookups', 'permerror', '10 lookup limit', 'email', 'txt', 'ip4', 'ip6'],
  component: () => import('./spf-flattener.vue'),
  icon: ArrowsMinimize,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'email.spfflatten',
  accepts: { domain: 'domain' },
})
