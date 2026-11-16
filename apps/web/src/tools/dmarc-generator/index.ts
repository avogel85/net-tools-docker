import { ShieldCheck } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.dmarc-generator.title'),
  path: '/dmarc-generator',
  description: translate('tools.dmarc-generator.description'),
  keywords: ['dmarc', 'generator', 'record', 'policy', 'rua', 'ruf', 'alignment', 'email', 'txt', 'quarantine', 'reject'],
  component: () => import('./dmarc-generator.vue'),
  icon: ShieldCheck,
  createdAt: new Date('2026-11-16'),
  runMode: 'browser',
  accepts: { domain: 'domain' },
})
