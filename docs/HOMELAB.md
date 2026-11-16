# Homelab additions (fork)

These tools exist only in this fork ([avogel85/net-tools-docker](https://github.com/avogel85/net-tools-docker),
branch `homelab`), not in the original [digibaro/net-tools-docker](https://github.com/digibaro/net-tools-docker).

| Tool | Where | Run | What it does |
|---|---|---|---|
| Homelab inventory | Homelab | Server | Proxmox guests (read-only API token), UniFi clients/devices and an ARP scan side by side; flags hosts nobody knows |
| VLAN reachability matrix | Homelab | Server | Checks hosts and ports from the VLAN this server lives in (useful after firewall or VLAN changes) |
| Internal certificate monitor | Homelab | Server | Expiry, issuer and trust of internal TLS hosts |
| DNS comparison | Homelab | Server | Asks several resolvers the same name and shows where answers differ |
| Public hostname check | Homelab | Server | Resolves and fetches your public hostnames: status, redirects, certificate days left |
| ARP conflict detection | LAN | Server | Duplicate IPs, MAC changes, new and missing devices against a baseline kept in the browser |
| UPS runtime calculator | Calculators & Reference | Browser | How long a UPS or battery bank carries a load |
| Switch CLI troubleshooting | Connectivity & Diagnostics | Browser | Ready-to-paste `show` commands for Juniper Junos, Cisco IOS/IOS-XE, Cisco NX-OS and Arista EOS: port status, flaps, errors, optical power (dBm), LLDP/CDP, power and alarms. Nothing is sent anywhere. The Junos commands come from field use; the Cisco, NX-OS and EOS commands have not been verified on every software version, so check the syntax on yours |

## Enabling the server-side tools

The five Homelab tools only work when both variables are set (they return an error otherwise):

```env
NETTOOLS_LOCAL=true
HOMELAB_CONFIG=/etc/nettools/homelab.json   # default path
```

The ARP scan in *Homelab inventory* and the *ARP conflict detection* tool use the existing LAN scan, so they also
need LAN mode (`NETTOOLS_LAN=true`, `docker-compose.lan.yml`, Linux host) and a trusted network
(`NETTOOLS_PROFILE=homelab`, see [SECURITY.md](SECURITY.md)). The config file is re-read on every request, so edits apply without a restart. It is a JSON file;
every section is optional:

```json
{
  "reach": { "targets": [ { "name": "NAS", "host": "192.168.20.10", "ports": [443, 445], "group": "Storage" } ] },
  "certs": { "hosts": ["app.home.example:443"] },
  "dns": {
    "resolvers": [ { "name": "Local DNS", "addr": "192.168.20.5" }, { "name": "Cloudflare", "addr": "1.1.1.1" } ],
    "defaultName": "example.com"
  },
  "public": { "hosts": ["www.example.com"] },
  "proxmox": { "nodes": [ { "name": "pve1", "url": "https://192.168.20.21:8006", "tokenFile": "/etc/nettools/secrets/pve1.token" } ] },
  "unifi": { "url": "https://192.168.1.1", "keyFile": "/etc/nettools/secrets/unifi.key" }
}
```

- A Proxmox token file contains one line, `user@realm!tokenid=secret`. Use a read-only role (for example `PVEAuditor`).
- The UniFi key file contains an API key created under Network → Settings → Control Plane → Integrations.
- Keep secrets in files outside the repository (mode 600, readable by the service user). They are never sent to the browser.

## Building and running this fork

The published images `ghcr.io/digibaro/nettools-web` and `ghcr.io/digibaro/nettools-api` are built from the
original repository and do **not** contain these tools. Build from this branch:

```bash
git clone -b homelab https://github.com/avogel85/net-tools-docker.git
cd net-tools-docker
docker compose up -d --build
```

## Staying in sync with upstream

This branch is a single commit on top of an upstream release (currently v0.9.0). To update, rebase it onto the
new upstream `main`. Conflicts are usually limited to `apps/web/locales/*.yml` and `apps/web/src/tools/index.ts`.
Upstream requires every API route to be documented in `apps/api/openapi.yaml`; the homelab routes are.
