import { describe, expect, it } from 'vitest'
import { state } from './remote-access-check.service'

const s = (open: boolean, findings: string[] = []) => ({ open, findings }) as never

describe('remote-access-check', () => {
  it('classifies services', () => {
    expect(state(s(false))).toBe('closed')
    expect(state(s(true))).toBe('open')
    expect(state(s(true, ['vnc-no-auth']))).toBe('exposed')
  })
})
