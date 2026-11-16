import { describe, expect, it } from 'vitest'
import { base32hex, nsec3Hash, paramWarnings, parseNsec3Param, parseSalt } from './nsec3-hash.service'

describe('nsec3-hash', () => {
  // RFC 5155 Appendix A: salt aabbccdd, 12 iterations.
  const salt = parseSalt('aabbccdd')!
  it.each([
    ['example', '0p9mhaveqvm6t7vbl5lop2u3t2rp3tom'],
    ['a.example', '35mthgpgcu1qg68fab165klnsnk3dpvl'],
    ['ns1.example', '2t7b4g4vsa5smi47k61mv5bv1a22bojr'],
    ['*.w.example', 'r53bq7cc2uvmubfu5ocmm6pers9tk9en'],
  ])('hashes %s', async (name, want) => {
    expect(await nsec3Hash(name, salt, 12)).toBe(want)
  })

  it('handles no salt and zero iterations', async () => {
    expect(await nsec3Hash('example.', new Uint8Array(), 0)).toMatch(/^[0-9a-v]{32}$/)
  })

  it('encodes base32hex without padding', () => {
    expect(base32hex(new Uint8Array([0xFF]))).toBe('vs')
    expect(base32hex(new TextEncoder().encode('foobar'))).toBe('cpnmuoj1e8')
  })

  it('parses salt and NSEC3PARAM', () => {
    expect(parseSalt('-')).toEqual(new Uint8Array())
    expect(parseSalt('abc')).toBeUndefined()
    expect(parseNsec3Param('1 0 10 AABBCCDD')).toEqual({ algorithm: 1, flags: 0, iterations: 10, salt: 'AABBCCDD' })
    expect(parseNsec3Param('garbage')).toBeUndefined()
  })

  it('warns about RFC 9276 parameters', () => {
    expect(paramWarnings(0, new Uint8Array())).toEqual([])
    expect(paramWarnings(10, new Uint8Array([1]))).toEqual(['iterations', 'salt'])
    expect(paramWarnings(150, new Uint8Array())).toEqual(['iterations-high'])
  })
})
