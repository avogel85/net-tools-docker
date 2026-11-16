import type { TlsAcme } from './acme-check.service'
import { describe, expect, it } from 'vitest'
import { isValidCertName, verdict } from './acme-check.service'

describe('acme-check', () => {
  it('validates certificate names', () => {
    expect(isValidCertName('www.example.com')).toBe(true)
    expect(isValidCertName('*.example.com')).toBe(true)
    expect(isValidCertName('a.*.example.com')).toBe(false)
    expect(isValidCertName('nodot')).toBe(false)
  })
  it('derives a verdict', () => {
    const base = { problems: [], warnings: [] } as unknown as TlsAcme
    expect(verdict(base)).toBe('ready')
    expect(verdict({ ...base, warnings: ['ipv6-used'] })).toBe('ready')
    expect(verdict({ ...base, warnings: ['stale-txt'] })).toBe('warnings')
    expect(verdict({ ...base, problems: ['caa-denies'] })).toBe('blocked')
  })
})
