import { Ghost } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.takeover-check.title'),
  path: '/takeover-check',
  description: translate('tools.takeover-check.description'),
  keywords: ['subdomain takeover', 'dangling cname', 'cname', 'github pages', 'azure', 's3', 'heroku', 'unclaimed', 'security'],
  component: () => import('./takeover-check.vue'),
  icon: Ghost,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'dns.takeover',
  accepts: { domain: 'domain' },
})
