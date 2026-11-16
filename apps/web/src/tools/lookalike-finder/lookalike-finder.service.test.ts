import { describe, expect, it } from 'vitest'
import { risk, visible } from './lookalike-finder.service'

const c = (domain: string, registered: boolean, kind: string, mx: string[] = []) => ({ domain, unicode: '', kind, registered, checked: true, a: [], mx, error: '' }) as never

describe('lookalike-finder', () => {
  it('filters and rates candidates', () => {
    const list = [c('a.com', true, 'tld', ['mx.a.com']), c('b.com', false, 'tld'), c('c.com', true, 'omission')]
    expect(visible(list, true, '').map((x: { domain: string }) => x.domain)).toEqual(['a.com', 'c.com'])
    expect(visible(list, false, 'tld')).toHaveLength(2)
    expect([risk(list[0]!), risk(list[1]!), risk(list[2]!)]).toEqual(['high', 'low', 'medium'])
  })
})
