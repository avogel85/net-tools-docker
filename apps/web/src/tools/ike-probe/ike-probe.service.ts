import type { components } from '@/api/schema'

export type IkeProbe = components['schemas']['IkeProbe']

export type Verdict = 'ikev2' | 'ikev1-only' | 'answers' | 'silent'

export function verdict(r: IkeProbe): Verdict {
  if (r.ikev2.accepted)
    return 'ikev2'
  if (r.ikev1.accepted)
    return 'ikev1-only'
  if (r.ikev2.answered || r.ikev1.answered)
    return 'answers'
  return 'silent'
}
