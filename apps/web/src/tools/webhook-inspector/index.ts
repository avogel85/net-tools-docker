import { Inbox } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.webhook-inspector.title'),
  path: '/webhook-inspector',
  description: translate('tools.webhook-inspector.description'),
  keywords: ['webhook', 'request bin', 'requestbin', 'inspect', 'callback', 'http request', 'debug', 'payload', 'hook'],
  component: () => import('./webhook-inspector.vue'),
  icon: Inbox,
  createdAt: new Date('2026-11-17'),
  runMode: 'server',
  capability: 'web.webhook',
})
