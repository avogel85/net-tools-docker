import type { components } from '@/api/schema'

export type EmailDkimVerify = components['schemas']['EmailDkimVerify']

export const MAX_MESSAGE = 900 * 1024

/** Byte length as the server counts it (UTF-8). */
export function byteLength(s: string): number {
  return new TextEncoder().encode(s).length
}

/** Quick check that the text starts with header lines ("Name: value"). */
export function looksLikeMessage(s: string): boolean {
  return /^[\x21-\x39\x3B-\x7E]+:[ \t]?\S/.test(s.trimStart())
}
