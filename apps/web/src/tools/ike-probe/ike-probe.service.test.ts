import type { IkeProbe } from './ike-probe.service'
import { describe, expect, it } from 'vitest'
import { verdict } from './ike-probe.service'

const v2 = { answered: false, accepted: false, encryption: '', prf: '', integrity: '', group: '', groupWeak: false, noProposal: false, cookie: false, fragmentation: false, sigHash: false, vendorIds: [] }
const v1 = { answered: false, accepted: false, encryption: '', hash: '', group: '', groupWeak: false, noProposal: false, vendorIds: [] }
const r = (a: Partial<typeof v2>, b: Partial<typeof v1>): IkeProbe => ({ host: '', address: '', findings: [], ikev2: { ...v2, ...a }, ikev1: { ...v1, ...b } })

describe('ike-probe', () => {
  it('derives a verdict', () => {
    expect(verdict(r({ answered: true, accepted: true }, {}))).toBe('ikev2')
    expect(verdict(r({}, { answered: true, accepted: true }))).toBe('ikev1-only')
    expect(verdict(r({ answered: true, noProposal: true }, {}))).toBe('answers')
    expect(verdict(r({}, {}))).toBe('silent')
  })
})
