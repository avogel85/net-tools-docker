import type { components } from '@/api/schema'
import { txtStrings } from '../zone-builder/zone-builder.service'

export type EmailSpfFlatten = components['schemas']['EmailSpfFlatten']
type FlatRecord = EmailSpfFlatten['records'][number]

export const DEFAULT_LABEL = '_spf'
export const DEFAULT_SIZE = 450

/** A record of the flattened policy as a zone-file line; long values are split into 255-byte strings. */
export function zoneLine(r: FlatRecord, ttl = 3600): string {
  return `${r.name}. ${ttl} IN TXT ${txtStrings(r.value).join(' ')}`
}

/** All records as zone-file lines, the policy record first. */
export function zoneText(records: FlatRecord[], ttl = 3600): string {
  return records.map(r => zoneLine(r, ttl)).join('\n')
}

/** Valid first label for the chained records (letters, digits, - and _; at most 30). */
export function isValidLabel(label: string): boolean {
  return /^\w[\w-]{0,29}$/.test(label)
}
