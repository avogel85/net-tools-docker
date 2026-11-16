import type { LanRadius } from './radius-tester.service'
import { describe, expect, it } from 'vitest'
import { verdict } from './radius-tester.service'

const base: LanRadius = { address: '', mode: 'status', answered: true, result: 'accept', replyMessages: [], responseAuthOk: true, messageAuth: 'ok', rttMs: 1 }

describe('radius-tester', () => {
  it('derives a verdict', () => {
    expect(verdict(base)).toBe('ok')
    expect(verdict({ ...base, result: 'reject' })).toBe('rejected')
    expect(verdict({ ...base, responseAuthOk: false })).toBe('untrusted')
    expect(verdict({ ...base, answered: false, result: 'no-reply' })).toBe('silent')
    expect(verdict({ ...base, answered: false, result: 'refused' })).toBe('closed')
  })
})
