export interface ArpEntry { ip: string, mac: string }
export interface Finding { kind: 'duplicate-ip' | 'multi-ip' | 'mac-changed' | 'new-device' | 'gone-device', ip: string, detail: string }

const norm = (m: string) => m.trim().toLowerCase()

/**
 * Analyses one ARP scan. duplicate-ip: two MACs answered for one IP (a real conflict). multi-ip: one MAC
 * claims several IPs (normal for routers and hosts with aliases, informational). With a baseline it also
 * reports a changed MAC per IP, new devices and devices that went away.
 */
export function analyze(scan: ArpEntry[], baseline?: ArpEntry[]): Finding[] {
  const out: Finding[] = []
  const byIp = new Map<string, Set<string>>()
  const byMac = new Map<string, Set<string>>()
  for (const e of scan) {
    const mac = norm(e.mac)
    byIp.set(e.ip, (byIp.get(e.ip) ?? new Set()).add(mac))
    byMac.set(mac, (byMac.get(mac) ?? new Set()).add(e.ip))
  }
  for (const [ip, macs] of byIp) {
    if (macs.size > 1)
      out.push({ kind: 'duplicate-ip', ip, detail: [...macs].sort().join(' / ') })
  }
  for (const [mac, ips] of byMac) {
    if (ips.size > 1)
      out.push({ kind: 'multi-ip', ip: [...ips].sort().join(', '), detail: mac })
  }
  if (baseline) {
    const base = new Map(baseline.map(e => [e.ip, norm(e.mac)]))
    for (const [ip, macs] of byIp) {
      const was = base.get(ip)
      if (was === undefined)
        out.push({ kind: 'new-device', ip, detail: [...macs].join(' / ') })
      else if (!macs.has(was))
        out.push({ kind: 'mac-changed', ip, detail: `${was} → ${[...macs].join(' / ')}` })
    }
    for (const [ip, mac] of base) {
      if (!byIp.has(ip))
        out.push({ kind: 'gone-device', ip, detail: mac })
    }
  }
  const order = ['duplicate-ip', 'mac-changed', 'new-device', 'gone-device', 'multi-ip']
  return out.sort((a, b) => order.indexOf(a.kind) - order.indexOf(b.kind) || a.ip.localeCompare(b.ip, undefined, { numeric: true }))
}
