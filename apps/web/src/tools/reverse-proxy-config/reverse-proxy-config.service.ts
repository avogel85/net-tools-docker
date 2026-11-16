export interface ProxyInput {
  /** Host names served (first is the primary). */
  hosts: string[]
  /** Upstream URL, e.g. http://127.0.0.1:3000. */
  upstream: string
  tls: 'acme' | 'files' | 'none'
  certPath: string
  keyPath: string
  redirectHttp: boolean
  hsts: boolean
  websockets: boolean
  compression: boolean
  securityHeaders: boolean
  /** Max request body in MB (0 = server default). */
  maxBodyMb: number
  /** Read timeout in seconds for long requests (0 = default). */
  timeoutS: number
}

export type Target = 'nginx' | 'caddy' | 'traefik' | 'traefik-labels' | 'haproxy'
export const TARGETS: Target[] = ['nginx', 'caddy', 'traefik', 'traefik-labels', 'haproxy']

const HOST = /^(?:\*\.)?(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]*$/i

export interface Upstream { scheme: 'http' | 'https', host: string, port: number }

export function parseUpstream(s: string): Upstream | undefined {
  try {
    const u = new URL(s.trim())
    if (u.protocol !== 'http:' && u.protocol !== 'https:')
      return undefined
    if (u.pathname !== '/' || u.search || u.hash || u.username)
      return undefined
    const scheme = u.protocol === 'https:' ? 'https' : 'http'
    return { scheme, host: u.hostname, port: u.port ? Number(u.port) : (scheme === 'https' ? 443 : 80) }
  }
  catch {
    return undefined
  }
}

export function validate(i: ProxyInput): string[] {
  const errors: string[] = []
  if (!i.hosts.length || i.hosts.some(h => !HOST.test(h)))
    errors.push('hosts')
  if (!parseUpstream(i.upstream))
    errors.push('upstream')
  if (i.tls === 'files' && (!i.certPath.startsWith('/') || !i.keyPath.startsWith('/')))
    errors.push('paths')
  return errors
}

const SECURITY_HEADERS: [string, string][] = [
  ['X-Content-Type-Options', 'nosniff'],
  ['Referrer-Policy', 'strict-origin-when-cross-origin'],
  ['X-Frame-Options', 'SAMEORIGIN'],
  ['Permissions-Policy', 'camera=(), microphone=(), geolocation=()'],
]
const HSTS = 'max-age=31536000; includeSubDomains'
const slug = (h: string) => h.replace(/^\*\./, 'wildcard-').replace(/[^a-z0-9]+/gi, '-').replace(/^-|-$/g, '').toLowerCase()

function nginx(i: ProxyInput, u: Upstream): string {
  const names = i.hosts.join(' ')
  const tls = i.tls !== 'none'
  const cert = i.tls === 'acme' ? `/etc/letsencrypt/live/${i.hosts[0]}/fullchain.pem` : i.certPath
  const key = i.tls === 'acme' ? `/etc/letsencrypt/live/${i.hosts[0]}/privkey.pem` : i.keyPath
  const out: string[] = []
  if (i.websockets) {
    out.push('map $http_upgrade $connection_upgrade {', '    default upgrade;', '    \'\'      close;', '}', '')
  }
  if (tls && i.redirectHttp) {
    out.push('server {', '    listen 80;', '    listen [::]:80;', `    server_name ${names};`, '    location /.well-known/acme-challenge/ { root /var/www/certbot; }', '    location / { return 301 https://$host$request_uri; }', '}', '')
  }
  out.push('server {')
  if (tls)
    out.push('    listen 443 ssl;', '    listen [::]:443 ssl;', '    http2 on;')
  else
    out.push('    listen 80;', '    listen [::]:80;')
  out.push(`    server_name ${names};`)
  if (tls) {
    out.push('', `    ssl_certificate     ${cert};`, `    ssl_certificate_key ${key};`, '    ssl_protocols TLSv1.2 TLSv1.3;', '    ssl_session_cache shared:SSL:10m;')
  }
  if (i.maxBodyMb > 0)
    out.push(`    client_max_body_size ${i.maxBodyMb}m;`)
  if (i.compression)
    out.push('    gzip on;', '    gzip_types text/plain text/css application/json application/javascript text/xml application/xml image/svg+xml;')
  if (tls && i.hsts)
    out.push(`    add_header Strict-Transport-Security "${HSTS}" always;`)
  if (i.securityHeaders) {
    for (const [k, v] of SECURITY_HEADERS) out.push(`    add_header ${k} "${v}" always;`)
  }
  out.push('', '    location / {', `        proxy_pass ${u.scheme}://${u.host}:${u.port};`, '        proxy_http_version 1.1;', '        proxy_set_header Host $host;', '        proxy_set_header X-Real-IP $remote_addr;', '        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;', '        proxy_set_header X-Forwarded-Proto $scheme;')
  if (i.websockets)
    out.push('        proxy_set_header Upgrade $http_upgrade;', '        proxy_set_header Connection $connection_upgrade;')
  if (u.scheme === 'https')
    out.push('        proxy_ssl_server_name on;')
  if (i.timeoutS > 0)
    out.push(`        proxy_read_timeout ${i.timeoutS}s;`, `        proxy_send_timeout ${i.timeoutS}s;`)
  out.push('    }', '}')
  return `${out.join('\n')}\n`
}

