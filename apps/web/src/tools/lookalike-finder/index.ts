import { Mask } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.lookalike-finder.title'),
  path: '/lookalike-finder',
  description: translate('tools.lookalike-finder.description'),
  keywords: ['typosquatting', 'lookalike', 'homoglyph', 'phishing', 'brand protection', 'dnstwist', 'idn', 'bitsquatting', 'domain variants'],
  component: () => import('./lookalike-finder.vue'),
  icon: Mask,
  createdAt: new Date('2026-11-16'),
  runMode: 'server',
  capability: 'dns.lookalike',
  accepts: { domain: 'domain' },
})
