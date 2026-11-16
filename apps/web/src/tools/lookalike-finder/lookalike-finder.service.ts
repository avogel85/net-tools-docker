import type { components } from '@/api/schema'

export type DnsLookalike = components['schemas']['DnsLookalike']
type Candidate = DnsLookalike['candidates'][number]

export function visible(cs: Candidate[], registeredOnly: boolean, kind: string): Candidate[] {
  return cs.filter(c => (!registeredOnly || c.registered) && (!kind || c.kind === kind))
}

/** A registered look-alike with mail servers can receive (and send) phishing mail. */
export function risk(c: Candidate): 'high' | 'medium' | 'low' {
  if (!c.registered)
    return 'low'
  return c.mx.length ? 'high' : 'medium'
}
