import type { DcLocator } from './dc-locator.service'
import { describe, expect, it } from 'vitest'
import { defaultResolver, lookupRows } from './dc-locator.service'

describe('dc-locator', () => {
  it('summarises lookups', () => {
    const r: DcLocator = {
      domain: 'corp.test',
      site: '',
      resolver: 'System',
      dcs: [],
      problems: [],
      warnings: [],
      lookups: [
        { id: 'dc', name: '_ldap._tcp.dc._msdcs.corp.test', port: 389, required: true, rcode: 'NOERROR', targets: [{ priority: 0, weight: 100, port: 389, target: 'dc1.corp.test', addresses: [] }, { priority: 0, weight: 100, port: 3389, target: 'dc2.corp.test', addresses: [] }] },
        { id: 'kerberos-tcp', name: '_kerberos._tcp.corp.test', port: 88, required: true, rcode: 'NXDOMAIN', targets: [] },
        { id: 'gc', name: '_ldap._tcp.gc._msdcs.corp.test', port: 3268, required: false, rcode: 'NXDOMAIN', targets: [] },
      ],
    }
    expect(lookupRows(r).map(x => [x.targets, x.status])).toEqual([
      ['dc1.corp.test, dc2.corp.test:3389', 'ok'],
      ['—', 'missing'],
      ['—', 'absent'],
    ])
  })

  it('picks the local resolver when DNS_RESOLVER is set', () => {
    expect(defaultResolver(true)).toBe('local')
    expect(defaultResolver(false)).toBe('system')
  })
})
