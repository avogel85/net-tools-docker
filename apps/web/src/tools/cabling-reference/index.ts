import { Plug } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.cabling-reference.title'),
  path: '/cabling-reference',
  description: translate('tools.cabling-reference.description'),
  keywords: ['cat6', 'cat6a', 'cat8', 'om3', 'om4', 'os2', 'sfp', 'sfp+', 'qsfp28', '10gbase-sr', 'lr', 'mpo', 'lc', 'apc', 'reach', 'cabling', 'transceiver'],
  component: () => import('./cabling-reference.vue'),
  icon: Plug,
  createdAt: new Date('2026-11-17'),
  runMode: 'browser',
})
