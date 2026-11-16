export type Vendor = 'junos' | 'ios' | 'nxos' | 'eos'
export const VENDORS: Vendor[] = ['junos', 'ios', 'nxos', 'eos']
export type GroupId = 'session' | 'status' | 'optics' | 'errors' | 'neighbors' | 'hardware' | 'logs'

export interface Inputs { device?: string, iface?: string, match?: string }
export interface Command { id: string, text: string, missing: boolean }
export interface Group { id: GroupId, commands: Command[] }

/** {i} interface, {w} wildcard/prefix of the interface, {m} free match text, {d} device. A missing value becomes a visible <placeholder>. */
type Templates = Partial<Record<Vendor, string>>
interface Def { id: string, t: Templates }

const GROUPS: { id: GroupId, defs: Def[] }[] = [
  { id: 'session', defs: [
    { id: 'ssh', t: { junos: 'ssh {d}', ios: 'ssh {d}', nxos: 'ssh {d}', eos: 'ssh {d}' } },
    { id: 'paging', t: { junos: 'set cli screen-length 0', ios: 'terminal length 0', nxos: 'terminal length 0', eos: 'terminal length 0' } },
    { id: 'version', t: { junos: 'show version', ios: 'show version', nxos: 'show version', eos: 'show version' } },
  ] },
  { id: 'status', defs: [
    { id: 'ifBrief', t: { junos: 'show interfaces terse', ios: 'show ip interface brief', nxos: 'show interface brief', eos: 'show interfaces status' } },
    { id: 'ifDown', t: { junos: 'show interfaces terse | match "down"', ios: 'show interfaces status | include notconnect|err-disabled|disabled', nxos: 'show interface brief | include down', eos: 'show interfaces status | include notconnect|errdisabled|disabled' } },
    { id: 'ifOne', t: { junos: 'show interfaces {i}', ios: 'show interfaces {i}', nxos: 'show interface {i}', eos: 'show interfaces {i}' } },
    { id: 'ifWild', t: { junos: 'show interfaces {w} terse', ios: 'show interfaces status | include {w}', nxos: 'show interface brief | include {w}', eos: 'show interfaces status | include {w}' } },
    { id: 'descr', t: { junos: 'show interfaces descriptions', ios: 'show interfaces description', nxos: 'show interface description', eos: 'show interfaces description' } },
    { id: 'descrMatch', t: { junos: 'show interfaces descriptions | match {m}', ios: 'show interfaces description | include {m}', nxos: 'show interface description | include {m}', eos: 'show interfaces description | include {m}' } },
    { id: 'errdisabled', t: { ios: 'show interfaces status err-disabled', nxos: 'show interface status err-disabled', eos: 'show interfaces status errdisabled' } },
    { id: 'lag', t: { junos: 'show lacp interfaces', ios: 'show etherchannel summary', nxos: 'show port-channel summary', eos: 'show port-channel summary' } },
  ] },
  { id: 'optics', defs: [
    { id: 'opticsOne', t: { junos: 'show interfaces diagnostics optics {i}', ios: 'show interfaces {i} transceiver detail', nxos: 'show interface {i} transceiver details', eos: 'show interfaces {i} transceiver' } },
    { id: 'opticsLevels', t: { junos: 'show interfaces diagnostics optics {i} | match "lane|on|dbm"', ios: 'show interfaces {i} transceiver detail | include Power|dBm', nxos: 'show interface {i} transceiver details | include dBm', eos: 'show interfaces {i} transceiver | include dBm|Rx|Tx' } },
    { id: 'opticsPower', t: { junos: 'show interfaces diagnostics optics {i} | match "power  "' } },
    { id: 'opticsAll', t: { junos: 'show interfaces diagnostics optics', ios: 'show interfaces transceiver detail', nxos: 'show interface transceiver details', eos: 'show interfaces transceiver' } },
    { id: 'transceivers', t: { junos: 'show chassis hardware | match "Xcvr|SFP|QSFP"', ios: 'show inventory', nxos: 'show interface transceiver', eos: 'show inventory' } },
  ] },
  { id: 'errors', defs: [
    { id: 'flap', t: { junos: 'show interfaces {i} | match flap', ios: 'show interfaces {i} | include line protocol|flapped', nxos: 'show interface {i} | include flapped|resets', eos: 'show interfaces {i} | include line protocol|link status changes' } },
    { id: 'errorsOne', t: { junos: 'show interfaces {i} extensive | match "rrors|rops|CRC|FCS|Carrier transitions"', ios: 'show interfaces {i} counters errors', nxos: 'show interface {i} counters errors', eos: 'show interfaces {i} counters errors' } },
    { id: 'errorsAll', t: { junos: 'show interfaces extensive | match "^Physical|Input errors|Output errors"', ios: 'show interfaces counters errors', nxos: 'show interface counters errors', eos: 'show interfaces counters errors' } },
    { id: 'discards', t: { eos: 'show interfaces counters discards' } },
  ] },
  { id: 'neighbors', defs: [
    { id: 'lldp', t: { junos: 'show lldp neighbors', ios: 'show lldp neighbors', nxos: 'show lldp neighbors', eos: 'show lldp neighbors' } },
    { id: 'lldpOne', t: { junos: 'show lldp neighbors interface {i}', ios: 'show lldp neighbors {i} detail', nxos: 'show lldp neighbors interface {i} detail', eos: 'show lldp neighbors {i} detail' } },
    { id: 'lldpMatch', t: { junos: 'show lldp neighbors | match {m}', ios: 'show lldp neighbors | include {m}', nxos: 'show lldp neighbors | include {m}', eos: 'show lldp neighbors | include {m}' } },
    { id: 'cdp', t: { ios: 'show cdp neighbors', nxos: 'show cdp neighbors' } },
    { id: 'cdpOne', t: { ios: 'show cdp neighbors {i} detail', nxos: 'show cdp neighbors interface {i} detail' } },
    { id: 'macOne', t: { junos: 'show ethernet-switching table interface {i}', ios: 'show mac address-table interface {i}', nxos: 'show mac address-table interface {i}', eos: 'show mac address-table interface {i}' } },
  ] },
  { id: 'hardware', defs: [
    { id: 'alarms', t: { junos: 'show chassis alarms' } },
    { id: 'sysAlarms', t: { junos: 'show system alarms' } },
    { id: 'env', t: { junos: 'show chassis environment', ios: 'show environment all', nxos: 'show environment', eos: 'show environment all' } },
    { id: 'power', t: { junos: 'show chassis power', ios: 'show environment power', nxos: 'show environment power', eos: 'show environment power' } },
    { id: 'pem', t: { junos: 'show chassis environment pem' } },
    { id: 'fans', t: { junos: 'show chassis fan', ios: 'show environment fan', nxos: 'show environment fan', eos: 'show environment cooling' } },
    { id: 'temp', t: { junos: 'show chassis temperature-thresholds', ios: 'show environment temperature', nxos: 'show environment temperature', eos: 'show environment temperature' } },
    { id: 'modules', t: { junos: 'show chassis fpc', nxos: 'show module' } },
    { id: 'inventory', t: { junos: 'show chassis hardware', ios: 'show inventory', nxos: 'show inventory', eos: 'show inventory' } },
    { id: 'poe', t: { junos: 'show poe interface {i}', ios: 'show power inline {i}' } },
  ] },
  { id: 'logs', defs: [
    { id: 'logIface', t: { junos: 'show log messages | match {i} | last 50', ios: 'show logging | include {i}', nxos: 'show logging logfile | include {i}', eos: 'show logging | include {i}' } },
    { id: 'logLink', t: { junos: 'show log messages | match "LINK_DOWN|LINK_UP" | last 50', ios: 'show logging | include UPDOWN', nxos: 'show logging logfile | include IF_DOWN|IF_UP', eos: 'show logging | include UPDOWN' } },
    { id: 'logLast', t: { junos: 'show log messages | last 50', ios: 'show logging', nxos: 'show logging last 50', eos: 'show logging last 30 minutes' } },
  ] },
]

