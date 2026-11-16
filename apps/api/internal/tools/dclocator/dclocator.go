// Package dclocator implements DNS-29: the SRV records Active Directory clients use to find
// domain controllers (MS-ADTS §6.3.2 "DNS-based discovery").
package dclocator

import (
	"context"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/miekg/dns"

	"github.com/digibaro/net-tools-docker/apps/api/internal/dnsx"
	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnslookup"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// MaxTargets caps the SRV targets whose addresses are looked up.
const MaxTargets = 20

// Tool is the AD / Kerberos DC locator.
type Tool struct{ deps tools.Deps }

func (*Tool) ID() string                                { return "dns.dclocator" }
func (*Tool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Light), d.Slot, d.Timeout(30*time.Second)).Get("/dns/dc-locator", t.handle)
}

// Service is one SRV name a DC registers.
type Service struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Port     uint16 `json:"port"`
	Required bool   `json:"required"`
}

// Target is one SRV record.
type Target struct {
	Priority  uint16   `json:"priority"`
	Weight    uint16   `json:"weight"`
	Port      uint16   `json:"port"`
	Target    string   `json:"target"`
	Addresses []string `json:"addresses"`
}

// Lookup is the answer for one service.
type Lookup struct {
	Service
	Rcode   string   `json:"rcode"`
	Targets []Target `json:"targets"`
}

// DC is one domain controller with the services it is listed for.
type DC struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	Services  []string `json:"services"`
}

// Result is the response payload.
type Result struct {
	Domain   string   `json:"domain"`
	Site     string   `json:"site"`
	Resolver string   `json:"resolver"`
	Lookups  []Lookup `json:"lookups"`
	DCs      []DC     `json:"dcs"`
	Problems []string `json:"problems"`
	Warnings []string `json:"warnings"`
}

// Services returns the SRV names to check for domain (and site, when given).
func Services(domain, site string) []Service {
	s := []Service{
		{"ldap", "_ldap._tcp." + domain, 389, true},
		{"dc", "_ldap._tcp.dc._msdcs." + domain, 389, true},
		{"pdc", "_ldap._tcp.pdc._msdcs." + domain, 389, false},
		{"gc", "_ldap._tcp.gc._msdcs." + domain, 3268, false},
		{"gc-tcp", "_gc._tcp." + domain, 3268, false},
		{"kerberos-tcp", "_kerberos._tcp." + domain, 88, true},
		{"kerberos-udp", "_kerberos._udp." + domain, 88, false},
		{"kerberos-dc", "_kerberos._tcp.dc._msdcs." + domain, 88, false},
		{"kpasswd-tcp", "_kpasswd._tcp." + domain, 464, false},
		{"kpasswd-udp", "_kpasswd._udp." + domain, 464, false},
		{"domain-dns-zones", "_ldap._tcp.DomainDnsZones." + domain, 389, false},
		{"forest-dns-zones", "_ldap._tcp.ForestDnsZones." + domain, 389, false},
	}
	if site != "" {
		s = append(s,
			Service{"site-ldap", "_ldap._tcp." + site + "._sites.dc._msdcs." + domain, 389, true},
			Service{"site-kerberos", "_kerberos._tcp." + site + "._sites.dc._msdcs." + domain, 88, false},
		)
	}
	return s
}

func addOnce(list *[]string, code string) {
	if !slices.Contains(*list, code) {
		*list = append(*list, code)
	}
}

