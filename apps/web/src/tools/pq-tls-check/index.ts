import { Atom } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.pq-tls-check.title'),
  path: '/pq-tls-check',
  description: translate('tools.pq-tls-check.description'),
  keywords: ['post-quantum', 'pqc', 'ml-kem', 'kyber', 'x25519mlkem768', 'tls 1.3', 'key exchange', 'hybrid', 'quantum'],
  component: () => import('./pq-tls-check.vue'),
  icon: Atom,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'tls.pq',
  accepts: { domain: 'host', ip: 'host' },
})
