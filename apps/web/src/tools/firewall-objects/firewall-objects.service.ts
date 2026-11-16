import type { Cidr } from '@/utils/ip'
import { aggregateCidrs, cidrToString, formatIP, hostMask, parseCidr, parseIP, prefixMask, rangeToCidrs } from '@/utils/ip'

export type Platform = 'asa' | 'ios' | 'fortigate' | 'paloalto' | 'juniper' | 'mikrotik' | 'pfsense' | 'ipset' | 'nftables'
export const PLATFORMS: Platform[] = ['fortigate', 'asa', 'ios', 'paloalto', 'juniper', 'mikrotik', 'pfsense', 'ipset', 'nftables']

export interface Entry { cidr: Cidr, comment: string }
export interface Parsed { entries: Entry[], invalid: { line: number, text: string }[] }

/** Lines of IPs, CIDRs or ranges (a-b), with an optional "# comment"; commas and spaces separate items. */
export function parseList(text: string): Parsed {
  const entries: Entry[] = []
  const invalid: Parsed['invalid'] = []
  text.split('\n').forEach((raw, i) => {
    const [body, ...rest] = raw.split('#')
    const comment = rest.join('#').trim()
    for (const item of body!.split(/[\s,;]+/).filter(Boolean)) {
      const dash = item.indexOf('-')
      if (dash > 0 && !item.includes('/')) {
        const a = parseIP(item.slice(0, dash))
        const b = parseIP(item.slice(dash + 1))
        if (a && b && a.version === b.version && a.value <= b.value) {
          for (const c of rangeToCidrs(a.version, a.value, b.value)) entries.push({ cidr: c, comment })
          continue
        }
      }
      const c = parseCidr(item)
      if (c)
        entries.push({ cidr: { ...c, address: c.network }, comment })
      else
        invalid.push({ line: i + 1, text: item })
    }
  })
  return { entries, invalid }
}

/** Removes duplicates and, with merge, combines adjacent and overlapping networks. */
export function normalize(entries: Entry[], merge: boolean): Entry[] {
  const key = (c: Cidr) => `${c.version}|${cidrToString(c)}`
  if (merge) {
    const comments = new Map<string, string>()
    for (const e of entries) {
      if (e.comment && !comments.has(key(e.cidr)))
        comments.set(key(e.cidr), e.comment)
    }
    return aggregateCidrs(entries.map(e => e.cidr)).map(c => ({ cidr: c, comment: comments.get(key(c)) ?? '' }))
  }
  const out = new Map<string, Entry>()
  for (const e of entries) {
    const k = key(e.cidr)
    const prev = out.get(k)
    if (!prev)
      out.set(k, e)
    else if (!prev.comment && e.comment)
      out.set(k, { ...prev, comment: e.comment })
  }
  return [...out.values()]
}

