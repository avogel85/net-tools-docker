import type { components } from '@/api/schema'

export type EmailValidate = components['schemas']['EmailValidate']
type Check = EmailValidate['results'][number]

export const MAX_ADDRESSES = 50

export function splitAddresses(text: string): string[] {
  return text.split(/[\n,;]+/).map(s => s.trim()).filter(Boolean)
}

export function counts(results: Check[]): Record<Check['verdict'], number> {
  const c = { valid: 0, risky: 0, invalid: 0 }
  for (const r of results) c[r.verdict]++
  return c
}
