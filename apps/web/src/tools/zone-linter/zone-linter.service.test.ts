import { describe, expect, it } from 'vitest'
import { lintZone, parseTtl, parseZone } from './zone-linter.service'

const GOOD = `$ORIGIN example.com.
$TTL 1h
@   IN SOA ns1 hostmaster (
        2026111601 ; serial
        3600 900 1209600 300 )
    IN NS  ns1
    IN NS  ns2.example.net.
    IN MX  10 mail
    IN TXT "v=spf1 mx -all"
ns1     A    192.0.2.1
mail    A    192.0.2.2
        AAAA 2001:db8::2
www  300 CNAME @
sub     NS   ns.sub
ns.sub  A    192.0.2.3
`

const codes = (text: string, origin = '') => lintZone(text, origin).findings.map(f => f.code)

describe('zone-linter', () => {
  it('parses TTL units', () => {
    expect(parseTtl('3600')).toBe(3600)
    expect(parseTtl('1h30m')).toBe(5400)
    expect(parseTtl('1w2d')).toBe(777600)
    expect(parseTtl('IN')).toBeUndefined()
  })

  it('parses names, multi-line records and inherited owners', () => {
    const r = parseZone(GOOD, '')
    expect(r.records.map(x => `${x.name} ${x.type}`)).toEqual([
      'example.com. SOA',
      'example.com. NS',
      'example.com. NS',
      'example.com. MX',
      'example.com. TXT',
      'ns1.example.com. A',
      'mail.example.com. A',
      'mail.example.com. AAAA',
      'www.example.com. CNAME',
      'sub.example.com. NS',
      'ns.sub.example.com. A',
    ])
    expect(r.records[0]!.data).toBe('ns1 hostmaster 2026111601 3600 900 1209600 300')
    expect(r.records.find(x => x.type === 'CNAME')!.ttl).toBe(300)
    expect(r.records[1]!.ttl).toBe(3600)
  })

  it('finds nothing wrong in a clean zone', () => {
    expect(lintZone(GOOD, '').findings).toEqual([])
  })

  it('flags CNAME conflicts and CNAME at the apex', () => {
    expect(codes(`${GOOD}www A 192.0.2.9\n`)).toContain('cname-other-data')
    expect(codes(`${GOOD}@ CNAME other.example.net.\n`)).toContain('cname-apex')
  })

  it('flags missing glue, IP targets, CNAME targets and missing dots', () => {
    expect(codes(GOOD.replace('ns1     A    192.0.2.1\n', ''))).toContain('missing-glue')
    expect(codes(`${GOOD}@ MX 20 192.0.2.5\n`)).toContain('target-is-ip')
    expect(codes(`${GOOD}@ MX 30 www\n`)).toContain('target-cname')
    expect(codes(`${GOOD}alias CNAME mail.example.com\n`)).toContain('missing-dot')
  })

  it('checks SOA and NS presence and sanity', () => {
    expect(codes('$ORIGIN example.com.\n$TTL 1h\nwww A 192.0.2.1\n')).toEqual(expect.arrayContaining(['no-soa', 'no-ns']))
    expect(codes(GOOD.replace('3600 900 1209600 300', '900 3600 86400 172800'))).toEqual(expect.arrayContaining(['soa-retry', 'soa-expire', 'soa-minimum', 'soa-refresh']))
    expect(codes(GOOD.replace('    IN NS  ns2.example.net.\n', ''))).toContain('one-ns')
  })

  it('flags bad data, duplicates, TTL mismatches and occluded names', () => {
    const extra = 'bad A 999.1.1.1\nmail A 192.0.2.2\nmail 60 A 192.0.2.7\nx.sub TXT "hidden"\nother.test. A 192.0.2.8\n@ TXT "v=spf1 -all"\n'
    expect(codes(GOOD + extra)).toEqual(expect.arrayContaining(['bad-address', 'duplicate', 'ttl-mismatch', 'occluded', 'out-of-zone', 'multiple-spf']))
  })

  it('reports parse problems', () => {
    expect(codes('www A 192.0.2.1\n')).toEqual(['no-origin'])
    expect(codes('$ORIGIN example.com.\nwww 300 IN FOO bar\nmail MX\n')).toEqual(expect.arrayContaining(['unknown-type', 'no-data']))
    expect(codes(`$ORIGIN example.com.\n@ TXT "${'a'.repeat(300)}"\n`)).toContain('txt-too-long')
    expect(codes('www A 192.0.2.1\n', 'example.com')).toContain('no-default-ttl')
  })
})
