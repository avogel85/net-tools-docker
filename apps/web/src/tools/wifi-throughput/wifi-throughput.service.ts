export type Standard = 'n' | 'ac' | 'ax' | 'be'

export interface StandardInfo {
  id: Standard
  name: string
  widths: number[]
  maxMcs: number
  maxStreams: number
  /** Guard intervals in µs. */
  gis: number[]
  /** OFDM symbol duration without GI, µs. */
  symbol: number
  /** Data subcarriers per channel width. */
  subcarriers: Record<number, number>
  /** Typical single-client TCP goodput as a share of the PHY rate. */
  efficiency: number
}

export const STANDARDS: StandardInfo[] = [
  { id: 'n', name: 'Wi-Fi 4 (802.11n)', widths: [20, 40], maxMcs: 7, maxStreams: 4, gis: [0.8, 0.4], symbol: 3.2, subcarriers: { 20: 52, 40: 108 }, efficiency: 0.6 },
  { id: 'ac', name: 'Wi-Fi 5 (802.11ac)', widths: [20, 40, 80, 160], maxMcs: 9, maxStreams: 8, gis: [0.8, 0.4], symbol: 3.2, subcarriers: { 20: 52, 40: 108, 80: 234, 160: 468 }, efficiency: 0.62 },
  { id: 'ax', name: 'Wi-Fi 6/6E (802.11ax)', widths: [20, 40, 80, 160], maxMcs: 11, maxStreams: 8, gis: [0.8, 1.6, 3.2], symbol: 12.8, subcarriers: { 20: 234, 40: 468, 80: 980, 160: 1960 }, efficiency: 0.7 },
  { id: 'be', name: 'Wi-Fi 7 (802.11be)', widths: [20, 40, 80, 160, 320], maxMcs: 13, maxStreams: 16, gis: [0.8, 1.6, 3.2], symbol: 12.8, subcarriers: { 20: 234, 40: 468, 80: 980, 160: 1960, 320: 3920 }, efficiency: 0.7 },
]

/** Modulation per MCS: bits per subcarrier and coding rate. */
export const MCS: { mod: string, bits: number, rate: [number, number] }[] = [
  { mod: 'BPSK', bits: 1, rate: [1, 2] },
  { mod: 'QPSK', bits: 2, rate: [1, 2] },
  { mod: 'QPSK', bits: 2, rate: [3, 4] },
  { mod: '16-QAM', bits: 4, rate: [1, 2] },
  { mod: '16-QAM', bits: 4, rate: [3, 4] },
  { mod: '64-QAM', bits: 6, rate: [2, 3] },
  { mod: '64-QAM', bits: 6, rate: [3, 4] },
  { mod: '64-QAM', bits: 6, rate: [5, 6] },
  { mod: '256-QAM', bits: 8, rate: [3, 4] },
  { mod: '256-QAM', bits: 8, rate: [5, 6] },
  { mod: '1024-QAM', bits: 10, rate: [3, 4] },
  { mod: '1024-QAM', bits: 10, rate: [5, 6] },
  { mod: '4096-QAM', bits: 12, rate: [3, 4] },
  { mod: '4096-QAM', bits: 12, rate: [5, 6] },
]

/** PHY rate in Mbit/s, or undefined for combinations the standard does not define. */
export function phyRate(std: StandardInfo, width: number, mcs: number, streams: number, gi: number): number | undefined {
  const sc = std.subcarriers[width]
  const m = MCS[mcs]
  if (!sc || !m || mcs > std.maxMcs || streams < 1 || streams > std.maxStreams || !std.gis.includes(gi))
    return undefined
  // 802.11ac: MCS 9 at 20 MHz exists only for 3 and 6 streams (non-integer bits per symbol otherwise).
  if (std.id === 'ac' && width === 20 && mcs === 9 && streams !== 3 && streams !== 6)
    return undefined
  const bitsPerSymbol = sc * m.bits * m.rate[0] / m.rate[1] * streams
  return Math.round(bitsPerSymbol / (std.symbol + gi) * 10) / 10
}

export function table(std: StandardInfo, width: number, streams: number, gi: number): { mcs: number, mod: string, coding: string, rate: number | undefined }[] {
  return MCS.slice(0, std.maxMcs + 1).map((m, i) => ({ mcs: i, mod: m.mod, coding: `${m.rate[0]}/${m.rate[1]}`, rate: phyRate(std, width, i, streams, gi) }))
}
