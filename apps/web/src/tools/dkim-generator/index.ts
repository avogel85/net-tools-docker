import { Key } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.dkim-generator.title'),
  path: '/dkim-generator',
  description: translate('tools.dkim-generator.description'),
  keywords: ['dkim', 'generator', 'key pair', 'rsa', 'ed25519', 'selector', 'domainkey', 'email', 'txt', 'private key'],
  component: () => import('./dkim-generator.vue'),
  icon: Key,
  createdAt: new Date('2026-11-16'),
  runMode: 'browser',
  accepts: { domain: 'domain' },
})
