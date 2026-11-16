import { ListCheck } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.zone-linter.title'),
  path: '/zone-linter',
  description: translate('tools.zone-linter.description'),
  keywords: ['zone file', 'lint', 'bind', 'named-checkzone', 'cname', 'glue', 'soa', 'ttl', 'validator', 'syntax'],
  component: () => import('./zone-linter.vue'),
  icon: ListCheck,
  createdAt: new Date('2026-11-16'),
  runMode: 'browser',
})