const host = (c: Cidr) => c.prefix === (c.version === 4 ? 32 : 128)
const net = (c: Cidr) => formatIP(c.version, c.network)
const mask = (c: Cidr) => formatIP(4, prefixMask(4, c.prefix))
const wildcard = (c: Cidr) => formatIP(4, hostMask(4, c.prefix))
const objName = (prefix: string, c: Cidr) => `${prefix}_${cidrToString(c).replace(/[:/.]/g, '_')}`.replace(/_+/g, '_').slice(0, 63)
const q = (s: string) => s.replace(/"/g, '')

export function render(entries: Entry[], platform: Platform, name: string): string {
  const v4 = entries.filter(e => e.cidr.version === 4)
  const v6 = entries.filter(e => e.cidr.version === 6)
  const n = name || 'NETTOOLS'
  const out: string[] = []
  switch (platform) {
    case 'asa':
      out.push(`object-group network ${n}`)
      for (const { cidr: c, comment } of entries) {
        if (comment)
          out.push(` description ${q(comment)}`.slice(0, 200))
        out.push(c.version === 4 ? (host(c) ? ` network-object host ${net(c)}` : ` network-object ${net(c)} ${mask(c)}`) : (host(c) ? ` network-object host ${net(c)}` : ` network-object ${cidrToString(c)}`))
      }
      break
    case 'ios':
      if (v4.length) {
        out.push(`ip access-list standard ${n}`)
        for (const { cidr: c, comment } of v4) {
          if (comment)
            out.push(` remark ${q(comment)}`)
          out.push(host(c) ? ` permit host ${net(c)}` : ` permit ${net(c)} ${wildcard(c)}`)
        }
      }
      if (v6.length) {
        out.push(`ipv6 access-list ${n}_V6`)
        for (const { cidr: c } of v6) out.push(` permit ipv6 ${host(c) ? `host ${net(c)}` : cidrToString(c)} any`)
      }
      break
    case 'fortigate':
      if (v4.length) {
        out.push('config firewall address')
        for (const { cidr: c, comment } of v4) out.push(`    edit "${objName(n, c)}"`, `        set subnet ${net(c)} ${mask(c)}`, ...(comment ? [`        set comment "${q(comment)}"`] : []), '    next')
        out.push('end', 'config firewall addrgrp', `    edit "${n}"`, `        set member ${v4.map(e => `"${objName(n, e.cidr)}"`).join(' ')}`, '    next', 'end')
      }
      if (v6.length) {
        out.push('config firewall address6')
        for (const { cidr: c } of v6) out.push(`    edit "${objName(n, c)}"`, `        set ip6 ${cidrToString(c)}`, '    next')
        out.push('end', 'config firewall addrgrp6', `    edit "${n}_V6"`, `        set member ${v6.map(e => `"${objName(n, e.cidr)}"`).join(' ')}`, '    next', 'end')
      }
      break
    case 'paloalto':
      for (const { cidr: c, comment } of entries) out.push(`set address ${objName(n, c)} ip-netmask ${cidrToString(c)}${comment ? ` description "${q(comment)}"` : ''}`)
      out.push(`set address-group ${n} static [ ${entries.map(e => objName(n, e.cidr)).join(' ')} ]`)
      break
    case 'juniper':
      for (const { cidr: c } of entries) out.push(`set security address-book global address ${objName(n, c)} ${cidrToString(c)}`)
      for (const { cidr: c } of entries) out.push(`set security address-book global address-set ${n} address ${objName(n, c)}`)
      break
    case 'mikrotik':
      for (const { cidr: c, comment } of entries) out.push(`/${c.version === 4 ? 'ip' : 'ipv6'} firewall address-list add list=${n} address=${host(c) && c.version === 4 ? net(c) : cidrToString(c)}${comment ? ` comment="${q(comment)}"` : ''}`)
      break
    case 'pfsense':
      for (const { cidr: c, comment } of entries) out.push(`${host(c) ? net(c) : cidrToString(c)}${comment ? ` # ${comment}` : ''}`)
      break
    case 'ipset':
      if (v4.length)
        out.push(`ipset create ${n} hash:net family inet -exist`, ...v4.map(e => `ipset add ${n} ${cidrToString(e.cidr)} -exist`))
      if (v6.length)
        out.push(`ipset create ${n}_v6 hash:net family inet6 -exist`, ...v6.map(e => `ipset add ${n}_v6 ${cidrToString(e.cidr)} -exist`))
      break
    case 'nftables':
      if (v4.length)
        out.push(`set ${n.toLowerCase()}_v4 {`, '    type ipv4_addr', '    flags interval', `    elements = { ${v4.map(e => cidrToString(e.cidr)).join(', ')} }`, '}')
      if (v6.length)
        out.push(`set ${n.toLowerCase()}_v6 {`, '    type ipv6_addr', '    flags interval', `    elements = { ${v6.map(e => cidrToString(e.cidr)).join(', ')} }`, '}')
      break
  }
  return out.length ? `${out.join('\n')}\n` : ''
}

export function isValidName(s: string): boolean {
  return /^[A-Z_][\w-]{0,40}$/i.test(s)
}
