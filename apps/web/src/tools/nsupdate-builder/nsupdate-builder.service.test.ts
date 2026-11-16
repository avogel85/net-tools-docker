import { describe, expect, it } from 'vitest'
import { absolute, buildScript, generateSecret, isValidSecret, keyConfigs, parseOperations } from './nsupdate-builder.service'

describe('nsupdate-builder', () => {
  it('generates secrets of the HMAC size', () => {
    expect(atob(generateSecret('hmac-sha256')).length).toBe(32)
    expect(atob(generateSecret('hmac-sha512')).length).toBe(64)
    expect(isValidSecret(generateSecret('hmac-sha1'))).toBe(true)
    expect(isValidSecret('not base64!')).toBe(false)
    expect(isValidSecret('QUJD')).toBe(false) // 3 bytes: too short
  })

  it('renders key configs', () => {
    const c = keyConfigs({ name: 'ddns-key.', algorithm: 'hmac-sha256', secret: 'c2VjcmV0c2VjcmV0' })
    expect(c.bind).toBe('key "ddns-key" {\n    algorithm hmac-sha256;\n    secret "c2VjcmV0c2VjcmV0";\n};\n')
    expect(c.knot).toContain('  - id: ddns-key\n    algorithm: hmac-sha256\n')
    expect(c.powerdns).toBe('pdnsutil import-tsig-key ddns-key hmac-sha256 c2VjcmV0c2VjcmV0\n')
    expect(c.cli).toBe('nsupdate -y hmac-sha256:ddns-key:c2VjcmV0c2VjcmV0\n')
  })

  it('parses operations with errors per line', () => {
    const r = parseOperations('# comment\nadd www 300 A 192.0.2.1\nreplace @ 3600 TXT "v=spf1 -all"\ndelete old\ndelete mail MX 10 mx.example.com.\nadd bad\nfrob x\nadd x -1 A 1.2.3.4\nadd x 60 a\n')
    expect(r.map(p => p.error ?? p.op?.action)).toEqual(['add', 'replace', 'delete', 'delete', 'ttl', 'action', 'ttl', 'data'])
    expect(r[1]!.op).toEqual({ action: 'replace', name: '@', ttl: 3600, type: 'TXT', data: '"v=spf1 -all"' })
    expect(r[3]!.op).toEqual({ action: 'delete', name: 'mail', type: 'MX', data: '10 mx.example.com.' })
    expect(r[0]!.line).toBe(2)
  })

  it('builds a script', () => {
    const ops = parseOperations('add www 300 A 192.0.2.1\nreplace @ 60 TXT "x"\ndelete old.example.com.').map(p => p.op!)
    expect(buildScript({ server: 'ns1.example.com', port: 5353, zone: 'example.com', ops, showAnswer: true })).toBe([
      'server ns1.example.com 5353',
      'zone example.com.',
      'update add www.example.com. 300 A 192.0.2.1',
      'update delete example.com. TXT',
      'update add example.com. 60 TXT "x"',
      'update delete old.example.com.',
      'show',
      'send',
      'answer',
      '',
    ].join('\n'))
    expect(absolute('www', '')).toBe('www.')
  })
})
