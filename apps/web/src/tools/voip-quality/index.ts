import { PhoneCall } from '@vicons/tabler'
import { translate } from '@/plugins/i18n.plugin'
import { defineTool } from '../tool'

export const tool = defineTool({
  name: translate('tools.voip-quality.title'),
  path: '/voip-quality',
  description: translate('tools.voip-quality.description'),
  keywords: ['voip', 'mos', 'jitter', 'voice quality', 'sip', 'teams', 'zoom', 'e-model', 'r-factor', 'g.711', 'opus', 'call quality'],
  component: () => import('./voip-quality.vue'),
  icon: PhoneCall,
  createdAt: new Date('2026-11-17'),
  runMode: 'server',
  capability: 'net.voip',
})
