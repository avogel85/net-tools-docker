import { describe, expect, it } from 'vitest'
import { byteLength, looksLikeMessage } from './dkim-verifier.service'

describe('dkim-verifier', () => {
  it('recognises raw messages', () => {
    expect(looksLikeMessage('DKIM-Signature: v=1; a=rsa-sha256\r\nFrom: a@b.c\r\n\r\nbody')).toBe(true)
    expect(looksLikeMessage('Hello Bob, how are you?')).toBe(false)
  })
  it('counts UTF-8 bytes', () => {
    expect(byteLength('é')).toBe(2)
  })
})
