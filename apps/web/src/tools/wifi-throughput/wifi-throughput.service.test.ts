import { describe, expect, it } from 'vitest'
import { phyRate, STANDARDS, table } from './wifi-throughput.service'

const std = (id: string) => STANDARDS.find(s => s.id === id)!

describe('wifi-throughput', () => {
  it('matches published rates', () => {
    expect(phyRate(std('n'), 20, 7, 1, 0.8)).toBe(65)
    expect(phyRate(std('n'), 40, 7, 2, 0.4)).toBe(300)
    expect(phyRate(std('ac'), 80, 9, 1, 0.4)).toBe(433.3)
    expect(phyRate(std('ac'), 160, 9, 2, 0.4)).toBe(1733.3)
    expect(phyRate(std('ax'), 80, 11, 1, 0.8)).toBe(600.5)
    expect(phyRate(std('ax'), 160, 11, 2, 0.8)).toBe(2402)
    expect(phyRate(std('be'), 320, 13, 2, 0.8)).toBe(5764.7)
  })

  it('rejects undefined combinations', () => {
    expect(phyRate(std('ac'), 20, 9, 1, 0.4)).toBeUndefined()
    expect(phyRate(std('ac'), 20, 9, 3, 0.4)).toBeDefined()
    expect(phyRate(std('n'), 80, 0, 1, 0.8)).toBeUndefined()
    expect(phyRate(std('ax'), 80, 12, 1, 0.8)).toBeUndefined()
    expect(phyRate(std('ax'), 80, 0, 1, 0.4)).toBeUndefined()
  })

  it('builds the MCS table', () => {
    const t = table(std('ax'), 80, 2, 0.8)
    expect(t).toHaveLength(12)
    expect(t[11]!.mod).toBe('1024-QAM')
  })
})
