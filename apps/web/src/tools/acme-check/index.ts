import { Certificate } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.acme-check.title'),
  path: '/acme-check',
  description: translate('tools.acme-check.description'),
  keywords: ['acme', 'let\'s encrypt', 'letsencrypt', 'certbot', 'http-01', 'dns-01', 'caa', 'certificate', 'challenge', 'zerossl'],
  component: () => import('./acme-check.vue'),
  icon: Certificate,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'tls.acme',
  accepts: { domain: 'domain' },
})
