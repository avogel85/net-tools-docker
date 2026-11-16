import { describe, expect, it } from 'vitest'
import { curlExample, loadBin, prettyBody, saveBin } from './webhook-inspector.service'

describe('webhook-inspector', () => {
  it('pretty-prints bodies', () => {
    expect(prettyBody({ body: '{"a":1}', contentType: 'application/json' })).toBe('{\n  "a": 1\n}')
    expect(prettyBody({ body: 'a=1&b=two', contentType: 'application/x-www-form-urlencoded' })).toBe('a = 1\nb = two')
    expect(prettyBody({ body: '{broken', contentType: 'application/json' })).toBe('{broken')
  })
  it('keeps the bin in session storage until it expires', () => {
    const mem = new Map<string, string>()
    const st = { getItem: (k: string) => mem.get(k) ?? null, setItem: (k: string, v: string) => void mem.set(k, v), removeItem: (k: string) => void mem.delete(k) }
    saveBin({ id: 'a', token: 't', path: '/api/v1/hook/a', expires: new Date(Date.now() + 60000).toISOString() }, st)
    expect(loadBin(st)?.id).toBe('a')
    saveBin({ id: 'b', token: 't', path: '/x', expires: new Date(Date.now() - 1).toISOString() }, st)
    expect(loadBin(st)).toBeUndefined()
    saveBin(undefined, st)
    expect(mem.size).toBe(0)
  })
  it('builds a curl example', () => {
    expect(curlExample('https://n.test/api/v1/hook/x')).toContain('curl -X POST \'https://n.test/api/v1/hook/x\'')
  })
})
