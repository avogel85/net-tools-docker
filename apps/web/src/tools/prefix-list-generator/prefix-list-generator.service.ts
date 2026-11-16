import type { IrrResult } from '../irr-expansion/irr-expansion.service'

export type Prefix = IrrResult['prefixes'][number]
export type Platform = 'ios' | 'iosxr' | 'junos' | 'eos' | 'frr' | 'routeros' | 'bird' | 'openbgpd'
export const PLATFORMS: Platform[] = ['ios', 'iosxr', 'junos', 'eos', 'frr', 'routeros', 'bird', 'openbgpd']

const len = (p: string) => Number(p.split('/')[1])
const isV6 = (p: string) => p.includes(':')

/** The length range a prefix entry allows: [min, max]. */
export function range(p: Prefix): [number, number] {
  const l = len(p.prefix)
  if (p.exact)
    return [l, l]
  return [p.greaterEqual ?? l, p.lessEqual ?? (isV6(p.prefix) ? 128 : 32)]
}

function iosLine(p: Prefix): string {
  const [ge, le] = range(p)
  const l = len(p.prefix)
  return `${p.prefix}${ge > l ? ` ge ${ge}` : ''}${le > l ? ` le ${le}` : ''}`
}

/** Renders one family's prefix list for a platform. */
export function render(prefixes: Prefix[], platform: Platform, name: string, family: 4 | 6): string {
  const n = family === 6 ? `${name}-V6` : name
  const out: string[] = []
  switch (platform) {
    case 'ios':
    case 'eos':
    case 'frr': {
      const kw = family === 6 ? 'ipv6 prefix-list' : 'ip prefix-list'
      out.push(`no ${kw} ${n}`)
      if (!prefixes.length)
        out.push(`${kw} ${n} seq 5 deny ${family === 6 ? '::/0 le 128' : '0.0.0.0/0 le 32'}`)
      prefixes.forEach((p, i) => out.push(`${kw} ${n} seq ${(i + 1) * 5} permit ${iosLine(p)}`))
      break
    }
    case 'iosxr':
      out.push(`prefix-set ${n}`)
      prefixes.forEach((p, i) => {
        const [ge, le] = range(p)
        const l = len(p.prefix)
        out.push(`  ${p.prefix}${ge > l ? ` ge ${ge}` : ''}${le > l ? ` le ${le}` : ''}${i < prefixes.length - 1 ? ',' : ''}`)
      })
      out.push('end-set')
      break
    case 'junos':
      out.push(`delete policy-options route-filter-list ${n}`)
      for (const p of prefixes) {
        const [ge, le] = range(p)
        const l = len(p.prefix)
        const kind = ge === l && le === l ? 'exact' : ge === l ? `upto /${le}` : `prefix-length-range /${ge}-/${le}`
        out.push(`set policy-options route-filter-list ${n} ${p.prefix} ${kind}`)
      }
      break
    case 'routeros':
      for (const p of prefixes) {
        const [ge, le] = range(p)
        out.push(`/routing filter rule add chain=${n} rule="if (dst in ${p.prefix} && dst-len in ${ge}-${le}) { accept }"`)
      }
      out.push(`/routing filter rule add chain=${n} rule="reject"`)
      break
    case 'bird':
      out.push(`define ${n.replace(/-/g, '_')} = [`)
      prefixes.forEach((p, i) => {
        const [ge, le] = range(p)
        const l = len(p.prefix)
        out.push(`  ${p.prefix}${ge === l && le === l ? '' : `{${ge},${le}}`}${i < prefixes.length - 1 ? ',' : ''}`)
      })
      out.push('];')
      break
    case 'openbgpd':
      out.push(`prefix-set ${n} {`)
      for (const p of prefixes) {
        const [ge, le] = range(p)
        const l = len(p.prefix)
        out.push(`  ${p.prefix}${ge === l && le === l ? '' : ` prefixlen ${ge} - ${le}`}`)
      }
      out.push('}')
      break
  }
  return `${out.join('\n')}\n`
}

export function isValidListName(s: string): boolean {
  return /^[A-Z][\w-]{0,62}$/i.test(s)
}
