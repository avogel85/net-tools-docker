import { describe, expect, it } from 'vitest'
import { counts, sortChecks } from './takeover-check.service'

const c = (name: string, status: never) => ({ name, status, chain: [], service: '', evidence: '' })

describe('takeover-check', () => {
  it('sorts by severity and hides missing names', () => {
    const list = [c('b', 'ok' as never), c('a', 'nxdomain' as never), c('z', 'vulnerable' as never), c('y', 'dangling' as never)]
    expect(sortChecks(list, false).map(x => x.name)).toEqual(['z', 'y', 'b'])
    expect(sortChecks(list, true)).toHaveLength(4)
    expect(counts(list)).toEqual({ ok: 1, nxdomain: 1, vulnerable: 1, dangling: 1 })
  })
})
