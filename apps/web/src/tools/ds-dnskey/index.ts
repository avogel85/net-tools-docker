import { ShieldLock } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.ds-dnskey.title'),
  path: '/ds-dnskey',
  description: translate('tools.ds-dnskey.description'),
  keywords: ['ds', 'dnskey', 'dnssec', 'key tag', 'ksk', 'zsk', 'rollover', 'digest', 'registrar', 'parent'],
  component: () => import('./ds-dnskey.vue'),
  icon: ShieldLock,
  createdAt: new Date('2026-11-16'),
  runMode: 'hybrid',
  accepts: { domain: 'domain' },
})
