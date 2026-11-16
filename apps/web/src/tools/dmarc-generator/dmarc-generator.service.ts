export type Policy = 'none' | 'quarantine' | 'reject'
export type Alignment = 'r' | 's'
export type FailureOption = '0' | '1' | 'd' | 's'

export interface DmarcInput {
  domain: string
  p: Policy
  /** Subdomain policy; empty inherits p. */
  sp: Policy | ''
  adkim: Alignment
  aspf: Alignment
  pct: number
  rua: string[]
  ruf: string[]
  fo: FailureOption[]
  /** Aggregate report interval in seconds. */
  ri: number
}

export interface DmarcRecord { name: string, value: string }

export interface DmarcResult {
  record: DmarcRecord
  /** Authorization records the external report domains must publish (RFC 7489 §7.1). */
  authorizations: DmarcRecord[]
  warnings: string[]
  errors: string[]
}

export const DEFAULTS: DmarcInput = { domain: '', p: 'none', sp: '', adkim: 'r', aspf: 'r', pct: 100, rua: [], ruf: [], fo: ['0'], ri: 86400 }

const EMAIL = /^[^\s@,;!]+@([a-z0-9-]+(?:\.[a-z0-9-]+)+)$/i

/** Normalises a report address to a mailto: URI; undefined when it is not an e-mail address. */
export function reportUri(addr: string): { uri: string, domain: string } | undefined {
  const a = addr.trim().replace(/^mailto:/i, '')
  const [mail, size] = a.split('!')
  const m = EMAIL.exec(mail ?? '')
  if (!m || (size !== undefined && !/^\d+[kmgt]?$/i.test(size)))
    return undefined
  return { uri: `mailto:${a}`, domain: m[1]!.toLowerCase() }
}

/** The organizational domain approximation used for "external" checks: the last two labels. */
function sameOrg(a: string, b: string): boolean {
  const tail = (d: string) => d.toLowerCase().split('.').slice(-2).join('.')
  return tail(a) === tail(b)
}

export function buildDmarc(input: DmarcInput): DmarcResult {
  const domain = input.domain.trim().toLowerCase().replace(/\.$/, '')
  const errors: string[] = []
  const warnings: string[] = []
  const tags = [`v=DMARC1`, `p=${input.p}`]
  if (input.sp && input.sp !== input.p)
    tags.push(`sp=${input.sp}`)
  if (input.adkim === 's')
    tags.push('adkim=s')
  if (input.aspf === 's')
    tags.push('aspf=s')
  const pct = Math.round(input.pct)
  if (!Number.isFinite(pct) || pct < 0 || pct > 100)
    errors.push('pct')
  else if (pct !== 100)
    tags.push(`pct=${pct}`)

  const authorizations: DmarcRecord[] = []
  const uris = (list: string[], tag: 'rua' | 'ruf') => {
    const out: string[] = []
    for (const addr of list.filter(a => a.trim())) {
      const r = reportUri(addr)
      if (!r) {
        errors.push(tag)
        continue
      }
      out.push(r.uri)
      if (domain && !sameOrg(r.domain, domain) && !authorizations.some(a => a.name.endsWith(`._report._dmarc.${r.domain}`)))
        authorizations.push({ name: `${domain}._report._dmarc.${r.domain}`, value: 'v=DMARC1' })
    }
    if (out.length)
      tags.push(`${tag}=${out.join(',')}`)
    return out.length
  }
  const nRua = uris(input.rua, 'rua')
  const nRuf = uris(input.ruf, 'ruf')
  const fo = [...new Set(input.fo)].filter(o => o !== '0' || input.fo.length === 1)
  if (nRuf && fo.length && !(fo.length === 1 && fo[0] === '0'))
    tags.push(`fo=${fo.join(':')}`)
  if (input.ri !== 86400 && input.ri > 0)
    tags.push(`ri=${Math.round(input.ri)}`)

  if (!domain || !/^[a-z0-9-]+(?:\.[a-z0-9-]+)+$/.test(domain))
    errors.push('domain')
  if (input.p === 'none')
    warnings.push('policy-none')
  if (pct < 100 && input.p !== 'none')
    warnings.push('partial-pct')
  if (!nRua)
    warnings.push('no-rua')
  if (nRuf)
    warnings.push('ruf-privacy')
  if (input.sp === 'none' && input.p !== 'none')
    warnings.push('sp-none')
  return { record: { name: `_dmarc.${domain || 'example.com'}`, value: tags.join('; ') }, authorizations, warnings, errors: [...new Set(errors)] }
}

/** Reads an existing DMARC record back into generator input (unknown tags are ignored). */
export function parseDmarc(text: string, domain = ''): DmarcInput | undefined {
  const t = text.trim().replace(/^"|"$/g, '')
  if (!/^v\s*=\s*DMARC1\b/i.test(t))
    return undefined
  const tags = new Map<string, string>()
  for (const part of t.split(';')) {
    const [k, ...v] = part.split('=')
    if (k && v.length)
      tags.set(k.trim().toLowerCase(), v.join('=').trim())
  }
  const pol = (v?: string): Policy | undefined => (v === 'none' || v === 'quarantine' || v === 'reject' ? v : undefined)
  const list = (v?: string) => (v ? v.split(',').map(s => s.trim().replace(/^mailto:/i, '')).filter(Boolean) : [])
  const fo = (tags.get('fo') ?? '0').split(':').filter((o): o is FailureOption => ['0', '1', 'd', 's'].includes(o))
  return {
    domain,
    p: pol(tags.get('p')?.toLowerCase()) ?? 'none',
    sp: pol(tags.get('sp')?.toLowerCase()) ?? '',
    adkim: tags.get('adkim') === 's' ? 's' : 'r',
    aspf: tags.get('aspf') === 's' ? 's' : 'r',
    pct: Number(tags.get('pct') ?? 100),
    rua: list(tags.get('rua')),
    ruf: list(tags.get('ruf')),
    fo: fo.length ? fo : ['0'],
    ri: Number(tags.get('ri') ?? 86400),
  }
}
