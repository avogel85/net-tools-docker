import { Certificate } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.homelab-certs.title'),
  path: '/homelab-certs',
  description: translate('tools.homelab-certs.description'),
  keywords: ['homelab', 'certificate', 'expiry', 'internal', 'wildcard', 'tls', 'monitor'],
  component: () => import('./homelab-certs.vue'),
  icon: Certificate,
  createdAt: new Date('2026-10-22'),
  runMode: 'server',
  capability: 'homelab.certs',
})
