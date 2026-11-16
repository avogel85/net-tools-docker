import type { components } from '@/api/schema'

export type TlsAcme = components['schemas']['TlsAcme']

export const CAS = ['letsencrypt', 'zerossl', 'google', 'buypass', 'sslcom'] as const

/** Accepts host names and "*.example.com" wildcards. */
export function isValidCertName(s: string): boolean {
  return /^(?:\*\.)?(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{0,62}\.?$/i.test(s.trim())
}

export type Verdict = 'ready' | 'warnings' | 'blocked'
export function verdict(r: TlsAcme): Verdict {
  return r.problems.length ? 'blocked' : r.warnings.some(w => w !== 'ipv6-used' && w !== 'wildcard-dns01') ? 'warnings' : 'ready'
}
