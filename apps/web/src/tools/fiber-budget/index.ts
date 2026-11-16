import { Ruler } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.fiber-budget.title'),
  path: '/fiber-budget',
  description: translate('tools.fiber-budget.description'),
  keywords: ['fiber', 'fibre', 'loss budget', 'link budget', 'db', 'dbm', 'attenuation', 'sfp', 'single mode', 'multimode', 'om3', 'om4', 'os2', 'splice', 'connector'],
  component: () => import('./fiber-budget.vue'),
  icon: Ruler,
  createdAt: new Date('2026-11-17'),
  runMode: 'browser',
})
