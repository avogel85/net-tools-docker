import { describe, expect, it } from 'vitest'
import { filterLinks, statusText } from './broken-links.service'

const l = (status: number, broken: boolean, kind = 'page', error = '', redirects = 0) => ({ url: 'u', kind, status, redirects, final: '', error, broken, from: '', refs: 1 }) as never

describe('broken-links', () => {
  it('formats status', () => {
    expect(statusText(l(200, false, 'page', '', 2))).toBe('200 (2 redirects)')
    expect(statusText(l(0, true, 'page', 'timeout'))).toBe('timeout')
    expect(statusText(l(404, true))).toBe('404')
  })
  it('filters', () => {
    expect(filterLinks([l(200, false), l(404, true), l(500, true, 'asset')], true, 'asset')).toHaveLength(1)
  })
})
