/** Codec presets: packet interval and size on the wire (RTP header included, IP/UDP not). */
export const CODECS = {
  g711: { label: 'G.711 (64 kbit/s)', intervalMs: 20, bytes: 172 },
  g729: { label: 'G.729 (8 kbit/s)', intervalMs: 20, bytes: 32 },
  opus: { label: 'Opus (≈32 kbit/s)', intervalMs: 20, bytes: 92 },
  g722: { label: 'G.722 (64 kbit/s, HD)', intervalMs: 20, bytes: 172 },
} as const
export type Codec = keyof typeof CODECS
export const DURATIONS = [5, 10, 20] as const
export const BUFFERS = [20, 40, 60, 100] as const
export const MIN_BYTES = 12

/** Packet: uint32 sequence, float64 send time (ms), zero padding. */
export function packet(seq: number, sentAt: number, bytes: number): ArrayBuffer {
  const buf = new ArrayBuffer(Math.max(MIN_BYTES, bytes))
  const v = new DataView(buf)
  v.setUint32(0, seq, true)
  v.setFloat64(4, sentAt, true)
  return buf
}

export function readPacket(buf: ArrayBuffer): { seq: number, sentAt: number } | undefined {
  if (buf.byteLength < MIN_BYTES)
    return undefined
  const v = new DataView(buf)
  return { seq: v.getUint32(0, true), sentAt: v.getFloat64(4, true) }
}

export interface Stats {
  sent: number
  received: number
  /** Not echoed before the end of the test. */
  lost: number
  /** Arrived later than the jitter buffer allows: a real phone would drop them. */
  late: number
  /** (lost + late) / sent, percent. */
  lossPct: number
  minRtt: number
  avgRtt: number
  maxRtt: number
  p95Rtt: number
  /** RFC 3550 interarrival jitter over the round-trip times, ms. */
  jitter: number
  /** ITU-T G.107 (simplified) R-factor and MOS. */
  r: number
  mos: number
  grade: 'excellent' | 'good' | 'fair' | 'poor' | 'bad'
}

const round = (n: number, d = 1) => Math.round(n * 10 ** d) / 10 ** d

/** R-factor and MOS from one-way latency, jitter and loss (the common simplified E-model). */
export function eModel(oneWayMs: number, jitterMs: number, lossPct: number): { r: number, mos: number } {
  const eff = oneWayMs + 2 * jitterMs + 10
  let r = eff < 160 ? 93.2 - eff / 40 : 93.2 - (eff - 120) / 10
  r = Math.min(100, Math.max(0, r - 2.5 * lossPct))
  const mos = r <= 0 ? 1 : Math.min(4.5, 1 + 0.035 * r + 0.000007 * r * (r - 60) * (100 - r))
  return { r: round(r), mos: round(Math.max(1, mos), 2) }
}

export function grade(mos: number): Stats['grade'] {
  return mos >= 4.3 ? 'excellent' : mos >= 4.0 ? 'good' : mos >= 3.6 ? 'fair' : mos >= 3.1 ? 'poor' : 'bad'
}

/** rtts[i] is the round trip of packet i in ms, or null when it never came back. */
export function stats(rtts: (number | null)[], bufferMs: number): Stats | undefined {
  const got = rtts.filter((v): v is number => v !== null)
  if (!got.length)
    return undefined
  const sorted = [...got].sort((a, b) => a - b)
  const min = sorted[0]!
  let jitter = 0
  let prev: number | undefined
  for (const v of rtts) {
    if (v === null)
      continue
    if (prev !== undefined)
      jitter += (Math.abs(v - prev) - jitter) / 16
    prev = v
  }
  // Delay variation is measured on the round trip; a receiver sees about half of it per direction.
  const late = got.filter(v => (v - min) / 2 > bufferMs).length
  const lost = rtts.length - got.length
  const lossPct = (lost + late) / rtts.length * 100
  const avg = got.reduce((a, b) => a + b, 0) / got.length
  const { r, mos } = eModel(avg / 2, jitter / 2, lossPct)
  return {
    sent: rtts.length,
    received: got.length,
    lost,
    late,
    lossPct: round(lossPct, 2),
    minRtt: round(min),
    avgRtt: round(avg),
    maxRtt: round(sorted.at(-1)!),
    p95Rtt: round(sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * 0.95))]!),
    jitter: round(jitter),
    r,
    mos,
    grade: grade(mos),
  }
}
