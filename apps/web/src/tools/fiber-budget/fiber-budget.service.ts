export interface FiberType { id: string, mode: 'sm' | 'mm', wavelength: number, dbPerKm: number }

/** Maximum cabled attenuation per ANSI/TIA-568 and IEC 60793 (typical planning values). */
export const FIBERS: FiberType[] = [
  { id: 'os2-1310', mode: 'sm', wavelength: 1310, dbPerKm: 0.4 },
  { id: 'os2-1550', mode: 'sm', wavelength: 1550, dbPerKm: 0.3 },
  { id: 'om3-850', mode: 'mm', wavelength: 850, dbPerKm: 3.0 },
  { id: 'om4-850', mode: 'mm', wavelength: 850, dbPerKm: 3.0 },
  { id: 'om5-850', mode: 'mm', wavelength: 850, dbPerKm: 3.0 },
  { id: 'om2-850', mode: 'mm', wavelength: 850, dbPerKm: 3.5 },
]

export interface Optic {
  id: string
  /** Minimum average launch power, dBm. */
  tx: number
  /** Receiver sensitivity, dBm. */
  rx: number
  /** Receiver overload, dBm. */
  overload: number
  mode: 'sm' | 'mm'
  /** Standard reach per fiber id, metres. */
  reach: Record<string, number>
}

/** Typical datasheet values; vendors differ, so check yours. */
export const OPTICS: Optic[] = [
  { id: '1000base-sx', tx: -9.5, rx: -17, overload: 0, mode: 'mm', reach: { 'om2-850': 550, 'om3-850': 550, 'om4-850': 550, 'om5-850': 550 } },
  { id: '1000base-lx', tx: -9.5, rx: -19, overload: -3, mode: 'sm', reach: { 'os2-1310': 10000 } },
  { id: '10gbase-sr', tx: -7.3, rx: -9.9, overload: -1, mode: 'mm', reach: { 'om2-850': 82, 'om3-850': 300, 'om4-850': 400, 'om5-850': 400 } },
  { id: '10gbase-lr', tx: -8.2, rx: -14.4, overload: 0.5, mode: 'sm', reach: { 'os2-1310': 10000 } },
  { id: '10gbase-er', tx: -4.7, rx: -15.8, overload: -1, mode: 'sm', reach: { 'os2-1550': 40000 } },
  { id: '10gbase-zr', tx: 0, rx: -24, overload: -7, mode: 'sm', reach: { 'os2-1550': 80000 } },
  { id: '25gbase-sr', tx: -8.4, rx: -10.3, overload: 2.4, mode: 'mm', reach: { 'om3-850': 70, 'om4-850': 100, 'om5-850': 100 } },
  { id: '25gbase-lr', tx: -7, rx: -13.3, overload: 2, mode: 'sm', reach: { 'os2-1310': 10000 } },
  { id: '100gbase-lr4', tx: -4.3, rx: -10.6, overload: 4.5, mode: 'sm', reach: { 'os2-1310': 10000 } },
]

export interface BudgetInput {
  fiber: string
  lengthM: number
  connectors: number
  connectorDb: number
  splices: number
  spliceDb: number
  marginDb: number
  tx: number
  rx: number
  overload: number
  optic: string
}

export interface Budget {
  fiberLoss: number
  connectorLoss: number
  spliceLoss: number
  totalLoss: number
  powerBudget: number
  margin: number
  rxPower: number
  verdict: 'ok' | 'tight' | 'fail'
  warnings: string[]
}

const r2 = (n: number) => Math.round(n * 100) / 100

export function calculate(i: BudgetInput): Budget {
  const fiber = FIBERS.find(f => f.id === i.fiber) ?? FIBERS[0]!
  const fiberLoss = r2(i.lengthM / 1000 * fiber.dbPerKm)
  const connectorLoss = r2(i.connectors * i.connectorDb)
  const spliceLoss = r2(i.splices * i.spliceDb)
  const totalLoss = r2(fiberLoss + connectorLoss + spliceLoss)
  const powerBudget = r2(i.tx - i.rx)
  const margin = r2(powerBudget - totalLoss - i.marginDb)
  const rxPower = r2(i.tx - totalLoss)
  const warnings: string[] = []
  const optic = OPTICS.find(o => o.id === i.optic)
  if (optic) {
    if (optic.mode !== fiber.mode)
      warnings.push('mode-mismatch')
    else if (optic.reach[fiber.id] === undefined)
      warnings.push('wavelength')
    else if (i.lengthM > optic.reach[fiber.id]!)
      warnings.push('reach')
  }
  if (rxPower > i.overload)
    warnings.push('overload')
  if (i.connectorDb > 0.75)
    warnings.push('connector-high')
  const verdict = powerBudget - totalLoss < 0 ? 'fail' : margin < 0 ? 'tight' : 'ok'
  return { fiberLoss, connectorLoss, spliceLoss, totalLoss, powerBudget, margin, rxPower, verdict, warnings }
}
