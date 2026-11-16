import type { HttpSocial } from './social-preview.service'
import { describe, expect, it } from 'vitest'
import { card, normalizeUrl } from './social-preview.service'

const base: HttpSocial = { url: 'https://www.example.com/x', status: 200, title: 'HTML title', description: 'meta', canonical: '', themeColor: '', manifest: '', og: {}, twitter: {}, icons: [], problems: [], warnings: [] }

describe('social-preview', () => {
  it('falls back from og to twitter to HTML', () => {
    expect(card(base)).toEqual({ title: 'HTML title', description: 'meta', site: 'example.com', image: '', large: true })
    expect(card({ ...base, og: { title: 'OG', site_name: 'Shop' }, twitter: { card: 'summary', description: 'tw' } })).toMatchObject({ title: 'OG', description: 'tw', site: 'Shop', large: false })
  })
  it('adds a scheme', () => {
    expect(normalizeUrl('example.com')).toBe('https://example.com')
    expect(normalizeUrl('http://x.test')).toBe('http://x.test')
  })
})
