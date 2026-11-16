import { describe, expect, it } from 'vitest'
import { md5 } from './md5'
import { fingerprints, keyWarnings, parseKeys } from './ssh-key-tool.service'

const hex = (b: Uint8Array) => [...b].map(x => x.toString(16).padStart(2, '0')).join('')
const ED = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl user@host'

describe('ssh-key-tool', () => {
  it('computes MD5 (RFC 1321 vectors)', () => {
    const enc = new TextEncoder()
    expect(hex(md5(enc.encode('')))).toBe('d41d8cd98f00b204e9800998ecf8427e')
    expect(hex(md5(enc.encode('abc')))).toBe('900150983cd24fb0d6963f7d28e17f72')
    expect(hex(md5(enc.encode('1234567890'.repeat(8))))).toBe('57edf4a22be3c955ac49da2e2107b67a')
  })

  it('parses an Ed25519 public key and fingerprints it', async () => {
    const { keys, errors } = parseKeys(ED)
    expect(errors).toEqual([])
    expect(keys[0]).toMatchObject({ type: 'ssh-ed25519', bits: 256, comment: 'user@host', hosts: [] })
    const fp = await fingerprints(keys[0]!)
    // Same as `ssh-keygen -l` and `ssh-keygen -E md5 -l`.
    expect(fp.sha256).toBe('SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU')
    expect(fp.md5).toBe('MD5:65:96:2d:fc:e8:d5:a9:11:64:0c:0f:ea:00:6e:5b:bd')
    expect(fp.sshfp?.algorithm).toBe(4)
    expect(fp.sshfp?.sha256).toHaveLength(64)
  })

  it('reads authorized_keys options and known_hosts hosts', () => {
    const text = [
      `command="echo hi there",no-pty ${ED}`,
      `host1.example.com,192.0.2.1 ${ED.split(' ').slice(0, 2).join(' ')}`,
      `|1|abc=|def= ${ED.split(' ').slice(0, 2).join(' ')}`,
      'ssh-ed25519 notbase64!!',
      '# comment',
    ].join('\n')
    const { keys, errors } = parseKeys(text)
    expect(keys.map(k => k.hosts)).toEqual([[], ['host1.example.com', '192.0.2.1'], ['(hashed)']])
    expect(keys[0]!.comment).toBe('user@host')
    expect(errors).toEqual([4])
  })

  it('warns about weak key types', () => {
    expect(keyWarnings({ type: 'ssh-dss', bits: 1024 } as never)).toEqual(['dsa'])
    expect(keyWarnings({ type: 'ssh-rsa', bits: 1024 } as never)).toEqual(['rsa-short'])
    expect(keyWarnings({ type: 'ssh-ed25519', bits: 256 } as never)).toEqual([])
  })
})
