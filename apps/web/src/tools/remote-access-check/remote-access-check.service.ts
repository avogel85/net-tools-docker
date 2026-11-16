import type { components } from '@/api/schema'

export type RemoteAccess = components['schemas']['RemoteAccess']
type Service = RemoteAccess['services'][number]

/** exposed: open with a finding; open: reachable; closed: not reachable. */
export function state(s: Service): 'exposed' | 'open' | 'closed' {
  if (!s.open)
    return 'closed'
  return s.findings.length ? 'exposed' : 'open'
}
