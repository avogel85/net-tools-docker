import type { components } from '@/api/schema'

export type DnsTakeover = components['schemas']['DnsTakeover']
export type Status = DnsTakeover['checks'][number]['status']

const ORDER: Record<Status, number> = { 'vulnerable': 0, 'dangling': 1, 'error': 2, 'ok': 3, 'no-cname': 4, 'nxdomain': 5 }

/** Most serious first; names that do not exist are hidden unless asked for. */
export function sortChecks(checks: DnsTakeover['checks'], showMissing: boolean) {
  return checks.filter(c => showMissing || c.status !== 'nxdomain').sort((a, b) => ORDER[a.status] - ORDER[b.status] || a.name.localeCompare(b.name))
}

export function counts(checks: DnsTakeover['checks']): Partial<Record<Status, number>> {
  const out: Partial<Record<Status, number>> = {}
  for (const c of checks) out[c.status] = (out[c.status] ?? 0) + 1
  return out
}
