import { describe, expect, it } from 'vitest'
import { asPathOf, locationList } from './reverse-traceroute.service'

describe('reverse-traceroute', () => {
  it('extracts the AS path without repeats', () => {
    const result = { hops: [{ asn: [3320] }, { asn: [3320] }, { asn: [] }, { asn: [1299] }, { asn: [33915] }] } as never
    expect(asPathOf({ result })).toEqual([3320, 1299, 33915])
  })
  it('splits locations', () => {
    expect(locationList('Germany, US+AWS,, Japan')).toEqual(['Germany', 'US+AWS', 'Japan'])
  })
})
