import { describe, expect, it } from 'vitest'
import { compare, computeDs, dsText, keyRole, nameWire, parseDnskeys, parseDs } from './ds-dnskey.service'

// RFC 4034 §5.4 example.
const RFC_KEY = `dskey.example.com. 86400 IN DNSKEY 256 3 5 ( AQOeiiR0GOMYkDshWoSKz9Xz
                                          fwJr1AYtsmx3TGkJaNXVbfi/
                                          2pHm822aJ5iI9BMzNXxeYCmZ
                                          DRD99WYwYqUSdjMmmAphXdvx
                                          egXd/M5+X7OrzKBaMbCVdFLU
                                          Uh6DhweJBjEVv5f2wwjM9Xzc
                                          nOf+EPbtG9DMBmADjFDc2w/r
                                          ljwvFw==
                                          ) ;  key id = 60485`
const RFC_DS_SHA1 = '2BB183AF5F22588179A53B0A98631FAD1A292118'

describe('ds-dnskey', () => {
  it('parses a multi-line DNSKEY and computes the RFC 4034 key tag and DS', async () => {
    const keys = parseDnskeys(RFC_KEY)!
    expect(keys).toHaveLength(1)
    expect(keys[0]!.owner).toBe('dskey.example.com.')
    expect(keys[0]!.key.keyTag).toBe(60485)
    const ds = await computeDs('dskey.example.com', keys[0]!.key, 1)
    expect(dsText(ds)).toBe(`60485 5 1 ${RFC_DS_SHA1}`)
  })

  it('parses bare RDATA and DS lines', () => {
    expect(parseDnskeys('257 3 13 mdsswUyr3DPW132mOi8V9xESWE8jTo0dxCjjnopKl+GqJxpVXckHAeF+KkxLbxILfDLUT0rAK9iUzy1L53eKGQ==')![0]!.key.flags).toBe(257)
    expect(parseDnskeys('not a key')).toBeUndefined()
    expect(parseDs('example.com. IN DS 2371 13 2 32996839a6d808afe3eb4a795a0e6a7a39a76fc52ff228b22b76f6d63826f2b9')).toEqual([
      { keyTag: 2371, algorithm: 13, digestType: 2, digest: '32996839A6D808AFE3EB4A795A0E6A7A39A76FC52FF228B22B76F6D63826F2B9' },
    ])
  })

  it('encodes owner names canonically', () => {
    expect([...nameWire('Ab.C.')]).toEqual([2, 97, 98, 1, 99, 0])
    expect([...nameWire('.')]).toEqual([0])
  })

  it('matches DS records to keys', async () => {
    const key = parseDnskeys(RFC_KEY.replace('DNSKEY 256', 'DNSKEY 257'))![0]!.key
    expect(keyRole(key)).toBe('ksk')
    const ds = await computeDs('dskey.example.com', key, 2)
    const stale = { keyTag: 11111, algorithm: 5, digestType: 2, digest: 'AB'.repeat(32) }
    const r = await compare('dskey.example.com', [key], [ds, stale])
    expect(r.ds.map(d => d.match)).toEqual([true, false])
    expect(r.ds[1]!.key).toBeUndefined()
    expect(r.unlinked).toEqual([])
    expect((await compare('dskey.example.com', [key], [stale])).unlinked).toEqual([key])
  })
})
