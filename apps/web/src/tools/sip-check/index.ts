import { PhoneCall } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.sip-check.title'),
  path: '/sip-check',
  description: translate('tools.sip-check.description'),
  keywords: ['sip', 'voip', 'options', 'naptr', 'srv', 'pbx', 'asterisk', 'trunk', '5060', '5061', 'sips'],
  component: () => import('./sip-check.vue'),
  icon: PhoneCall,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'con.sip',
  accepts: { domain: 'host', ip: 'host' },
})
