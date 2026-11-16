import { describe, expect, it } from 'vitest'
import { functionalLevel, rootDseRows } from './ldap-check.service'

describe('ldap-check', () => {
  it('orders rootDSE attributes', () => {
    const rows = rootDseRows({ zzz: ['1'], defaultnamingcontext: ['DC=x'], dnsHostName: ['dc1'] })
    expect(rows.map(r => r.key)).toEqual(['dnsHostName', 'defaultnamingcontext', 'zzz'])
  })
  it('names functional levels', () => {
    expect(functionalLevel('7')).toBe('7 (Windows Server 2016)')
    expect(functionalLevel('x')).toBe('x')
  })
})
