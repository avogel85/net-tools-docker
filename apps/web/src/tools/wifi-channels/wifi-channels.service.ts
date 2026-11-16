export type Band = '2.4' | '5' | '6'
export type Region = 'eu' | 'us'

export interface Channel {
  /** Primary (lowest) 20 MHz channel number and the block it belongs to. */
  number: number
  centerMhz: number
  width: number
  /** 20 MHz channels covered. */
  covers: number[]
  dfs: boolean
  /** Weather radar channels in Europe: 10-minute DFS check before use. */
  weatherRadar: boolean
}

const allowed20: Record<Band, Record<Region, number[]>> = {
  2.4: { eu: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13], us: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11] },
  5: {
    eu: [36, 40, 44, 48, 52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124, 128, 132, 136, 140],
    us: [36, 40, 44, 48, 52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124, 128, 132, 136, 140, 144, 149, 153, 157, 161, 165],
  },
  6: {
    eu: Array.from({ length: 24 }, (_, i) => 1 + 4 * i), // 5945–6425 MHz (U-NII-5)
    us: Array.from({ length: 59 }, (_, i) => 1 + 4 * i), // 5925–7125 MHz (U-NII-5..8)
  },
}

export const WIDTHS: Record<Band, number[]> = { 2.4: [20, 40], 5: [20, 40, 80, 160], 6: [20, 40, 80, 160, 320] }

/** First 20 MHz channel of each 5 GHz bonding group. */
const BONDING_5: Record<number, number[]> = {
  40: [36, 44, 52, 60, 100, 108, 116, 124, 132, 140, 149, 157],
  80: [36, 52, 100, 116, 132, 149],
  160: [36, 100],
}

const center = (band: Band, ch: number) => (band === '2.4' ? 2407 + 5 * ch : band === '5' ? 5000 + 5 * ch : 5950 + 5 * ch)

/** Channels of a band, region and width; wider channels are aligned blocks of 20 MHz channels. */
export function channels(band: Band, region: Region, width: number): Channel[] {
  const ok = allowed20[band][region]
  if (band === '2.4') {
    if (width === 20)
      return ok.map(n => ({ number: n, centerMhz: center(band, n), width, covers: [n], dfs: false, weatherRadar: false }))
    // 40 MHz in 2.4 GHz: primary + secondary 4 channels up.
    return ok.filter(n => ok.includes(n + 4)).map(n => ({ number: n, centerMhz: center(band, n) + 10, width, covers: [n, n + 4], dfs: false, weatherRadar: false }))
  }
  const per = width / 20
  const all = allowed20[band][region]
  // 5 GHz uses fixed bonding groups; 6 GHz blocks are aligned from channel 1.
  const starts = band === '5' ? (BONDING_5[width] ?? all) : all.filter(n => ((n - 1) / 4) % per === 0)
  const out: Channel[] = []
  for (const n of starts) {
    const covers = Array.from({ length: per }, (_, i) => n + 4 * i)
    if (!covers.every(c => all.includes(c)))
      continue
    const dfs = band === '5' && covers.some(c => c >= 52 && c <= 144)
    out.push({ number: n, centerMhz: (center(band, covers[0]!) + center(band, covers.at(-1)!)) / 2, width, covers, dfs, weatherRadar: band === '5' && region === 'eu' && covers.some(c => c >= 120 && c <= 128) })
  }
  return out
}

/** Non-overlapping channels for a plan; 2.4 GHz uses the classic 1/6/11 (20 MHz). */
export function nonOverlapping(band: Band, region: Region, width: number, avoidDfs: boolean): Channel[] {
  if (band === '2.4')
    return channels(band, region, 20).filter(c => [1, 6, 11].includes(c.number))
  return channels(band, region, width).filter(c => !avoidDfs || !c.dfs)
}

/** Assigns channels to n access points; reuse starts when the channels run out. */
export function plan(band: Band, region: Region, width: number, aps: number, avoidDfs: boolean): { ap: number, channel: Channel, reuse: number }[] {
  const ch = nonOverlapping(band, region, width, avoidDfs)
  if (!ch.length)
    return []
  return Array.from({ length: Math.max(0, Math.min(aps, 200)) }, (_, i) => ({ ap: i + 1, channel: ch[i % ch.length]!, reuse: Math.floor(i / ch.length) }))
}
