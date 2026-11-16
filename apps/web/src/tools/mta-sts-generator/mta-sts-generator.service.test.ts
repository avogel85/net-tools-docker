import { describe, expect, it } from 'vitest'
import { buildMtaSts, policyId, rptUri } from './mta-sts-generator.service'

const base = { domain: 'example.com', mode: 'enforce' as const, mx: ['mx1.example.com', '*.mail.example.net.', 'MX1.example.com'], maxAge: 604800, rua: ['tls@example.com'], id: '20261116120000' }

describe('mta-sts-generator', () => {
  it('builds policy, records and nginx snippet', () => {
    const r = buildMtaSts(base)
    expect(r.policy).toBe('version: STSv1\r\nmode: enforce\r\nmx: mx1.example.com\r\nmx: *.mail.example.net\r\nmax_age: 604800\r\n')
    expect(r.policyUrl).toBe('https://mta-sts.example.com/.well-known/mta-sts.txt')
    expect(r.records).toEqual([
      { name: '_mta-sts.example.com', value: 'v=STSv1; id=20261116120000' },
      { name: '_smtp._tls.example.com', value: 'v=TLSRPTv1; rua=mailto:tls@example.com' },
    ])
    expect(r.nginx).toContain('server_name mta-sts.example.com;')
    expect(r.nginx).toContain('return 200 "version: STSv1\\r\\nmode: enforce')
    expect(r.errors).toEqual([])
    expect(r.warnings).toEqual([])
  })

  it('omits mx lines for mode none and warns for testing', () => {
    expect(buildMtaSts({ ...base, mode: 'none', mx: [] }).policy).toBe('version: STSv1\r\nmode: none\r\nmax_age: 604800\r\n')
    expect(buildMtaSts({ ...base, mode: 'testing' }).warnings).toContain('testing')
  })

  it('validates input', () => {
    const r = buildMtaSts({ ...base, domain: 'bad', mx: ['not a host'], maxAge: 40000000, id: 'x-y', rua: ['nope'] })
    expect(r.errors.sort()).toEqual(['domain', 'id', 'max-age', 'mx', 'rua'])
    expect(buildMtaSts({ ...base, mx: [] }).errors).toEqual(['mx-missing'])
    expect(buildMtaSts({ ...base, maxAge: 86400 }).warnings).toEqual(['short-max-age'])
  })

  it('normalises report URIs and ids', () => {
    expect(rptUri('https://tlsrpt.example.com/v1')).toBe('https://tlsrpt.example.com/v1')
    expect(rptUri('mailto:a@b.test')).toBe('mailto:a@b.test')
    expect(rptUri('http://insecure.test')).toBeUndefined()
    expect(policyId(new Date('2026-11-16T09:08:07Z'))).toBe('20261116090807')
  })
})
