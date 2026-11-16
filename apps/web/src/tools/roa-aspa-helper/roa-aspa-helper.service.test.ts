import type { RoaCoverage } from '../roa-coverage/roa-coverage.service'
import { describe, expect, it } from 'vitest'
import { krillAspa, krillDelta, parseAspas, parsePath, roaCsv, roaPlan, verify, within } from './roa-aspa-helper.service'

const cov: RoaCoverage = {
  asn: 64500,
  name: 'TEST',
  valid: 2,
  invalid: 1,
  notFound: 1,
  unchecked: 0,
  validator: 'test',
  generated: '',
  prefixes: [
    { prefix: '192.0.2.0/24', status: 'not-found', reason: '', roa: '' },
    { prefix: '198.51.100.0/25', status: 'invalid', reason: 'length', roa: '198.51.100.0/24-24' },
    { prefix: '203.0.112.0/23', status: 'valid', reason: '', roa: '203.0.112.0/22-24' },
    { prefix: '203.0.114.0/24', status: 'valid', reason: '', roa: '203.0.112.0/22-24' },
  ],
  roas: [
    { prefix: '203.0.112.0/22', maxLength: 24, ta: 'ripe', used: true, loose: true },
    { prefix: '198.51.100.0/24', maxLength: 24, ta: 'ripe', used: false, loose: false },
  ],
}

describe('roa plan', () => {
  it('checks containment', () => {
    expect(within('203.0.114.0/24', '203.0.112.0/22')).toBe(true)
    expect(within('203.0.112.0/22', '203.0.112.0/23')).toBe(false)
    expect(within('2001:db8::/48', '192.0.2.0/24')).toBe(false)
  })

  it('builds minimal ROA changes', () => {
    const plan = roaPlan(cov)
    expect(plan.map(c => `${c.action} ${c.prefix}-${c.maxLength} ${c.why}`)).toEqual([
      'add 192.0.2.0/24-24 not-found',
      'add 198.51.100.0/25-25 invalid-length',
      'remove 203.0.112.0/22-24 loose',
      'add 203.0.112.0/23-23 loose',
      'add 203.0.114.0/24-24 loose',
      'remove 198.51.100.0/24-24 unused',
    ])
    expect(krillDelta(64500, plan)).toContain('R: 203.0.112.0/22-24 => 64500\n')
    expect(krillDelta(64500, plan)).toContain('A: 192.0.2.0/24 => 64500\n')
    expect(krillDelta(64500, plan)).toContain('# R: 198.51.100.0/24 => 64500')
    expect(roaCsv(64500, plan).split('\n')[1]).toBe('add,AS64500,192.0.2.0/24,24')
  })
})

describe('aspa', () => {
  const { aspas, invalid } = parseAspas('AS64500: AS64510, AS64511\n64510 => 64520\nAS64511: AS64520\n# comment\nAS64530: AS0\nbad line')
  it('parses records and paths', () => {
    expect([...aspas.keys()]).toEqual([64500, 64510, 64511, 64530])
    expect(invalid).toEqual(['bad line'])
    expect(parsePath('AS64520 64510 64510 64500')).toEqual([64520, 64510, 64500])
    expect(parsePath('x')).toBeUndefined()
    expect(krillAspa(64500, [64510])).toBe('krillc aspas add --aspa "AS64500 => AS64510"\n')
  })

  it('verifies upstream paths', () => {
    // 64500 → its provider 64510 → its provider 64520: a clean up-ramp.
    expect(verify(aspas, [64520, 64510, 64500], 'upstream').verdict).toBe('valid')
    // 64500 sends to 64599, which is not its provider: a leak.
    expect(verify(aspas, [64599, 64500], 'upstream').verdict).toBe('invalid')
    // 64599 has no ASPA: cannot tell.
    expect(verify(aspas, [64598, 64599], 'upstream').verdict).toBe('unknown')
    expect(verify(aspas, [64500], 'upstream').verdict).toBe('valid')
  })

  it('verifies downstream paths', () => {
    // Up 64500 → 64510, then one lateral (peer) hop to 64530.
    expect(verify(aspas, [64530, 64510, 64500], 'downstream').verdict).toBe('valid')
    // Leak: up to 64510, across to 64530, then on to 64511, which attests 64530 is not its customer.
    expect(verify(aspas, [64511, 64530, 64510, 64500], 'downstream').verdict).toBe('invalid')
    expect(verify(aspas, [64598, 64599, 64597], 'downstream').verdict).toBe('unknown')
    // Same shape without 64511's ASPA could be peer + down-ramp: unknown, not invalid.
    expect(verify(parseAspas('AS64500: AS64510\nAS64510: AS64520\nAS64530: AS0').aspas, [64511, 64530, 64510, 64500], 'downstream').verdict).toBe('unknown')
  })
})
