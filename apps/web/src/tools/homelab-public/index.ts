import { Cloud } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.homelab-public.title'),
  path: '/homelab-public',
  description: translate('tools.homelab-public.description'),
  keywords: ['homelab', 'cloudflare', 'tunnel', 'public', 'hostname', 'uptime', 'dns', 'certificate'],
  component: () => import('./homelab-public.vue'),
  icon: Cloud,
  createdAt: new Date('2026-10-22'),
  runMode: 'server',
  capability: 'homelab.public-check',
})
