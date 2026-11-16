import { describe, expect, it } from 'vitest'
import { analyze } from './arp-conflicts.service'

describe('arp-conflicts', () => {
  it('flags two MACs on one IP as a duplicate', () => {
    const f = analyze([{ ip: '10.0.0.5', mac: 'AA:AA' }, { ip: '10.0.0.5', mac: 'bb:bb' }])
    expect(f).toEqual([{ kind: 'duplicate-ip', ip: '10.0.0.5', detail: 'aa:aa / bb:bb' }])
  })
  it('reports one MAC on several IPs as informational', () => {
    const f = analyze([{ ip: '10.0.0.1', mac: 'aa' }, { ip: '10.0.0.2', mac: 'aa' }])
    expect(f.map(x => x.kind)).toEqual(['multi-ip'])
  })
  it('compares with a baseline', () => {
    const base = [{ ip: '10.0.0.1', mac: 'aa' }, { ip: '10.0.0.2', mac: 'bb' }, { ip: '10.0.0.3', mac: 'cc' }]
    const scan = [{ ip: '10.0.0.1', mac: 'aa' }, { ip: '10.0.0.2', mac: 'zz' }, { ip: '10.0.0.4', mac: 'dd' }]
    expect(analyze(scan, base).map(x => `${x.kind}:${x.ip}`)).toEqual(['mac-changed:10.0.0.2', 'new-device:10.0.0.4', 'gone-device:10.0.0.3'])
  })
  it('is clean when nothing differs', () => {
    const s = [{ ip: '10.0.0.1', mac: 'aa' }]
    expect(analyze(s, s)).toEqual([])
  })
})
