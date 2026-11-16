import { Mailbox } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.email-provider-detector.title'),
  path: '/email-provider-detector',
  description: translate('tools.email-provider-detector.description'),
  keywords: ['email provider', 'mx', 'microsoft 365', 'google workspace', 'proofpoint', 'mimecast', 'spf', 'sendgrid', 'saas', 'verification', 'detect'],
  component: () => import('./email-provider-detector.vue'),
  icon: Mailbox,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'email.providers',
  accepts: { domain: 'domain' },
})
