import type { components } from '@/api/schema'

export type DcLocator = components['schemas']['DcLocator']

/** One row per SRV lookup: what was asked, how many targets came back and whether that is a problem. */
export function lookupRows(r: DcLocator) {
  return r.lookups.map(l => ({
    id: l.id,
    name: l.name,
    port: l.port,
    rcode: l.rcode,
    targets: l.targets.map(t => (t.port === l.port ? t.target : `${t.target}:${t.port}`)).join(', ') || '—',
    status: l.targets.length ? 'ok' : l.required ? 'missing' : 'absent',
  }))
}

/** Default resolver: with DNS_RESOLVER set, "system" is a public resolver that cannot see internal AD zones. */
export function defaultResolver(dnsUpstream: boolean | undefined): string {
  return dnsUpstream ? 'local' : 'system'
}
