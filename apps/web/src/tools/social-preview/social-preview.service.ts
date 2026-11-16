import type { components } from '@/api/schema'

export type HttpSocial = components['schemas']['HttpSocial']

export interface Card { title: string, description: string, site: string, image: string, large: boolean }

/** What a link preview will show, with the usual fallbacks (og → twitter → HTML). */
export function card(r: HttpSocial): Card {
  let site = r.og.site_name ?? ''
  if (!site) {
    try {
      site = new URL(r.url).hostname.replace(/^www\./, '')
    }
    catch {}
  }
  return {
    title: r.og.title || r.twitter.title || r.title,
    description: r.og.description || r.twitter.description || r.description,
    site,
    image: r.image?.dataUri ?? '',
    large: (r.twitter.card ?? 'summary_large_image') !== 'summary',
  }
}

/** URLs are often entered without a scheme. */
export function normalizeUrl(s: string): string {
  const v = s.trim()
  return /^https?:\/\//i.test(v) ? v : `https://${v}`
}
