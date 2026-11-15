// @vitest-environment jsdom
import type { components } from '@/api/schema'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import { parse } from 'yaml'
import DnsFilterAlert from './DnsFilterAlert.vue'

type DnsLookup = components['schemas']['DnsLookup']
const en = parse(readFileSync(join(import.meta.dirname, '../../locales/en.yml'), 'utf8')) as Record<string, unknown>
const base: DnsLookup = { name: 'ads.example', type: 'A', resolver: 'System (127.0.0.11)', transport: 'udp', rcode: 'NOERROR', flags: { aa: false, tc: false, rd: true, ra: true, ad: false, cd: false }, answer: [], authority: [], additional: [], rttMs: 1 }
const filtered: DnsLookup = { ...base, filtered: { reason: 'null-address', ede: 0, edeText: '', reference: 'Cloudflare (1.1.1.1)', referenceRcode: 'NOERROR', referenceData: ['192.0.2.7'] } }

function render(results: (DnsLookup | undefined)[]) {
  const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } } as Parameters<typeof createI18n>[0])
  return mount(DnsFilterAlert, { props: { results }, global: { plugins: [i18n] } })
}

describe('dnsFilterAlert', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders nothing for normal answers', () => {
    expect(render([base, undefined]).find('[data-testid="dns-filtered"]').exists()).toBe(false)
  })

  it('explains one blocked answer with the unfiltered data', () => {
    const text = render([filtered]).text()
    expect(text).toContain('0.0.0.0 or ::')
    expect(text).toContain('Cloudflare (1.1.1.1) answers: 192.0.2.7')
    expect(text).toContain('DNS_RESOLVER')
  })

  it('summarizes blocked answers in a report', () => {
    const text = render([base, filtered, { ...filtered, type: 'MX' }]).text()
    expect(text).toContain('2 answer(s)')
    expect(text).toContain('ads.example A, ads.example MX')
  })
})
