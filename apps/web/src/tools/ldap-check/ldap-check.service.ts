import type { components } from '@/api/schema'

export type LdapCheck = components['schemas']['LdapCheck']

const ORDER = ['dnsHostName', 'serverName', 'defaultNamingContext', 'rootDomainNamingContext', 'namingContexts', 'configurationNamingContext', 'vendorName', 'vendorVersion', 'supportedLDAPVersion', 'supportedSASLMechanisms', 'domainFunctionality', 'forestFunctionality', 'domainControllerFunctionality', 'isGlobalCatalogReady', 'isSynchronized', 'currentTime', 'supportedExtension']

/** rootDSE attributes in a readable order (known ones first). */
export function rootDseRows(dse: Record<string, string[]>): { key: string, values: string[] }[] {
  const keys = Object.keys(dse)
  const known = ORDER.filter(k => keys.some(x => x.toLowerCase() === k.toLowerCase()))
  const rest = keys.filter(k => !ORDER.some(o => o.toLowerCase() === k.toLowerCase())).sort()
  return [...known, ...rest].map((k) => {
    const actual = keys.find(x => x.toLowerCase() === k.toLowerCase())!
    return { key: actual, values: dse[actual] ?? [] }
  })
}

const AD_LEVELS: Record<string, string> = { 0: '2000', 1: '2003 interim', 2: '2003', 3: '2008', 4: '2008 R2', 5: '2012', 6: '2012 R2', 7: '2016', 10: '2025' }

/** Active Directory functional levels as Windows Server versions. */
export function functionalLevel(v: string): string {
  return AD_LEVELS[v] ? `${v} (Windows Server ${AD_LEVELS[v]})` : v
}
