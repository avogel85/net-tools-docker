import { Filter } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.capture-filter.title'),
  path: '/capture-filter',
  description: translate('tools.capture-filter.description'),
  keywords: ['tcpdump', 'bpf', 'capture filter', 'wireshark', 'display filter', 'tshark', 'pcap', 'packet capture', 'sniff'],
  component: () => import('./capture-filter.vue'),
  icon: Filter,
  createdAt: new Date('2026-11-17'),
  runMode: 'browser',
})