const JUNOS_IF = /^(?:(?:ge|xe|et|fe|mge|xle|fte|so|em|fxp|me|vme)-\d+\/\d+\/\d+(?::\d+)?(?:\.\d+)?|(?:ae|lo|reth|em|fxp|me|vme)\d+(?:\.\d+)?|(?:irb|vlan)(?:\.\d+)?)$/
const CISCO_LIKE_IF = /^[a-z][a-z-]*\d+(?:\/\d+){0,3}(?::\d+)?(?:\.\d+)?$/i
const HOSTNAME = /^(?:[a-z0-9]|[a-z0-9][\w.-]*[a-z0-9])$/i

/** Match text goes into a pasted terminal line: drop control characters (a newline would run a second command) and double quotes. */
export function cleanMatch(s: string): string {
  // eslint-disable-next-line no-control-regex
  return s.replace(/[\u0000-\u001F\u007F"]/g, ' ').replace(/\s+/g, ' ').trim()
}

export function isValidIface(vendor: Vendor, s: string): boolean {
  return (vendor === 'junos' ? JUNOS_IF : CISCO_LIKE_IF).test(s.trim())
}

export function isValidDevice(s: string): boolean {
  return HOSTNAME.test(s.trim()) && s.trim().length <= 253
}

/** Junos: xe-1/1/9:0 -> xe-1/1/*, ae0 -> ae*. Others: Te1/0/5 -> Te1/0/ (a prefix for `| include`). */
export function wildcardIface(vendor: Vendor, iface: string): string {
  const s = iface.trim()
  if (!s || !isValidIface(vendor, s))
    return ''
  if (vendor === 'junos')
    return s.replace(/(?::\d+)?(?:\.\d+)?$/, '').replace(/\d+$/, '*')
  return s.replace(/(?:\.\d+)?$/, '').replace(/\d+$/, '')
}

export const IFACE_EXAMPLES: Record<Vendor, string[]> = {
  junos: ['xe-0/0/0', 'xe-1/1/9:0', 'et-0/0/48', 'ge-0/0/1', 'ae0'],
  ios: ['Gi1/0/1', 'Te1/1/1', 'Hu1/0/1', 'Po1'],
  nxos: ['Ethernet1/1', 'Ethernet1/49/1', 'port-channel1'],
  eos: ['Ethernet1', 'Ethernet1/1', 'Port-Channel1', 'Management1'],
}

export function buildGroups(vendor: Vendor, inputs: Inputs): Group[] {
  const iface = (inputs.iface ?? '').trim()
  const valid = iface && isValidIface(vendor, iface)
  const values: Record<string, string> = {
    i: valid ? iface : '',
    w: wildcardIface(vendor, iface),
    m: cleanMatch(inputs.match ?? ''),
    d: (inputs.device ?? '').trim() && isValidDevice(inputs.device ?? '') ? (inputs.device ?? '').trim() : '',
  }
  const placeholders: Record<string, string> = { i: '<interface>', w: '<interface-prefix>', m: '<match>', d: '<device>' }
  return GROUPS.map(g => ({
    id: g.id,
    commands: g.defs.flatMap((d) => {
      const tpl = d.t[vendor]
      if (!tpl)
        return []
      let missing = false
      const text = tpl.replace(/\{([iwmd])\}/g, (_, k: string) => {
        const v = values[k] ?? ''
        if (v)
          return k === 'm' && vendor === 'junos' && /[\s|]/.test(v) ? `"${v}"` : v
        missing = true
        return placeholders[k] ?? ''
      })
      return [{ id: d.id, text, missing }]
    }),
  }))
}

export function groupText(g: Group): string {
  return g.commands.map(c => c.text).join('\n')
}
