import type { TlsPq } from './pq-tls-check.service'
import { describe, expect, it } from 'vitest'
import { verdict } from './pq-tls-check.service'

const base: TlsPq = { host: 'x', address: '', negotiated: 'X25519', negotiatedPq: false, version: 'TLS 1.3', groups: [], error: '' }

describe('pq-tls-check', () => {
  it('derives the verdict', () => {
    expect(verdict({ ...base, negotiated: 'X25519MLKEM768', negotiatedPq: true })).toBe('pq')
    expect(verdict({ ...base, groups: [{ name: 'X25519MLKEM768', pq: true, supported: true }] })).toBe('pq-available')
    expect(verdict({ ...base, groups: [{ name: 'X25519MLKEM768', pq: true, supported: false }] })).toBe('classic')
    expect(verdict({ ...base, error: 'timeout' })).toBe('unreachable')
  })
})
