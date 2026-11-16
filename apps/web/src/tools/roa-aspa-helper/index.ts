import { ShieldCheck } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.roa-aspa-helper.title'),
  path: '/roa-aspa-helper',
  description: translate('tools.roa-aspa-helper.description'),
  keywords: ['roa', 'aspa', 'rpki', 'maxlength', 'krill', 'route leak', 'as path verification', 'provider authorization', 'rfc 9319'],
  component: () => import('./roa-aspa-helper.vue'),
  icon: ShieldCheck,
  createdAt: new Date('2026-11-17'),
  runMode: 'hybrid',
  capability: 'bgp.roacoverage',
  accepts: { asn: 'asn' },
})
