import { Hierarchy } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.ldap-check.title'),
  path: '/ldap-check',
  description: translate('tools.ldap-check.description'),
  keywords: ['ldap', 'ldaps', 'starttls', 'rootdse', 'active directory', 'openldap', '389', '636', 'naming context', 'directory'],
  component: () => import('./ldap-check.vue'),
  icon: Hierarchy,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'con.ldap',
  accepts: { domain: 'host', ip: 'host' },
})
