import type { components } from '@/api/schema'

export type LanRadius = components['schemas']['LanRadius']

export type Verdict = 'ok' | 'rejected' | 'untrusted' | 'silent' | 'closed'

/** What the reply means for the operator. */
export function verdict(r: LanRadius): Verdict {
  if (r.result === 'refused')
    return 'closed'
  if (!r.answered)
    return 'silent'
  if (!r.responseAuthOk || r.messageAuth === 'bad')
    return 'untrusted'
  return r.result === 'reject' ? 'rejected' : 'ok'
}
