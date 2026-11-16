import { describe, expect, it } from 'vitest'
import { decodePri, keyValues, parseLine, parseLog, summary } from './log-parser.service'

const field = (fields: [string, string][], k: string) => fields.find(([n]) => n === k)?.[1]

describe('log-parser', () => {
  it('decodes PRI', () => {
    expect(decodePri(34)).toEqual([['facility', 'auth'], ['severity', '2 crit']])
    expect(decodePri(190)).toEqual([['facility', 'local7'], ['severity', '6 info']])
  })

  it('parses RFC 5424', () => {
    const p = parseLine('<165>1 2003-10-11T22:14:15.003Z mymachine.example.com evntslog - ID47 [exampleSDID@32473 iut="3" eventSource="Application"] An application event')
    expect(p.format).toBe('syslog5424')
    expect(field(p.fields, 'host')).toBe('mymachine.example.com')
    expect(field(p.fields, 'msgid')).toBe('ID47')
    expect(field(p.fields, 'message')).toBe('An application event')
    expect(field(p.fields, 'severity')).toBe('5 notice')
  })

  it('parses RFC 3164 with and without PRI', () => {
    const a = parseLine('<34>Oct 11 22:14:15 mymachine su: \'su root\' failed for lonvick on /dev/pts/8')
    expect(a.format).toBe('syslog3164')
    expect(field(a.fields, 'program')).toBe('su')
    const b = parseLine('Nov 17 10:01:02 gw sshd[1234]: Failed password for root from 203.0.113.5 port 52211 ssh2')
    expect(field(b.fields, 'pid')).toBe('1234')
    expect(field(b.fields, 'message')).toContain('Failed password')
  })

  it('parses Cisco ASA', () => {
    const p = parseLine('<166>Nov 17 2026 10:00:00: %ASA-6-302013: Built inbound TCP connection 4711 for outside:203.0.113.5/52211 (203.0.113.5/52211) to inside:10.0.0.5/443 (198.51.100.10/443)')
    expect(p.format).toBe('cisco-asa')
    expect(field(p.fields, 'message-id')).toBe('302013')
    expect(field(p.fields, 'severity')).toBe('6 info')
    expect(field(p.fields, 'endpoint 1')).toBe('outside 203.0.113.5 port 52211')
    expect(field(p.fields, 'endpoint 2')).toBe('inside 10.0.0.5 port 443')
  })

  it('parses key=value (FortiGate, iptables)', () => {
    const fg = parseLine('date=2026-11-17 time=10:00:00 devname="FG-60F" type="traffic" subtype="forward" srcip=10.0.0.5 dstip=203.0.113.9 dstport=443 action="deny" msg="policy \\"x\\""')
    expect(fg.format).toBe('keyvalue')
    expect(field(fg.fields, 'devname')).toBe('FG-60F')
    expect(field(fg.fields, 'msg')).toBe('policy "x"')
    const ipt = parseLine('Nov 17 10:00:00 fw kernel: [UFW BLOCK] IN=eth0 OUT= MAC=aa SRC=203.0.113.5 DST=10.0.0.1 PROTO=TCP SPT=40000 DPT=22')
    expect(ipt.format).toBe('keyvalue')
    expect(field(ipt.fields, 'host')).toBe('fw')
    expect(field(ipt.fields, 'DPT')).toBe('22')
    expect(keyValues('a=1 b="x y"')).toEqual([['a', '1'], ['b', 'x y']])
  })

  it('parses access logs and JSON', () => {
    const a = parseLine('203.0.113.5 - - [17/Nov/2026:10:00:00 +0100] "GET /index.html HTTP/1.1" 200 1234 "-" "curl/8.0"')
    expect(a.format).toBe('access-log')
    expect(field(a.fields, 'status')).toBe('200')
    expect(field(a.fields, 'user-agent')).toBe('curl/8.0')
    const j = parseLine('{"level":"error","msg":"boom","ctx":{"user":"x","n":2}}')
    expect(j.format).toBe('json')
    expect(field(j.fields, 'ctx.n')).toBe('2')
  })

  it('handles unknown lines and summaries', () => {
    expect(parseLine('just some text').format).toBe('unknown')
    const lines = parseLog('\nNov 17 10:01:02 gw sshd[1234]: hello\n\n')
    expect(lines).toHaveLength(1)
    expect(lines[0]!.line).toBe(2)
    expect(summary(lines[0]!)).toEqual({ time: 'Nov 17 10:01:02', source: 'gw', severity: '', text: 'hello' })
  })
})
