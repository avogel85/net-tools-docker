import { Share } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.social-preview.title'),
  path: '/social-preview',
  description: translate('tools.social-preview.description'),
  keywords: ['open graph', 'og:image', 'twitter card', 'social preview', 'link preview', 'meta tags', 'favicon', 'linkedin', 'facebook', 'slack'],
  component: () => import('./social-preview.vue'),
  icon: Share,
  createdAt: new Date('2026-11-17'),
  runMode: 'server',
  capability: 'web.social',
  accepts: { url: 'url' },
})
