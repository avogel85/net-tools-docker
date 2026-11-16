import { AntennaBars5 } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.wifi-channels.title'),
  path: '/wifi-channels',
  description: translate('tools.wifi-channels.description'),
  keywords: ['wifi channels', 'channel plan', 'dfs', '5 ghz', '6 ghz', '2.4 ghz', 'non-overlapping', 'channel width', 'etsi', 'fcc', 'access points'],
  component: () => import('./wifi-channels.vue'),
  icon: AntennaBars5,
  createdAt: new Date('2026-11-17'),
  runMode: 'browser',
})
