import { describe, expect, it } from 'vitest'
import { formatDuration, runtime } from './ups-runtime.service'

const base = { wh: 0, volts: 0, ah: 0, efficiency: 85, depth: 80, watts: 0, va: 0, powerFactor: 0.9 }

describe('ups-runtime', () => {
  it('computes runtime from Wh and watts', () => {
    const r = runtime({ ...base, wh: 1000, watts: 200 })!
    expect(r.usableWh).toBeCloseTo(680)
    expect(r.minutes).toBeCloseTo(204)
  })
  it('derives Wh from V×Ah and watts from VA×PF', () => {
    const r = runtime({ ...base, volts: 12, ah: 100, va: 1000 })!
    expect(r.loadW).toBeCloseTo(900)
    expect(r.usableWh).toBeCloseTo(816)
  })
  it('returns undefined for incomplete input', () => {
    expect(runtime({ ...base, wh: 1000 })).toBeUndefined()
    expect(runtime({ ...base, wh: 1000, watts: 100, efficiency: 0 })).toBeUndefined()
  })
  it('formats durations', () => {
    expect(formatDuration(204)).toBe('3 h 24 min')
    expect(formatDuration(40)).toBe('40 min')
  })
})
