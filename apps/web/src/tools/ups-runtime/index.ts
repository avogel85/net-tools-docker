import { BatteryCharging } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.ups-runtime.title'),
  path: '/ups-runtime',
  description: translate('tools.ups-runtime.description'),
  keywords: ['ups', 'battery', 'runtime', 'backup time', 'va', 'watt', 'wh', 'inverter', 'power outage'],
  component: () => import('./ups-runtime.vue'),
  icon: BatteryCharging,
  createdAt: new Date('2026-10-22'),
  runMode: 'browser',
})
