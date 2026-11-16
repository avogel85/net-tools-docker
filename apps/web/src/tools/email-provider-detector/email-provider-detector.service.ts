import type { components } from '@/api/schema'

export type EmailProviders = components['schemas']['EmailProviders']
type Detection = EmailProviders['detections'][number]
export type Role = Detection['role']

export const ROLES: Role[] = ['inbound', 'gateway', 'sending', 'reporting', 'verification']

/** Detections grouped by role, each provider once with all its evidence. */
export function groupDetections(ds: Detection[]): { role: Role, providers: { provider: string, evidence: string[] }[] }[] {
  return ROLES.map((role) => {
    const by = new Map<string, string[]>()
    for (const d of ds.filter(d => d.role === role))
      by.set(d.provider, [...(by.get(d.provider) ?? []), d.evidence])
    return { role, providers: [...by].map(([provider, evidence]) => ({ provider, evidence })) }
  }).filter(g => g.providers.length)
}
