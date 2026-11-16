import { describe, expect, it } from 'vitest'
import { eModel, grade, packet, readPacket, stats } from './voip-quality.service'

describe('voip-quality', () => {
  it('round-trips packets', () => {
    const p = packet(7, 1234.5, 172)
    expect(p.byteLength).toBe(172)
    expect(readPacket(p)).toEqual({ seq: 7, sentAt: 1234.5 })
    expect(readPacket(new ArrayBuffer(4))).toBeUndefined()
    expect(packet(1, 0, 4).byteLength).toBe(12)
  })

  it('computes the E-model', () => {
    expect(eModel(0, 0, 0)).toEqual({ r: 93, mos: 4.4 })
    expect(eModel(20, 2, 0).mos).toBeGreaterThan(4.3)
    expect(eModel(200, 30, 0).mos).toBeLessThan(4)
    expect(eModel(20, 2, 5).r).toBeCloseTo(92.4 - 12.5, 0)
    expect(eModel(2000, 500, 50)).toEqual({ r: 0, mos: 1 })
    expect([4.4, 4.1, 3.7, 3.2, 2].map(grade)).toEqual(['excellent', 'good', 'fair', 'poor', 'bad'])
  })

  it('summarises a stream', () => {
    const s = stats([20, 22, 20, null, 21, 200], 40)!
    expect([s.sent, s.received, s.lost, s.late]).toEqual([6, 5, 1, 1])
    expect(s.lossPct).toBeCloseTo(33.33, 1)
    expect([s.minRtt, s.maxRtt]).toEqual([20, 200])
    expect(s.jitter).toBeGreaterThan(0)
    expect(stats(Array.from<number>({ length: 50 }).fill(10), 40)!.grade).toBe('excellent')
    expect(stats([null, null], 40)).toBeUndefined()
  })
})
