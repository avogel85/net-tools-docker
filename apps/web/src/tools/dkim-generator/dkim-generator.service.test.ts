import { describe, expect, it } from 'vitest'
import { buildDkimRecord, generateKeys, isValidDomain, isValidSelector, pem } from './dkim-generator.service'

describe('dkim-generator', () => {
  it('builds the selector record', () => {
    expect(buildDkimRecord({ selector: 'S1', domain: 'Example.com.', kind: 'rsa-2048', publicKey: 'AAAA', testing: true, strict: true }))
      .toEqual({ name: 's1._domainkey.example.com', value: 'v=DKIM1; k=rsa; t=y:s; p=AAAA' })
    expect(buildDkimRecord({ selector: 'ed', domain: 'example.com', kind: 'ed25519', publicKey: 'BBBB', testing: false, strict: false }).value)
      .toBe('v=DKIM1; k=ed25519; p=BBBB')
  })

  it('wraps PEM at 64 characters', () => {
    const out = pem(new Uint8Array(100).buffer, 'PRIVATE KEY')
    const lines = out.trim().split('\n')
    expect(lines[0]).toBe('-----BEGIN PRIVATE KEY-----')
    expect(lines.at(-1)).toBe('-----END PRIVATE KEY-----')
    expect(lines.slice(1, -1).every(l => l.length <= 64)).toBe(true)
  })

  it('generates an RSA key (SPKI) and an Ed25519 key (raw 32 bytes)', async () => {
    const rsa = await generateKeys('rsa-1024')
    expect(atob(rsa.publicKey).length).toBeGreaterThan(150)
    expect(rsa.privatePem).toContain('BEGIN PRIVATE KEY')
    const ed = await generateKeys('ed25519')
    expect(atob(ed.publicKey).length).toBe(32)
  })

  it('validates selector and domain', () => {
    expect(isValidSelector('selector1')).toBe(true)
    expect(isValidSelector('s1.mail')).toBe(true)
    expect(isValidSelector('-bad')).toBe(false)
    expect(isValidSelector('a_b')).toBe(false)
    expect(isValidDomain('example.com')).toBe(true)
    expect(isValidDomain('nodot')).toBe(false)
  })
})
