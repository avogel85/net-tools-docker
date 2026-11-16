import type { components } from '@/api/schema'

export type TlsPq = components['schemas']['TlsPq']

export type Verdict = 'pq' | 'pq-available' | 'classic' | 'unreachable'

/** pq: negotiated by default; pq-available: supported but a classic group wins by default. */
export function verdict(r: TlsPq): Verdict {
  if (r.error)
    return 'unreachable'
  if (r.negotiatedPq)
    return 'pq'
  return r.groups.some(g => g.pq && g.supported) ? 'pq-available' : 'classic'
}
