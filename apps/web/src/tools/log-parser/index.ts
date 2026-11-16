import { FileSearch } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.log-parser.title'),
  path: '/log-parser',
  description: translate('tools.log-parser.description'),
  keywords: ['log', 'syslog', 'rfc 5424', 'rfc 3164', 'fortigate', 'cisco asa', 'nginx', 'apache', 'access log', 'iptables', 'parse', 'pri', 'facility', 'severity'],
  component: () => import('./log-parser.vue'),
  icon: FileSearch,
  createdAt: new Date('2026-11-17'),
  runMode: 'browser',
})