function caddy(i: ProxyInput, u: Upstream): string {
  const site = i.tls === 'none' ? i.hosts.map(h => `http://${h}`).join(', ') : i.hosts.join(', ')
  const out = [`${site} {`]
  if (i.tls === 'files')
    out.push(`    tls ${i.certPath} ${i.keyPath}`)
  if (i.maxBodyMb > 0)
    out.push('    request_body {', `        max_size ${i.maxBodyMb}MB`, '    }')
  if (i.compression)
    out.push('    encode zstd gzip')
  const headers: [string, string][] = []
  if (i.tls !== 'none' && i.hsts)
    headers.push(['Strict-Transport-Security', HSTS])
  if (i.securityHeaders)
    headers.push(...SECURITY_HEADERS)
  if (headers.length) {
    out.push('    header {')
    for (const [k, v] of headers) out.push(`        ${k} "${v}"`)
    out.push('    }')
  }
  const target = `${u.scheme === 'https' ? 'https://' : ''}${u.host}:${u.port}`
  if (i.timeoutS > 0 || u.scheme === 'https') {
    out.push(`    reverse_proxy ${target} {`)
    if (i.timeoutS > 0)
      out.push('        transport http {', `            read_timeout ${i.timeoutS}s`, '        }')
    out.push('    }')
  }
  else {
    out.push(`    reverse_proxy ${target}`)
  }
  out.push('}')
  // Caddy redirects HTTP to HTTPS and handles websockets automatically.
  return `${out.join('\n')}\n`
}

function traefik(i: ProxyInput, u: Upstream): string {
  const name = slug(i.hosts[0]!)
  const rule = i.hosts.map(h => `Host(\`${h}\`)`).join(' || ')
  const tls = i.tls !== 'none'
  const mw: string[] = []
  const out = ['http:', '  routers:', `    ${name}:`, `      rule: "${rule}"`, `      entryPoints: [${tls ? 'websecure' : 'web'}]`, `      service: ${name}`]
  if (i.compression)
    mw.push(`${name}-compress`)
  if (i.securityHeaders || (tls && i.hsts))
    mw.push(`${name}-headers`)
  if (i.maxBodyMb > 0)
    mw.push(`${name}-body`)
  if (mw.length)
    out.push(`      middlewares: [${mw.join(', ')}]`)
  if (i.tls === 'acme')
    out.push('      tls:', '        certResolver: letsencrypt')
  else if (i.tls === 'files')
    out.push('      tls: {}')
  out.push('  services:', `    ${name}:`, '      loadBalancer:', '        servers:', `          - url: "${u.scheme}://${u.host}:${u.port}"`)
  if (mw.length) {
    out.push('  middlewares:')
    if (i.compression)
      out.push(`    ${name}-compress:`, '      compress: {}')
    if (i.securityHeaders || (tls && i.hsts)) {
      out.push(`    ${name}-headers:`, '      headers:')
      if (tls && i.hsts)
        out.push('        stsSeconds: 31536000', '        stsIncludeSubdomains: true')
      if (i.securityHeaders)
        out.push('        contentTypeNosniff: true', '        referrerPolicy: "strict-origin-when-cross-origin"', '        customFrameOptionsValue: "SAMEORIGIN"', '        permissionsPolicy: "camera=(), microphone=(), geolocation=()"')
    }
    if (i.maxBodyMb > 0)
      out.push(`    ${name}-body:`, '      buffering:', `        maxRequestBodyBytes: ${i.maxBodyMb * 1024 * 1024}`)
  }
  if (i.tls === 'files')
    out.push('tls:', '  certificates:', `    - certFile: ${i.certPath}`, `      keyFile: ${i.keyPath}`)
  out.push('# The HTTP to HTTPS redirect and the letsencrypt resolver are set on the entry points in the static config.')
  return `${out.join('\n')}\n`
}

