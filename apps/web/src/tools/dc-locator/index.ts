import { BrandWindows } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.dc-locator.title'),
  path: '/dc-locator',
  description: translate('tools.dc-locator.description'),
  keywords: ['active directory', 'ad', 'domain controller', 'kerberos', 'ldap', 'srv', '_msdcs', 'global catalog', 'pdc', 'kpasswd', 'dc locator'],
  component: () => import('./dc-locator.vue'),
  icon: BrandWindows,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'dns.dclocator',
  accepts: { domain: 'domain' },
})
