import { Signature } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.dkim-verifier.title'),
  path: '/dkim-verifier',
  description: translate('tools.dkim-verifier.description'),
  keywords: ['dkim', 'verify', 'signature', 'body hash', 'eml', 'raw message', 'arc', 'authentication-results', 'email', 'alignment'],
  component: () => import('./dkim-verifier.vue'),
  icon: Signature,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'email.dkimverify',
})
