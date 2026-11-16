import type { components } from '@/api/schema'

export type HttpLinks = components['schemas']['HttpLinks']
type Link = HttpLinks['links'][number]

/** Short status text: HTTP code, the error, or both. */
export function statusText(l: Link): string {
  if (l.error)
    return l.status ? `${l.status} · ${l.error}` : l.error
  return l.redirects ? `${l.status} (${l.redirects} redirect${l.redirects > 1 ? 's' : ''})` : String(l.status)
}

export function filterLinks(links: Link[], onlyBroken: boolean, kind: string): Link[] {
  return links.filter(l => (!onlyBroken || l.broken) && (!kind || l.kind === kind))
}
