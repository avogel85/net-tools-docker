import { At } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.email-validator.title'),
  path: '/email-validator',
  description: translate('tools.email-validator.description'),
  keywords: ['email', 'address', 'validate', 'validator', 'syntax', 'mx', 'disposable', 'role address', 'typo', 'null mx', 'smtputf8'],
  component: () => import('./email-validator.vue'),
  icon: At,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'email.validate',
})