// Locate runs every lookup against srv.
func Locate(ctx context.Context, srv dnsx.Server, domain, site string) (Result, error) {
	res := Result{Domain: domain, Site: site, Resolver: srv.Label, Lookups: []Lookup{}, DCs: []DC{}, Problems: []string{}, Warnings: []string{}}
	services := Services(domain, site)
	lookups := make([]Lookup, len(services))
	errs := make([]error, len(services))
	var wg sync.WaitGroup
	for i, s := range services {
		wg.Go(func() {
			m, _, err := dnsx.Query(ctx, srv, s.Name, dns.TypeSRV)
			if err != nil {
				errs[i] = err
				return
			}
			l := Lookup{Service: s, Rcode: dnsx.RcodeName(m.Rcode), Targets: []Target{}}
			for _, rr := range m.Answer {
				if v, ok := rr.(*dns.SRV); ok {
					l.Targets = append(l.Targets, Target{Priority: v.Priority, Weight: v.Weight, Port: v.Port, Target: strings.ToLower(strings.TrimSuffix(v.Target, ".")), Addresses: []string{}})
				}
			}
			slices.SortFunc(l.Targets, func(a, b Target) int { return strings.Compare(a.Target, b.Target) })
			lookups[i] = l
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return res, dnslookup.Upstream(err)
		}
	}

	// Resolve each distinct target once (capped).
	names := []string{}
	for _, l := range lookups {
		for _, tg := range l.Targets {
			if tg.Target != "" && !slices.Contains(names, tg.Target) {
				names = append(names, tg.Target)
			}
		}
	}
	slices.Sort(names)
	if len(names) > MaxTargets {
		names = names[:MaxTargets]
		addOnce(&res.Warnings, "too-many-targets")
	}
	addrs := make(map[string][]string, len(names))
	var mu sync.Mutex
	for _, n := range names {
		wg.Go(func() {
			a, err := dnsx.Addresses(ctx, srv, n)
			out := []string{}
			if err == nil {
				for _, ip := range a {
					out = append(out, ip.String())
				}
				slices.SortFunc(out, func(x, y string) int { return netip.MustParseAddr(x).Compare(netip.MustParseAddr(y)) })
			}
			mu.Lock()
			addrs[n] = out
			mu.Unlock()
		})
	}
	wg.Wait()

	dcs := map[string]*DC{}
	for i := range lookups {
		l := &lookups[i]
		if len(l.Targets) == 0 {
			if l.Required {
				addOnce(&res.Problems, "missing-"+l.ID)
			}
			continue
		}
		for j := range l.Targets {
			tg := &l.Targets[j]
			if tg.Target == "" {
				continue // "." = service explicitly not available
			}
			if a, ok := addrs[tg.Target]; ok {
				tg.Addresses = a
				if len(a) == 0 {
					addOnce(&res.Problems, "target-no-address")
				}
			}
			if tg.Port != l.Port {
				addOnce(&res.Warnings, "unexpected-port")
			}
			dc := dcs[tg.Target]
			if dc == nil {
				dc = &DC{Name: tg.Target, Addresses: tg.Addresses, Services: []string{}}
				dcs[tg.Target] = dc
			}
			dc.Services = append(dc.Services, l.ID)
		}
	}
	if !hasTargets(lookups, "pdc") && hasTargets(lookups, "dc") {
		addOnce(&res.Warnings, "missing-pdc")
	}
	if !hasTargets(lookups, "gc") && hasTargets(lookups, "dc") {
		addOnce(&res.Warnings, "missing-gc")
	}
	if a, b := targets(lookups, "dc"), targets(lookups, "kerberos-tcp"); len(a) > 0 && len(b) > 0 && !slices.Equal(a, b) {
		addOnce(&res.Warnings, "dc-kerberos-mismatch")
	}
	for _, n := range names {
		if dc, ok := dcs[n]; ok {
			res.DCs = append(res.DCs, *dc)
		}
	}
	res.Lookups = lookups
	return res, nil
}

func targets(ls []Lookup, id string) []string {
	for _, l := range ls {
		if l.ID == id {
			out := []string{}
			for _, t := range l.Targets {
				out = append(out, t.Target)
			}
			return out
		}
	}
	return nil
}

func hasTargets(ls []Lookup, id string) bool { return len(targets(ls, id)) > 0 }

func validSite(s string) bool {
	if s == "" {
		return true
	}
	if len(s) > 63 {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	raw := q.String("domain", "", true, 253)
	site := strings.TrimSpace(q.String("site", "", false, 63))
	resolver := q.String("resolver", "system", false, 300)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	domain, err := target.NormalizeHostname(strings.TrimSpace(raw))
	if err != nil || !strings.Contains(domain, ".") {
		httpapi.Error(w, r, httpapi.InvalidInput("domain", "enter an Active Directory DNS domain such as corp.example.com"))
		return
	}
	if !validSite(site) {
		httpapi.Error(w, r, httpapi.InvalidInput("site", "a site name is one label of letters, digits, - and _"))
		return
	}
	srv, err := dnsx.ResolveServer(r.Context(), t.deps.Target, resolver, dnsx.UDP, 0)
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	start := time.Now()
	res, err := Locate(r.Context(), srv, domain, site)
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start)), Target: &httpapi.TargetMeta{Input: raw, Used: srv.Addr.String()}})
}
