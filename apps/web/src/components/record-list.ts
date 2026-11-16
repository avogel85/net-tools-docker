import { txtStrings } from '@/tools/zone-builder/zone-builder.service'

export interface ListedRecord {
  name: string
  /** Record type; TXT values are quoted and split into 255-byte strings in zone lines. */
  type?: string
  value: string
}

/** A record as a BIND zone-file line. */
export function recordZoneLine(r: ListedRecord, ttl = 3600): string {
  const type = r.type ?? 'TXT'
  const data = type === 'TXT' ? txtStrings(r.value).join(' ') : r.value
  return `${r.name.replace(/\.?$/, '.')} ${ttl} IN ${type} ${data}`
}

export function recordZoneText(records: ListedRecord[], ttl = 3600): string {
  return records.map(r => recordZoneLine(r, ttl)).join('\n')
}