function traefikLabels(i: ProxyInput, u: Upstream): string {
  const name = slug(i.hosts[0]!)
  const rule = i.hosts.map(h => `Host(\`${h}\`)`).join(' || ')
  const tls = i.tls !== 'none'
  const l = ['labels:', '  - "traefik.enable=true"', `  - "traefik.http.routers.${name}.rule=${rule}"`, `  - "traefik.http.routers.${name}.entrypoints=${tls ? 'websecure' : 'web'}"`]
  if (i.tls === 'acme')
    l.push(`  - "traefik.http.routers.${name}.tls.certresolver=letsencrypt"`)
  else if (i.tls === 'files')
    l.push(`  - "traefik.http.routers.${name}.tls=true"`)
  l.push(`  - "traefik.http.services.${name}.loadbalancer.server.port=${u.port}"`)
  if (u.scheme === 'https')
    l.push(`  - "traefik.http.services.${name}.loadbalancer.server.scheme=https"`)
  const mw: string[] = []
  if (i.compression) {
    mw.push(`${name}-compress`)
    l.push(`  - "traefik.http.middlewares.${name}-compress.compress=true"`)
  }
  if (tls && i.hsts) {
    mw.push(`${name}-hsts`)
    l.push(`  - "traefik.http.middlewares.${name}-hsts.headers.stsSeconds=31536000"`, `  - "traefik.http.middlewares.${name}-hsts.headers.stsIncludeSubdomains=true"`)
  }
  if (i.securityHeaders) {
    mw.push(`${name}-sec`)
    l.push(`  - "traefik.http.middlewares.${name}-sec.headers.contentTypeNosniff=true"`, `  - "traefik.http.middlewares.${name}-sec.headers.referrerPolicy=strict-origin-when-cross-origin"`, `  - "traefik.http.middlewares.${name}-sec.headers.customFrameOptionsValue=SAMEORIGIN"`)
  }
  if (i.maxBodyMb > 0) {
    mw.push(`${name}-body`)
    l.push(`  - "traefik.http.middlewares.${name}-body.buffering.maxRequestBodyBytes=${i.maxBodyMb * 1024 * 1024}"`)
  }
  if (mw.length)
    l.push(`  - "traefik.http.routers.${name}.middlewares=${mw.join(',')}"`)
  l.push(`# The container must be on Traefik's network; the upstream host (${u.host}) is the container itself.`)
  return `${l.join('\n')}\n`
}

function haproxy(i: ProxyInput, u: Upstream): string {
  const name = slug(i.hosts[0]!)
  const tls = i.tls !== 'none'
  const pem = i.tls === 'acme' ? `/etc/haproxy/certs/${i.hosts[0]}.pem` : i.certPath
  const out = ['frontend http-in', '    mode http', '    bind :80']
  if (tls)
    out.push(`    bind :443 ssl crt ${pem} alpn h2,http/1.1`)
  if (tls && i.redirectHttp)
    out.push('    http-request redirect scheme https code 301 unless { ssl_fc }')
  out.push('    option forwardfor', '    http-request set-header X-Forwarded-Proto https if { ssl_fc }')
  if (i.maxBodyMb > 0)
    out.push(`    http-request deny deny_status 413 if { req.body_size gt ${i.maxBodyMb * 1024 * 1024} }`)
  if (tls && i.hsts)
    out.push(`    http-response set-header Strict-Transport-Security "${HSTS}"`)
  if (i.securityHeaders) {
    for (const [k, v] of SECURITY_HEADERS) out.push(`    http-response set-header ${k} "${v}"`)
  }
  out.push(`    acl host_${name} hdr(host),field(1,:) -i ${i.hosts.join(' ')}`, `    use_backend ${name} if host_${name}`, '', `backend ${name}`, '    mode http')
  if (i.compression)
    out.push('    compression algo gzip', '    compression type text/html text/plain text/css application/javascript application/json')
  if (i.timeoutS > 0)
    out.push(`    timeout server ${i.timeoutS}s`)
  if (i.websockets)
    out.push('    timeout tunnel 1h')
  out.push(`    server app1 ${u.host}:${u.port}${u.scheme === 'https' ? ' ssl verify required ca-file /etc/ssl/certs/ca-certificates.crt' : ''} check`)
  if (i.tls === 'acme')
    out.push('', '# HAProxy needs certificate and key concatenated in one .pem; renew with certbot and a deploy hook.')
  return `${out.join('\n')}\n`
}

export function generate(i: ProxyInput, target: Target): string {
  const u = parseUpstream(i.upstream)
  if (!u || validate(i).length)
    return ''
  switch (target) {
    case 'nginx': return nginx(i, u)
    case 'caddy': return caddy(i, u)
    case 'traefik': return traefik(i, u)
    case 'traefik-labels': return traefikLabels(i, u)
    case 'haproxy': return haproxy(i, u)
  }
}
