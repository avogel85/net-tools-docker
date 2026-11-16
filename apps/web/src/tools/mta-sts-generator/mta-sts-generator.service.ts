import type { ListedRecord } from '@/components/record-list'

export type Mode = 'testing' | 'enforce' | 'none'

export interface MtaStsInput {
  domain: string
  mode: Mode
  mx: string[]
  /** Seconds; RFC 8461 allows up to 31557600. */
  maxAge: number
  /** TLS-RPT report destinations (mailto: or https:). */
  rua: string[]
  /** Policy id; any 1–32 alphanumeric string, changed whenever the policy changes. */
  id: string
}

export interface MtaStsResult {
  policy: string
  policyUrl: string
  records: ListedRecord[]
  nginx: string
  errors: string[]
  warnings: string[]
}

export const MAX_AGE_LIMIT = 31557600

const HOST = /^(?:\*\.)?(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]*$/i

/** A policy id from the current time (YYYYMMDDhhmmss), as most guides recommend. */
export function policyId(now = new Date()): string {
  return now.toISOString().replace(/\D/g, '').slice(0, 14)
}

export function rptUri(s: string): string | undefined {
  const v = s.trim()
  if (/^https:\/\/[^\s/]+\.[^\s/]+/i.test(v))
    return v
  const mail = v.replace(/^mailto:/i, '')
  return /^[^\s@,;]+@[a-z0-9-]+(?:\.[a-z0-9-]+)+$/i.test(mail) ? `mailto:${mail}` : undefined
}

export function buildMtaSts(i: MtaStsInput): MtaStsResult {
  const domain = i.domain.trim().toLowerCase().replace(/\.$/, '')
  const errors: string[] = []
  const warnings: string[] = []
  if (!HOST.test(domain) || domain.startsWith('*'))
    errors.push('domain')
  const mx = [...new Set(i.mx.map(m => m.trim().toLowerCase().replace(/\.$/, '')).filter(Boolean))]
  if (i.mode !== 'none' && !mx.length)
    errors.push('mx-missing')
  if (mx.some(m => !HOST.test(m)))
    errors.push('mx')
  if (!Number.isInteger(i.maxAge) || i.maxAge < 0 || i.maxAge > MAX_AGE_LIMIT)
    errors.push('max-age')
  if (!/^[a-z0-9]{1,32}$/i.test(i.id))
    errors.push('id')
  if (i.mode === 'enforce' && i.maxAge < 604800)
    warnings.push('short-max-age')
  if (i.mode === 'testing')
    warnings.push('testing')
  const rua = i.rua.filter(r => r.trim()).map(rptUri)
  if (rua.includes(undefined))
    errors.push('rua')
  if (!rua.length)
    warnings.push('no-rua')

  const lines = ['version: STSv1', `mode: ${i.mode}`, ...(i.mode === 'none' ? [] : mx.map(m => `mx: ${m}`)), `max_age: ${i.maxAge}`]
  const policy = `${lines.join('\r\n')}\r\n`
  const host = `mta-sts.${domain || 'example.com'}`
  const records: ListedRecord[] = [{ name: `_mta-sts.${domain || 'example.com'}`, value: `v=STSv1; id=${i.id}` }]
  const okRua = rua.filter((r): r is string => !!r)
  if (okRua.length)
    records.push({ name: `_smtp._tls.${domain || 'example.com'}`, value: `v=TLSRPTv1; rua=${okRua.join(',')}` })
  const nginx = [
    'server {',
    '    listen 443 ssl;',
    '    listen [::]:443 ssl;',
    `    server_name ${host};`,
    `    # ssl_certificate     /etc/ssl/${host}/fullchain.pem;`,
    `    # ssl_certificate_key /etc/ssl/${host}/privkey.pem;`,
    '    location = /.well-known/mta-sts.txt {',
    '        default_type text/plain;',
    `        return 200 "${policy.replace(/\r\n/g, '\\r\\n')}";`,
    '    }',
    '    location / { return 404; }',
    '}',
  ].join('\n')
  return { policy, policyUrl: `https://${host}/.well-known/mta-sts.txt`, records, nginx, errors: [...new Set(errors)], warnings }
}
