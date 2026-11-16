# Changelog

All notable changes to NET-TOOLS. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project uses [Semantic Versioning](https://semver.org/). Release notes with install
instructions are on the [GitHub releases page](https://github.com/digibaro/net-tools-docker/releases).

## [Unreleased]

## [0.9.0] - 2026-11-16

### Added
- SPF flattener (Email): replaces include, a, mx and redirect with the IP ranges they resolve to,
  merges adjacent ranges, keeps what cannot be flattened (exists, ptr, macros) and splits long
  results over chained `_spfN` records. Shows the DNS lookups before and after and a zone file.
- Generators, all running in the browser:
  - DMARC record generator, with the authorization records for external report addresses.
  - DKIM key pair and selector record generator (RSA or Ed25519; the private key stays in the browser).
  - MTA-STS policy generator: policy file, `_mta-sts` and TLS-RPT records, nginx snippet.
  - DS / DNSKEY tool: computes DS records and checks which published DS records match the zone's keys.
  - NSEC3 hash calculator, with loading of a zone's NSEC3PARAM.
  - nsupdate / TSIG builder: TSIG keys for BIND, Knot and PowerDNS plus nsupdate scripts.
  - Zone file linter: CNAME conflicts, missing glue, missing final dots, SOA values, TTLs and more.
  - SSH key & SSHFP tool: SHA256/MD5 fingerprints, authorized_keys and known_hosts parsing, SSHFP records.
- Email and DNS checks:
  - Email provider detector: mailbox provider, security gateway, sending services, DMARC
    reporting and verified SaaS services, from public DNS.
  - Email address validator: syntax, MX / null MX, disposable and role addresses, typo hints
    (no mail is sent; addresses go in a POST body and are not logged).
  - DKIM signature verifier: paste a raw message or open an .eml file; shows each signature's
    result, alignment with From, Authentication-Results and ARC.
  - DNS snapshot & diff: save a domain's records in the browser and compare snapshots.
  - Subdomain takeover check: dangling CNAMEs and unclaimed pages at hosting services.
  - Lookalike domain finder: typo, homoglyph, IDN and TLD variants and whether they are registered.
  - AD / Kerberos DC locator: the SRV records Windows clients use to find domain controllers.
  - ACME readiness check: CAA, the HTTP-01 challenge path and DNS-01 delegation before you
    request a certificate.
- Service probes:
  - UDP service probe: real requests for DNS, NTP, SNMPv3 (engine discovery, no community),
    SIP, STUN, IKE, QUIC, OpenVPN, NetBIOS and TFTP, to tell open from filtered.
  - IPsec / IKE probe: IKEv2 and IKEv1 proposals, DH groups, fragmentation and vendor IDs.
  - SIP server check: NAPTR/SRV discovery and OPTIONS over UDP, TCP and TLS.
  - LDAP / LDAPS check: anonymous rootDSE, StartTLS and certificate (no bind).
  - Remote access check: RDP network level authentication and certificate, VNC security
    types, SSH banner.
  - Post-quantum TLS check: whether a server negotiates hybrid ML-KEM key exchange.
  - Reverse traceroute: MTR from Globalping probes back to your public IP, with AS paths.
  - RADIUS tester (LAN profile): Status-Server or a test Access-Request, verified with the
    shared secret.
- Web and client tools:
  - Webhook / request inspector: a temporary URL that captures incoming requests for a short time.
  - Social preview checker: Open Graph and Twitter cards, favicons, manifest, preview image size.
  - Broken link checker: a small same-site crawl (30 pages, depth 2) that reports broken links,
    assets and redirect loops.
  - Origin vs CDN compare: the same request through the CDN and directly to an origin IP, side by side.
  - Reverse proxy config generator for nginx, Caddy, Traefik and HAProxy.
  - Expiry watchlist: certificate and domain expiry for a list kept in the browser.
  - The speed test is now also found as the bufferbloat test.
  - VoIP quality test: a voice-like packet stream (G.711, G.729, Opus, G.722) over a WebSocket
    to the server, with round-trip time, jitter, packets too late for the jitter buffer and a
    MOS score (E-model). nginx forwards the WebSocket upgrade on `/api/v1/voip/ws`.
- BGP tools and calculators:
  - Prefix-list generator: IRR (bgpq4) prefix filters for Cisco IOS/IOS-XR, Juniper, Arista,
    FRR, MikroTik, BIRD and OpenBGPD.
  - Prefix watch: a browser-kept list of prefixes with expected origins, checked for unexpected
    origins, foreign more-specifics and RPKI status.
  - ROA & ASPA helper: a minimal ROA plan (Krill delta or CSV) from what an AS announces, and an
    ASPA record builder with AS-path verification.
  - Fiber link budget, Wi-Fi throughput estimator (Wi-Fi 4–7 MCS tables), Wi-Fi channel planner
    (2.4, 5 and 6 GHz, EU/US) and a cabling & transceiver reference.
  - Firewall object converter, capture filter builder (tcpdump/BPF and Wireshark) and log parser.
- DNS lookup accepts the NSEC3PARAM record type.

### Fixed
- Tools that fill several fields at once (such as "Load" in the DMARC generator) no longer lose
  all but the last value in the URL.

## [0.8.1] - 2026-11-15

### Added
- `DNS_RESOLVER`: an unfiltered resolver for every server-side lookup, for hosts whose DNS filters
  (Pi-hole, AdGuard Home). Names it doesn't know still go to the container resolver (LAN hosts),
  which stays selectable as "Local resolver" in DNS lookup, reverse DNS and the benchmark.
- DNS lookup, bulk lookup, the DNS health report and the domain scorecard warn when an answer
  looks blocked by a DNS filter (extended DNS error, `0.0.0.0` / `::`, or a record the reference
  resolver has) and show the unfiltered answer.

## [0.8.0] - 2026-11-13

### Added
- 5 new tools (173 in total):
  - Email: Microsoft 365 domain check (DNS records Microsoft 365 expects, tenant ID, region and
    managed/federated sign-in).
  - HTTP: domain scorecard (internet.nl-style grade for website, email and DNS).
  - Client: IPv6 connection test (scores the visitor's own connection out of 10).
  - New category "Status & links": cloud service status (16 public status sources;
    `STATUS_FEEDS_FILE`) and quick links (`LINKS_FILE`).
- Smart search: typing an IP, domain, URL, ASN, prefix, email or MAC address in Ctrl+K lists the
  tools that take it, filled in, plus external lookup sites.
- "Look up elsewhere" menu on tool pages (Shodan, VirusTotal, crt.sh, DNSViz, Internet.nl,
  bgp.tools and more); the external sites can be switched off under Settings → Privacy.

### Fixed
- DNS health report and email deliverability report: sub-checks run at most 3 at a time, so they
  are no longer refused by the per-visitor concurrency limit and shown as "skipped".

### Security
- node-forge 1.4.3, golang.org/x/net 0.62.0, Go 1.27.4; the web image installs Alpine security
  updates (brotli CVE-2026-111497).

## [0.7.0] - 2026-10-26

### Added
- 17 new tools (168 in total):
  - IP: subnet overlap checker.
  - DNS: DNS health report, DNS TTL change planner, bulk lookup (DNS / PTR / ASN / port check).
  - Connectivity: latency monitor (`MONITOR_MAX_MINUTES`).
  - Email: email deliverability report, TLS-RPT report viewer.
  - BGP: prefix-list tester, BGP community decoder.
  - Calculators: PoE budget, NAT/PAT capacity, wireless link budget, DSCP / ToS converter, config
    diff, ACL analyzer.
  - Client: provider status, report bundle.
- Tool history with pinning, compare mode (one tool twice side by side), "Add to report" on every
  result table.
- Rate-limit feedback: `X-RateLimit-*` headers, retry countdown, remaining-budget hint; stream
  requests get rate-limit and busy rejections as an SSE `error` event.
- Speed test: bufferbloat grade.
- Optional OIDC sign-in (`AUTH_MODE=oidc`) with user and admin roles and signed session cookies;
  `AUTH_ADMIN_TOOLS`, `AUTH_ANONYMOUS_ROLE`.
- `docker-compose.proxy.yml`: join the web container to an existing reverse-proxy network.

### Fixed
- Lab CA generator: the server certificate no longer outlives its CA by a fraction of a second.

## [0.6.0] - 2026-10-15

### Added
- 26 new tools (151 in total):
  - IP: IPv6 address analyzer, IP address plan (IPAM sheet), NAT64 / DNS64 check, country IP blocks.
  - DNS: open resolver test, reverse DNS coverage (FCrDNS), HTTPS / SVCB & ECH viewer, EDNS
    compliance test.
  - HTTP: CDN & WAF detector, cache header analyzer, mixed content checker, CSP builder & evaluator.
  - TLS: DANE / TLSA checker, certificate chain validator, PKCS#12 / PFX inspector, lab CA &
    certificate generator.
  - Email: SMTP banner & FCrDNS check.
  - BGP: route history & visibility, ASN neighbour graph, ROA coverage per ASN, common IXPs &
    facilities.
  - Calculators: SLA & uptime, VoIP bandwidth & Erlang B, packet decoder, pcap viewer.
  - Client: DNS hijack & proxy check.
- HTTP/2 & HTTP/3 check: a real HTTP/3 request, TTFB compared with TCP, session resumption and
  0-RTT.
- Traceroute and MTR: AS path and the route on a world map (with geolocation available).

## [0.5.0] - 2026-10-15

### Added
- LAN tools: **LLDP / CDP neighbours** (which switch and port the server is plugged into),
  **IPv6 neighbours & routers** (NDP scan plus router advertisements: prefixes, SLAAC / DHCPv6,
  DNS, MTU) and **DHCPv6 probe** (rogue DHCPv6 check; never takes a lease). 125 tools in total.
- Dutch interface (Nederlands) with an NL / UK switch in the top bar; the choice is remembered and
  sets the page language. Numbers follow the chosen language (decimal comma in Dutch). A unit test
  keeps every translation in step with English.
- World map of probe locations for every Worldwide (Globalping) result; select a dot to jump to it.
  The map is bundled (Natural Earth), no external tiles.
- CSV download for every result table, and a print / save-as-PDF button on every tool.
- Cloud / CDN IP lookup covers Microsoft Azure (Service Tags, with service and region).
- Optional audit log (`AUDIT_LOG=true`): one line per server-tool request, client IP hashed with
  `LOG_HASH_IPS`, request bodies never logged.
- Subtitle "Tools for Networkers" under the name in the sidebar.
- Favicon, sidebar logo and loading screen follow the accent colour.
- Screenshots in the README (light, dark, phone, worldwide map).
- This changelog.

### Changed
- The "New" badge marks only tools added after the latest release; the release date comes from
  the git tag at build time.
- The sidebar footer shows version and licence only.
- On phones the search box in the top bar becomes a search button, so every header button fits
  at 360 px.
- `make up` and related targets stamp local builds with the git version (for example
  `0.4.0-3-gabc1234`) instead of `dev`.
- Repository renamed to `digibaro/net-tools-docker`; Go module path follows.

### Fixed
- Choice buttons (DNS lookup "Run from", HTTP status classes, URL parser, WireGuard routing) ran
  off the screen on phones; they now wrap.

## [0.4.0] - 2026-10-14

First public release: 122 tools in 11 categories, published as multi-arch images
`ghcr.io/digibaro/nettools-web` and `ghcr.io/digibaro/nettools-api`.

### Added
- LAN mode tools: Wake-on-LAN, ARP scan, mDNS / DNS-SD browser, SSDP / UPnP discovery, DHCP probe
  (rogue DHCP) and gateway / NAT info (NAT-PMP, PCP, UPnP IGD, double-NAT detection).
- Server tools: port scanner, iperf3 bandwidth test, SNMP v2c/v3 get and walk, AS-SET / IRR
  expansion (bgpq4), open relay test, URL / domain reputation (Safe Browsing, VirusTotal,
  urlscan.io).
- Browser tools: router config snippets, AS path visualizer, DMARC aggregate report viewer,
  JWK / PEM / SSH key converter.
- Accent colours (green, blue, indigo, purple, orange, red), checked for WCAG AA contrast.
- Sidebar footer with version and licence; product name shown as NET-TOOLS.
- Release workflow publishing images with SBOM and provenance; gitleaks secret scan in CI.

### Fixed
- LAN override: the web container could not reach the api (every API call failed).
- Speed test: uploads were refused by nginx (413) and long tests hit the rate limit (429).
- TLS scanner: data race on the cipher map.
- Comment-only values in a copied `.env.example` broke startup or enabled tools with bogus keys.
- iperf profile server failed every test (read-only `/tmp`).

### Earlier phases
Phases 0–3 (foundations, 32 browser tools, DNS / connectivity / HTTP / TLS / email / BGP server
tools, providers and Worldwide measurements) were developed before the first release; see the git
history and `docs/IMPLEMENTATION_PLAN.md`.

[Unreleased]: https://github.com/digibaro/net-tools-docker/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/digibaro/net-tools-docker/compare/v0.8.1...v0.9.0
[0.8.1]: https://github.com/digibaro/net-tools-docker/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/digibaro/net-tools-docker/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/digibaro/net-tools-docker/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/digibaro/net-tools-docker/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/digibaro/net-tools-docker/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/digibaro/net-tools-docker/releases/tag/v0.4.0
