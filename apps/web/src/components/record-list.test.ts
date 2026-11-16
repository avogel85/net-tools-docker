import { describe, expect, it } from 'vitest'
import { recordZoneLine, recordZoneText } from './record-list'

describe('record-list', () => {
  it('quotes TXT values and leaves other types alone', () => {
    expect(recordZoneLine({ name: '_dmarc.example.com', value: 'v=DMARC1; p=none' })).toBe('_dmarc.example.com. 3600 IN TXT "v=DMARC1; p=none"')
    expect(recordZoneLine({ name: 'host.example.com.', type: 'SSHFP', value: '4 2 abcd' }, 300)).toBe('host.example.com. 300 IN SSHFP 4 2 abcd')
  })

  it('joins records', () => {
    expect(recordZoneText([{ name: 'a.test', value: 'x' }, { name: 'b.test', value: 'y' }]).split('\n')).toHaveLength(2)
  })
})
