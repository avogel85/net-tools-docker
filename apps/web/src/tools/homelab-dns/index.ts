import { Equal } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.homelab-dns.title'),
  path: '/homelab-dns',
  description: translate('tools.homelab-dns.description'),
  keywords: ['homelab', 'dns', 'technitium', 'compare', 'resolver', 'split horizon', 'dnssec'],
  component: () => import('./homelab-dns.vue'),
  icon: Equal,
  createdAt: new Date('2026-10-22'),
  runMode: 'server',
  capability: 'homelab.dns-compare',
})
