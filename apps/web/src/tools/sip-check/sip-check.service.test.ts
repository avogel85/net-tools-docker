import { describe, expect, it } from 'vitest'
import { probeState } from './sip-check.service'

const p = (reachable: boolean, status: number) => ({ transport: 'udp', address: '', reachable, status, reason: '', server: '', allow: [], rttMs: 0, certError: '', error: '' }) as never

describe('sip-check', () => {
  it('classifies probes', () => {
    expect(probeState(p(true, 200))).toBe('ok')
    expect(probeState(p(true, 403))).toBe('answered')
    expect(probeState(p(false, 0))).toBe('down')
  })
})
