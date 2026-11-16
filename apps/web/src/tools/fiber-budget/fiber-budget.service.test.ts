import type { BudgetInput } from './fiber-budget.service'
import { describe, expect, it } from 'vitest'
import { calculate } from './fiber-budget.service'

const base: BudgetInput = { fiber: 'os2-1310', lengthM: 8000, connectors: 2, connectorDb: 0.5, splices: 4, spliceDb: 0.1, marginDb: 3, tx: -8.2, rx: -14.4, overload: 0.5, optic: '10gbase-lr' }

describe('fiber-budget', () => {
  it('computes losses and margin', () => {
    const b = calculate(base)
    expect([b.fiberLoss, b.connectorLoss, b.spliceLoss, b.totalLoss]).toEqual([3.2, 1, 0.4, 4.6])
    expect(b.powerBudget).toBe(6.2)
    expect(b.margin).toBe(-1.4)
    expect(b.rxPower).toBe(-12.8)
    expect(b.verdict).toBe('tight')
    expect(b.warnings).toEqual([])
  })

  it('flags failures, reach, mode and overload', () => {
    expect(calculate({ ...base, lengthM: 20000 }).verdict).toBe('fail')
    expect(calculate({ ...base, lengthM: 20000 }).warnings).toContain('reach')
    expect(calculate({ ...base, fiber: 'om4-850' }).warnings).toContain('mode-mismatch')
    expect(calculate({ ...base, optic: '10gbase-zr', tx: 0, rx: -24, overload: -7, fiber: 'os2-1550', lengthM: 200, connectors: 2 }).warnings).toContain('overload')
    expect(calculate({ ...base, lengthM: 1000, connectors: 2, splices: 0 }).verdict).toBe('ok')
  })
})
