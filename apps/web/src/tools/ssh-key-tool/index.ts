import { Fingerprint } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.ssh-key-tool.title'),
  path: '/ssh-key-tool',
  description: translate('tools.ssh-key-tool.description'),
  keywords: ['ssh', 'fingerprint', 'sshfp', 'known_hosts', 'authorized_keys', 'public key', 'ed25519', 'rsa', 'ecdsa', 'md5', 'sha256'],
  component: () => import('./ssh-key-tool.vue'),
  icon: Fingerprint,
  createdAt: new Date('2026-11-16'),
  runMode: 'browser',
})
