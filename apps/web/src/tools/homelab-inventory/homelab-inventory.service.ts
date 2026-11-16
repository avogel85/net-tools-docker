export interface Guest { node: string, vmid: number, type: string, name: string, status: string, ip?: string }
export interface Host { ip: string, mac: string }
export interface NamedHost extends Host { name: string }
export interface UniFiLike { mac: string, name: string }

const mac = (m: string) => m.trim().toLowerCase()

/** MAC → friendly name, from UniFi clients and devices. */
export function nameIndex(...lists: UniFiLike[][]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const l of lists) {
    for (const e of l) {
      if (e.mac && e.name)
        out[mac(e.mac)] = e.name
    }
  }
  return out
}

/** ARP-visible hosts whose IP no running Proxmox guest (or known extra address) claims, with a UniFi name when known. */
export function unknownHosts(arp: Host[], guests: Guest[], known: string[] = [], names: Record<string, string> = {}): NamedHost[] {
  const owned = new Set([...guests.map(g => g.ip).filter(Boolean), ...known])
  return arp
    .filter(h => !owned.has(h.ip))
    .map(h => ({ ...h, name: names[mac(h.mac)] ?? '' }))
    .sort((a, b) => a.ip.localeCompare(b.ip, undefined, { numeric: true }))
}

export function parseKnown(s: string): string[] {
  return s.split(/[\s,;]+/).map(x => x.trim()).filter(Boolean)
}
