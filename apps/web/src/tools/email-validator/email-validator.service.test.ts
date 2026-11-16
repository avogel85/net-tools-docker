import { describe, expect, it } from 'vitest'
import { counts, splitAddresses } from './email-validator.service'

describe('email-validator', () => {
  it('splits on newlines, commas and semicolons', () => {
    expect(splitAddresses('a@b.test, c@d.test;\n\n e@f.test ')).toEqual(['a@b.test', 'c@d.test', 'e@f.test'])
  })
  it('counts verdicts', () => {
    expect(counts([{ verdict: 'valid' }, { verdict: 'risky' }, { verdict: 'valid' }] as never)).toEqual({ valid: 2, risky: 1, invalid: 0 })
  })
})
