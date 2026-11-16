import { ShieldLock } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.radius-tester.title'),
  path: '/radius-tester',
  description: translate('tools.radius-tester.description'),
  keywords: ['radius', 'aaa', '802.1x', 'nps', 'freeradius', 'shared secret', 'access-request', 'status-server', '1812', 'pap'],
  component: () => import('./radius-tester.vue'),
  icon: ShieldLock,
  createdAt: new Date('2026-11-17'),
  runMode: 'server',
  capability: 'lan.radius',
  accepts: { ip: 'host' },
})
