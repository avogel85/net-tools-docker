import { describe, expect, it } from 'vitest'
import { sortResults } from './udp-probe.service'

describe('udp-probe', () => {
  it('sorts open results first', () => {
    const r = (service: string, state: string) => ({ service, state, port: 1, detail: '', rttMs: 0 }) as never
    expect(sortResults([r('a', 'closed'), r('b', 'open'), r('c', 'open|filtered')]).map((x: { service: string }) => x.service)).toEqual(['b', 'c', 'a'])
  })
})
