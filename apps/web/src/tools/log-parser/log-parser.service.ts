export type Format = 'syslog5424' | 'syslog3164' | 'cisco-asa' | 'keyvalue' | 'access-log' | 'json' | 'unknown'

export interface ParsedLine {
  line: number
  raw: string
  format: Format
  fields: [string, string][]
}

const FACILITIES = ['kern', 'user', 'mail', 'daemon', 'auth', 'syslog', 'lpr', 'news', 'uucp', 'cron', 'authpriv', 'ftp', 'ntp', 'security', 'console', 'solaris-cron', 'local0', 'local1', 'local2', 'local3', 'local4', 'local5', 'local6', 'local7']
export const SEVERITIES = ['emerg', 'alert', 'crit', 'err', 'warning', 'notice', 'info', 'debug']

/** <PRI> → facility and severity (RFC 5424 §6.2.1). */
export function decodePri(pri: number): [string, string][] {
  return [['facility', FACILITIES[pri >> 3] ?? String(pri >> 3)], ['severity', `${pri & 7} ${SEVERITIES[pri & 7]}`]]
}

/** key=value and key="quoted value" pairs (FortiGate, iptables, many appliances). */
export function keyValues(s: string): [string, string][] {
  const out: [string, string][] = []
  for (const m of s.matchAll(/([\w.:-]+)=("(?:[^"\\]|\\.)*"|\S*)/g))
    out.push([m[1]!, m[2]!.startsWith('"') ? m[2]!.slice(1, -1).replace(/\\"/g, '"') : m[2]!])
  return out
}

function flatten(prefix: string, v: unknown, out: [string, string][]) {
  if (v !== null && typeof v === 'object' && !Array.isArray(v)) {
    for (const [k, x] of Object.entries(v)) flatten(prefix ? `${prefix}.${k}` : k, x, out)
  }
  else {
    out.push([prefix, typeof v === 'string' ? v : JSON.stringify(v)])
  }
}

const RE_5424 = /^<(\d{1,3})>1 (\S+) (\S+) (\S+) (\S+) (\S+) (-|\[(?:[^\]\\]|\\.)*\](?:\[(?:[^\]\\]|\\.)*\])*) ?(.*)$/
const RE_3164 = /^(?:<(\d{1,3})>)?([A-Z][a-z]{2} [ \d]\d \d\d:\d\d:\d\d) (\S+) ([^:[\s]+)(?:\[(\d+)\])?: ?(.*)$/
const RE_ASA = /%(ASA|FTD|PIX|FWSM)-(\d)-(\d{6}): (.*)$/
const RE_ACCESS = /^(\S+) (\S+) (\S+) \[([^\]]+)\] "([A-Z]+) (\S+) ([^"]+)" (\d{3}) (\d+|-)(?: "([^"]*)" "([^"]*)")?/

export function parseLine(raw: string, line = 1): ParsedLine {
  const s = raw.trim()
  const res = (format: Format, fields: [string, string][]): ParsedLine => ({ line, raw, format, fields })
  let m = RE_5424.exec(s)
  if (m) {
    const f: [string, string][] = [...decodePri(Number(m[1])), ['timestamp', m[2]!], ['host', m[3]!], ['app', m[4]!], ['procid', m[5]!], ['msgid', m[6]!]]
    if (m[7] !== '-')
      f.push(['structured-data', m[7]!])
    f.push(['message', m[8]!])
    return res('syslog5424', f)
  }
  if (s.startsWith('{')) {
    try {
      const f: [string, string][] = []
      flatten('', JSON.parse(s), f)
      return res('json', f)
    }
    catch {}
  }
  m = RE_ACCESS.exec(s)
  if (m) {
    return res('access-log', [['client', m[1]!], ['user', m[3]!], ['time', m[4]!], ['method', m[5]!], ['path', m[6]!], ['protocol', m[7]!], ['status', m[8]!], ['bytes', m[9]!], ...(m[10] !== undefined ? [['referer', m[10]] as [string, string], ['user-agent', m[11]!] as [string, string]] : [])])
  }
  const asa = RE_ASA.exec(s)
  m = RE_3164.exec(s)
  const head: [string, string][] = []
  let body = s
  if (m) {
    if (m[1])
      head.push(...decodePri(Number(m[1])))
    head.push(['timestamp', m[2]!], ['host', m[3]!], ['program', m[4]!])
    if (m[5])
      head.push(['pid', m[5]])
    body = m[6]!
  }
  if (asa) {
    const ips = [...asa[4]!.matchAll(/(\w+):(\d{1,3}(?:\.\d{1,3}){3}|[\d:A-F][\dA-F]*:[\d:A-F]+)\/(\d+)/gi)].map(x => `${x[1]} ${x[2]} port ${x[3]}`)
    return res('cisco-asa', [...head, ['product', asa[1]!], ['severity', `${asa[2]} ${SEVERITIES[Number(asa[2])]}`], ['message-id', asa[3]!], ['message', asa[4]!], ...ips.map((v, i) => [`endpoint ${i + 1}`, v] as [string, string])])
  }
  const kv = keyValues(body)
  if (kv.length >= 3)
    return res('keyvalue', [...head, ...kv])
  if (m)
    return res('syslog3164', [...head, ['message', body]])
  return res('unknown', [['text', s]])
}

export function parseLog(text: string): ParsedLine[] {
  return text.split('\n').map((l, i) => ({ l, i })).filter(x => x.l.trim()).slice(0, 500).map(x => parseLine(x.l, x.i + 1))
}

/** A short summary per line: time, source, severity and the message or the most telling fields. */
export function summary(p: ParsedLine): { time: string, source: string, severity: string, text: string } {
  const get = (...keys: string[]) => keys.map(k => p.fields.find(([n]) => n === k)?.[1]).find(Boolean) ?? ''
  return {
    time: get('timestamp', 'time', 'date', '@timestamp', 'ts'),
    source: get('host', 'devname', 'client', 'srcip', 'SRC', 'hostname'),
    severity: get('severity', 'level', 'pri', 'status').replace(/^\d /, ''),
    text: get('message', 'msg', 'action', 'logdesc', 'path') || p.fields.slice(0, 4).map(([k, v]) => `${k}=${v}`).join(' '),
  }
}
