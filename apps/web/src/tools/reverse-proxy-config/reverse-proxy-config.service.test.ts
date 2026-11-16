import type { ProxyInput } from './reverse-proxy-config.service'
import { describe, expect, it } from 'vitest'
import { generate, parseUpstream, validate } from './reverse-proxy-config.service'

const base: ProxyInput = { hosts: ['app.example.com', 'www.example.com'], upstream: 'http://127.0.0.1:3000', tls: 'acme', certPath: '', keyPath: '', redirectHttp: true, hsts: true, websockets: true, compression: true, securityHeaders: true, maxBodyMb: 50, timeoutS: 120 }

describe('reverse-proxy-config', () => {
  it('parses upstreams', () => {
    expect(parseUpstream('http://127.0.0.1:3000')).toEqual({ scheme: 'http', host: '127.0.0.1', port: 3000 })
    expect(parseUpstream('https://backend.internal')).toEqual({ scheme: 'https', host: 'backend.internal', port: 443 })
    expect(parseUpstream('http://x/path')).toBeUndefined()
    expect(parseUpstream('ftp://x')).toBeUndefined()
  })

  it('validates input', () => {
    expect(validate(base)).toEqual([])
    expect(validate({ ...base, hosts: ['bad host'], upstream: 'nope', tls: 'files', certPath: 'relative' })).toEqual(['hosts', 'upstream', 'paths'])
  })

  it('renders nginx', () => {
    const c = generate(base, 'nginx')
    expect(c).toContain('server_name app.example.com www.example.com;')
    expect(c).toContain('ssl_certificate     /etc/letsencrypt/live/app.example.com/fullchain.pem;')
    expect(c).toContain('location / { return 301 https://$host$request_uri; }')
    expect(c).toContain('proxy_set_header Connection $connection_upgrade;')
    expect(c).toContain('client_max_body_size 50m;')
    expect(c).toContain('proxy_read_timeout 120s;')
    expect(c).toContain('Strict-Transport-Security')
    // Braces balance.
    expect(c.split('{').length).toBe(c.split('}').length)
  })

  it('renders caddy, traefik and haproxy', () => {
    const caddy = generate(base, 'caddy')
    expect(caddy.startsWith('app.example.com, www.example.com {')).toBe(true)
    expect(caddy).toContain('max_size 50MB')
    expect(caddy.split('{').length).toBe(caddy.split('}').length)
    expect(generate({ ...base, tls: 'none' }, 'caddy').startsWith('http://app.example.com, http://www.example.com {')).toBe(true)
    const tr = generate(base, 'traefik')
    expect(tr).toContain('rule: "Host(`app.example.com`) || Host(`www.example.com`)"')
    expect(tr).toContain('certResolver: letsencrypt')
    expect(tr).toContain('maxRequestBodyBytes: 52428800')
    expect(generate(base, 'traefik-labels')).toContain('traefik.http.routers.app-example-com.middlewares=app-example-com-compress,app-example-com-hsts,app-example-com-sec,app-example-com-body')
    const hp = generate(base, 'haproxy')
    expect(hp).toContain('http-request redirect scheme https code 301 unless { ssl_fc }')
    expect(hp).toContain('server app1 127.0.0.1:3000 check')
  })

  it('returns nothing for invalid input', () => {
    expect(generate({ ...base, upstream: 'x' }, 'nginx')).toBe('')
  })
})
