import { Route } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.reverse-proxy-config.title'),
  path: '/reverse-proxy-config',
  description: translate('tools.reverse-proxy-config.description'),
  keywords: ['reverse proxy', 'nginx', 'caddy', 'traefik', 'haproxy', 'config', 'tls', 'hsts', 'websocket', 'proxy_pass', 'labels'],
  component: () => import('./reverse-proxy-config.vue'),
  icon: Route,
  createdAt: new Date('2026-11-17'),
  runMode: 'browser',
})
