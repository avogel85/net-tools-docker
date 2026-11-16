import { Lock } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.mta-sts-generator.title'),
  path: '/mta-sts-generator',
  description: translate('tools.mta-sts-generator.description'),
  keywords: ['mta-sts', 'tls-rpt', 'generator', 'policy', 'smtp', 'starttls', 'email', 'nginx', 'stsv1'],
  component: () => import('./mta-sts-generator.vue'),
  icon: Lock,
  createdAt: new Date('2026-11-16'),
  runMode: 'browser',
  accepts: { domain: 'domain' },
})
