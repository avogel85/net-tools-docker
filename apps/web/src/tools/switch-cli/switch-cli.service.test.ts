import { describe, expect, it } from 'vitest'
import { buildGroups, groupText, isValidDevice, isValidIface, wildcardIface } from './switch-cli.service'

const find = (gs: ReturnType<typeof buildGroups>, id: string) => gs.flatMap(g => g.commands).find(c => c.id === id)

describe('switch-cli', () => {
  it('validates interface names per vendor', () => {
    expect(['xe-1/1/9:0', 'ae0', 'et-0/0/48', 'irb.10', 'fe-0/0/1.0'].every(i => isValidIface('junos', i))).toBe(true)
    expect(isValidIface('junos', 'Gi1/0/1')).toBe(false)
    expect(['Gi1/0/1', 'Te1/1/1', 'Po1'].every(i => isValidIface('ios', i))).toBe(true)
    expect(['Ethernet1/1', 'Ethernet1/49/1', 'port-channel1'].every(i => isValidIface('nxos', i))).toBe(true)
    expect(['Ethernet1', 'Management1', 'Port-Channel1'].every(i => isValidIface('eos', i))).toBe(true)
    expect(isValidIface('eos', 'Ethernet 1; reboot')).toBe(false)
    expect(isValidIface('ios', 'x"; rm')).toBe(false)
  })
  it('validates device names', () => {
    expect(isValidDevice('sw-ams01.example.net')).toBe(true)
    expect(isValidDevice('192.0.2.10')).toBe(true)
    expect(isValidDevice('a b')).toBe(false)
    expect(isValidDevice('x;reboot')).toBe(false)
  })
  it('derives wildcards', () => {
    expect(wildcardIface('junos', 'xe-1/1/9:0')).toBe('xe-1/1/*')
    expect(wildcardIface('junos', 'fe-0/0/3')).toBe('fe-0/0/*')
    expect(wildcardIface('junos', 'ae0')).toBe('ae*')
    expect(wildcardIface('ios', 'Te1/0/5')).toBe('Te1/0/')
    expect(wildcardIface('eos', 'Ethernet12')).toBe('Ethernet')
    expect(wildcardIface('junos', 'nonsense')).toBe('')
  })
  it('fills the Junos commands from the example list', () => {
    const gs = buildGroups('junos', { iface: 'xe-1/1/9:0', match: 'BB-002.AMS101', device: 'sw1' })
    expect(find(gs, 'opticsLevels')?.text).toBe('show interfaces diagnostics optics xe-1/1/9:0 | match "lane|on|dbm"')
    expect(find(gs, 'opticsPower')?.text).toBe('show interfaces diagnostics optics xe-1/1/9:0 | match "power  "')
    expect(find(gs, 'flap')?.text).toBe('show interfaces xe-1/1/9:0 | match flap')
    expect(find(gs, 'descrMatch')?.text).toBe('show interfaces descriptions | match BB-002.AMS101')
    expect(find(gs, 'ifWild')?.text).toBe('show interfaces xe-1/1/* terse')
    expect(find(gs, 'ssh')?.text).toBe('ssh sw1')
    expect(find(gs, 'pem')?.text).toBe('show chassis environment pem')
    expect(find(gs, 'ifOne')?.missing).toBe(false)
  })
  it('shows placeholders when fields are empty or invalid', () => {
    const gs = buildGroups('ios', { iface: 'bad iface;', device: '' })
    expect(find(gs, 'opticsOne')).toMatchObject({ text: 'show interfaces <interface> transceiver detail', missing: true })
    expect(find(gs, 'ssh')?.text).toBe('ssh <device>')
    expect(find(gs, 'ifBrief')?.missing).toBe(false)
  })
  it('quotes Junos match text with spaces or pipes only', () => {
    expect(find(buildGroups('junos', { match: 'core 1' }), 'descrMatch')?.text).toBe('show interfaces descriptions | match "core 1"')
    expect(find(buildGroups('eos', { match: 'core 1' }), 'descrMatch')?.text).toBe('show interfaces description | include core 1')
  })
  it('never lets match text break out of the line', () => {
    expect(find(buildGroups('junos', { match: 'a\nshow version" | x' }), 'descrMatch')?.text).toBe('show interfaces descriptions | match "a show version | x"')
    expect(find(buildGroups('ios', { match: 'x\ry' }), 'descrMatch')?.text).toBe('show interfaces description | include x y')
  })
  it('omits commands a vendor lacks and covers all vendors', () => {
    expect(find(buildGroups('eos', {}), 'cdp')).toBeUndefined()
    expect(find(buildGroups('ios', {}), 'alarms')).toBeUndefined()
    for (const v of ['junos', 'ios', 'nxos', 'eos'] as const) {
      const gs = buildGroups(v, { iface: v === 'junos' ? 'xe-0/0/1' : 'Ethernet1' })
      expect(gs.map(g => g.id)).toContain('optics')
      expect(find(gs, 'opticsOne')?.text).toContain(v === 'junos' ? 'xe-0/0/1' : 'Ethernet1')
    }
    expect(groupText(buildGroups('junos', {}).find(g => g.id === 'session') ?? { id: 'session', commands: [] }).split('\n').length).toBe(3)
  })
})
