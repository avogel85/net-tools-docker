import { describe, expect, it } from 'vitest'
import { groupDetections } from './email-provider-detector.service'

describe('email-provider-detector', () => {
  it('groups by role in a fixed order and merges evidence', () => {
    const g = groupDetections([
      { provider: 'SendGrid', role: 'sending', evidence: 'SPF include:sendgrid.net' },
      { provider: 'Microsoft 365', role: 'inbound', evidence: 'MX a' },
      { provider: 'SendGrid', role: 'sending', evidence: 'DKIM s1._domainkey' },
    ])
    expect(g.map(x => x.role)).toEqual(['inbound', 'sending'])
    expect(g[1]!.providers).toEqual([{ provider: 'SendGrid', evidence: ['SPF include:sendgrid.net', 'DKIM s1._domainkey'] }])
  })
})
