import type { components } from '@/api/schema'

export type HttpOriginCompare = components['schemas']['HttpOriginCompare']

export function isValidIp(s: string): boolean {
  const v = s.trim().replace(/^\[|\]$/g, '')
  if (/^(?:25[0-5]|2[0-4]\d|1?\d?\d)(?:\.(?:25[0-5]|2[0-4]\d|1?\d?\d)){3}$/.test(v))
    return true
  return v.includes(':') && /^[0-9a-f:.]+$/i.test(v) && v.split('::').length <= 2
}

/** Differences first, then fields that are the same. */
export function sortDiffs(d: HttpOriginCompare['diffs']): HttpOriginCompare['diffs'] {
  return [...d].sort((a, b) => Number(a.same) - Number(b.same))
}
