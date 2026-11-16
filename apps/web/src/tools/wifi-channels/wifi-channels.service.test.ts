import { describe, expect, it } from 'vitest'
import { channels, nonOverlapping, plan } from './wifi-channels.service'

const nums = (c: { number: number }[]) => c.map(x => x.number)

describe('wifi-channels', () => {
  it('lists 2.4 GHz channels per region', () => {
    expect(channels('2.4', 'eu', 20)).toHaveLength(13)
    expect(channels('2.4', 'us', 20)).toHaveLength(11)
    expect(channels('2.4', 'eu', 20)[0]!.centerMhz).toBe(2412)
    expect(nums(nonOverlapping('2.4', 'eu', 40, false))).toEqual([1, 6, 11])
  })

  it('bonds 5 GHz channels in the standard groups', () => {
    expect(nums(channels('5', 'eu', 80))).toEqual([36, 52, 100, 116])
    expect(nums(channels('5', 'us', 80))).toEqual([36, 52, 100, 116, 132, 149])
    expect(nums(channels('5', 'eu', 160))).toEqual([36, 100])
    expect(nums(channels('5', 'eu', 40))).toEqual([36, 44, 52, 60, 100, 108, 116, 124, 132])
    const c36 = channels('5', 'eu', 80)[0]!
    expect([c36.centerMhz, c36.dfs]).toEqual([5210, false])
    expect(channels('5', 'eu', 80)[1]!.dfs).toBe(true)
    expect(channels('5', 'eu', 80).find(c => c.number === 116)!.weatherRadar).toBe(true)
    expect(nums(nonOverlapping('5', 'eu', 80, true))).toEqual([36])
  })

  it('lists 6 GHz channels', () => {
    expect(channels('6', 'eu', 20)).toHaveLength(24)
    expect(nums(channels('6', 'eu', 160))).toEqual([1, 33, 65])
    expect(nums(channels('6', 'us', 320))).toEqual([1, 65, 129])
    expect(channels('6', 'eu', 20)[0]!.centerMhz).toBe(5955)
  })

  it('plans access points with reuse', () => {
    const p = plan('2.4', 'eu', 20, 4, false)
    expect(p.map(x => x.channel.number)).toEqual([1, 6, 11, 1])
    expect(p[3]!.reuse).toBe(1)
  })
})
