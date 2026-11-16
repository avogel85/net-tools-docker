import { PlugConnected } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.switch-cli.title'),
  path: '/switch-cli',
  description: translate('tools.switch-cli.description'),
  keywords: ['switch', 'cisco', 'juniper', 'junos', 'arista', 'eos', 'nx-os', 'ios', 'sfp', 'optics', 'dbm', 'transceiver', 'dom', 'fiber', 'light', 'port', 'flap', 'lldp', 'troubleshooting', 'cli', 'show'],
  component: () => import('./switch-cli.vue'),
  icon: PlugConnected,
  createdAt: new Date('2026-11-16'),
  runMode: 'browser',
})
