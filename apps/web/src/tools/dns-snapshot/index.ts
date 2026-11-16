import { Camera } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.dns-snapshot.title'),
  path: '/dns-snapshot',
  description: translate('tools.dns-snapshot.description'),
  keywords: ['dns', 'snapshot', 'diff', 'compare', 'migration', 'changes', 'before after', 'backup', 'records', 'export'],
  component: () => import('./dns-snapshot.vue'),
  icon: Camera,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'dns.lookup',
  accepts: { domain: 'domain' },
})
