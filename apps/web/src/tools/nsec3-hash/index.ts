import { Hash } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.nsec3-hash.title'),
  path: '/nsec3-hash',
  description: translate('tools.nsec3-hash.description'),
  keywords: ['nsec3', 'hash', 'dnssec', 'salt', 'iterations', 'nsec3param', 'base32hex', 'denial of existence', 'rfc 5155'],
  component: () => import('./nsec3-hash.vue'),
  icon: Hash,
  createdAt: new Date('2026-11-16'),
  runMode: 'hybrid',
  accepts: { domain: 'domain' },
})
