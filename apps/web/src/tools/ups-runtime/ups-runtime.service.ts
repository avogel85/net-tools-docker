export interface RuntimeInput {
  /** Battery energy in Wh, or 0 to derive it from volts × amp-hours. */
  wh: number
  volts: number
  ah: number
  /** Combined battery-to-AC efficiency, percent. */
  efficiency: number
  /** Usable depth of discharge, percent. */
  depth: number
  /** Load in watts, or 0 to derive it from VA × power factor. */
  watts: number
  va: number
  powerFactor: number
}

export interface RuntimeResult { usableWh: number, loadW: number, minutes: number }

export function runtime(i: RuntimeInput): RuntimeResult | undefined {
  const wh = i.wh > 0 ? i.wh : i.volts * i.ah
  const loadW = i.watts > 0 ? i.watts : i.va * i.powerFactor
  if (!(wh > 0) || !(loadW > 0) || i.efficiency <= 0 || i.depth <= 0)
    return undefined
  const usableWh = wh * (i.depth / 100) * (i.efficiency / 100)
  return { usableWh, loadW, minutes: (usableWh / loadW) * 60 }
}

export function formatDuration(minutes: number): string {
  const total = Math.round(minutes)
  const h = Math.floor(total / 60)
  const m = total % 60
  return h ? `${h} h ${m} min` : `${m} min`
}
