import { Terminal2 } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.nsupdate-builder.title'),
  path: '/nsupdate-builder',
  description: translate('tools.nsupdate-builder.description'),
  keywords: ['nsupdate', 'tsig', 'dynamic dns', 'rfc 2136', 'ddns', 'bind', 'knot', 'powerdns', 'hmac-sha256', 'key'],
  component: () => import('./nsupdate-builder.vue'),
  icon: Terminal2,
  createdAt: new Date('2026-11-16'),
  runMode: 'browser',
})
