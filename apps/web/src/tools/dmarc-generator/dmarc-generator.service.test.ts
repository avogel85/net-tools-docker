import { describe, expect, it } from 'vitest'
import { buildDmarc, DEFAULTS, parseDmarc, reportUri } from './dmarc-generator.service'

describe('dmarc-generator', () => {
  it('builds a minimal record with defaults left out', () => {
    const r = buildDmarc({ ...DEFAULTS, domain: 'example.com', rua: ['dmarc@example.com'] })
    expect(r.record).toEqual({ name: '_dmarc.example.com', value: 'v=DMARC1; p=none; rua=mailto:dmarc@example.com' })
    expect(r.warnings).toEqual(['policy-none'])
    expect(r.errors).toEqual([])
  })

  it('includes every non-default tag', () => {
    const r = buildDmarc({ ...DEFAULTS, domain: 'example.com', p: 'reject', sp: 'quarantine', adkim: 's', aspf: 's', pct: 50, rua: ['a@example.com'], ruf: ['f@example.com'], fo: ['1', 'd'], ri: 3600 })
    expect(r.record.value).toBe('v=DMARC1; p=reject; sp=quarantine; adkim=s; aspf=s; pct=50; rua=mailto:a@example.com; ruf=mailto:f@example.com; fo=1:d; ri=3600')
    expect(r.warnings).toEqual(['partial-pct', 'ruf-privacy'])
  })

  it('lists authorization records for external report domains', () => {
    const r = buildDmarc({ ...DEFAULTS, domain: 'shop.example.com', p: 'quarantine', rua: ['x@dmarc.vendor.test', 'y@dmarc.vendor.test', 'own@example.com'] })
    expect(r.authorizations).toEqual([{ name: 'shop.example.com._report._dmarc.dmarc.vendor.test', value: 'v=DMARC1' }])
  })

  it('flags invalid input', () => {
    const r = buildDmarc({ ...DEFAULTS, domain: 'nodot', rua: ['not-an-address'], pct: 120 })
    expect(r.errors.sort()).toEqual(['domain', 'pct', 'rua'])
  })

  it('accepts a size limit on report URIs', () => {
    expect(reportUri('mailto:a@example.com!10m')).toEqual({ uri: 'mailto:a@example.com!10m', domain: 'example.com' })
    expect(reportUri('a@example.com!ten')).toBeUndefined()
  })

  it('round-trips an existing record', () => {
    const text = 'v=DMARC1; p=reject; sp=none; adkim=s; pct=25; rua=mailto:a@example.com,mailto:b@example.com; fo=1'
    const input = parseDmarc(text, 'example.com')!
    expect(input).toMatchObject({ p: 'reject', sp: 'none', adkim: 's', aspf: 'r', pct: 25, rua: ['a@example.com', 'b@example.com'], fo: ['1'] })
    expect(buildDmarc(input).record.value).toBe('v=DMARC1; p=reject; sp=none; adkim=s; pct=25; rua=mailto:a@example.com,mailto:b@example.com')
    expect(parseDmarc('v=spf1 -all')).toBeUndefined()
  })
})
