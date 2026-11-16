import type { components } from '@/api/schema'

export type SipCheck = components['schemas']['SipCheck']
type Probe = SipCheck['probes'][number]

/** ok: answered 2xx; answered: any SIP reply (e.g. 403/404 — the server is there); down: no SIP reply. */
export function probeState(p: Probe): 'ok' | 'answered' | 'down' {
  if (!p.reachable || !p.status)
    return 'down'
  return p.status >= 200 && p.status < 300 ? 'ok' : 'answered'
}
