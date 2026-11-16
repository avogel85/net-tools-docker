import type { RemoteProbeResult } from '@/composables/useRemote'
import { hopRows } from '@/components/remote'

/** The networks (AS numbers) a probe's path crosses, in order, without repeats. */
export function asPathOf(p: Pick<RemoteProbeResult, 'result'>): number[] {
  const out: number[] = []
  for (const h of hopRows(p.result)) {
    for (const a of h.asn.split(' ').filter(Boolean)) {
      const asn = Number(a.replace(/^AS/, ''))
      if (asn && out.at(-1) !== asn)
        out.push(asn)
    }
  }
  return out
}

/** Locations as Globalping "magic" strings; empty means "anywhere". */
export function locationList(s: string): string[] {
  return s.split(',').map(x => x.trim()).filter(Boolean).slice(0, 5)
}
