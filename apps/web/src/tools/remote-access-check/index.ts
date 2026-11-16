import { DeviceDesktop } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.remote-access-check.title'),
  path: '/remote-access-check',
  description: translate('tools.remote-access-check.description'),
  keywords: ['rdp', 'remote desktop', 'nla', 'credssp', 'vnc', 'rfb', 'ssh', 'banner', '3389', '5900', 'exposure'],
  component: () => import('./remote-access-check.vue'),
  icon: DeviceDesktop,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'con.remoteaccess',
  accepts: { domain: 'host', ip: 'host' },
})
