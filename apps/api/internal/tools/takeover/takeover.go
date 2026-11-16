// Package takeover implements DNS-27: subdomain takeover check. It follows each name's CNAME
// chain and flags targets that no longer exist, or that a hosting service reports as unclaimed.
package takeover

import (
	"context"
	"net/http"
	"slices"
	"strconv"
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
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/httpcore"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// Limits (docs/SECURITY.md).
const (
	MaxNames     = 50
	MaxChain     = 8
	MaxHTTPCheck = 20
	httpBody     = 64 << 10
)

// CommonNames are checked when no names are given.
var CommonNames = []string{
	"www", "blog", "shop", "store", "dev", "staging", "stage", "test", "beta", "demo", "docs", "help", "support", "status", "cdn", "assets", "static",
	"img", "images", "media", "app", "api", "portal", "admin", "mail", "email", "news", "events", "careers", "jobs", "go", "link", "info", "landing", "promo",
}

// Service is a hosting service and how an unclaimed name shows there.
type Service struct {
	Name     string
	Suffixes []string
	// Dangling: a CNAME to a target that does not exist can be claimed by registering the target.
	Dangling bool
	// Body markers the service shows for an unclaimed name (checked over HTTP).
	Body []string
}

// Services is a curated subset of community-maintained takeover fingerprints
// (github.com/EdOverflow/can-i-take-over-xyz), limited to services listed as vulnerable.
var Services = []Service{
	{Name: "AWS S3", Suffixes: []string{".s3.amazonaws.com", ".amazonaws.com"}, Body: []string{"NoSuchBucket", "The specified bucket does not exist"}},
	{Name: "AWS Elastic Beanstalk", Suffixes: []string{".elasticbeanstalk.com"}, Dangling: true},
	{Name: "Microsoft Azure", Suffixes: []string{".azurewebsites.net", ".cloudapp.net", ".cloudapp.azure.com", ".trafficmanager.net", ".blob.core.windows.net", ".azure-api.net", ".azurehdinsight.net", ".azureedge.net", ".azurecontainer.io", ".database.windows.net", ".azurecr.io", ".redis.cache.windows.net", ".servicebus.windows.net", ".search.windows.net", ".azurestaticapps.net"}, Dangling: true},
	{Name: "GitHub Pages", Suffixes: []string{".github.io"}, Body: []string{"There isn't a GitHub Pages site here."}},
	{Name: "Heroku", Suffixes: []string{".herokuapp.com", ".herokudns.com", ".herokussl.com"}, Dangling: true, Body: []string{"no-such-app.html", "No such app"}},
	{Name: "Shopify", Suffixes: []string{".myshopify.com"}, Body: []string{"Sorry, this shop is currently unavailable."}},
	{Name: "Fastly", Suffixes: []string{".fastly.net"}, Body: []string{"Fastly error: unknown domain"}},
	{Name: "Ghost", Suffixes: []string{".ghost.io"}, Body: []string{"The thing you were looking for is no longer here, or never was"}},
	{Name: "Pantheon", Suffixes: []string{".pantheonsite.io"}, Body: []string{"The gods are wise, but do not know of the site which you seek."}},
	{Name: "Read the Docs", Suffixes: []string{".readthedocs.io"}, Body: []string{"is unknown to Read the Docs"}},
	{Name: "Surge.sh", Suffixes: []string{".surge.sh"}, Body: []string{"project not found"}},
	{Name: "Bitbucket", Suffixes: []string{".bitbucket.io"}, Body: []string{"Repository not found"}},
	{Name: "Help Scout", Suffixes: []string{".helpscoutdocs.com"}, Body: []string{"No settings were found for this company:"}},
	{Name: "Zendesk", Suffixes: []string{".zendesk.com"}, Body: []string{"Help Center Closed"}},
	{Name: "UserVoice", Suffixes: []string{".uservoice.com"}, Body: []string{"This UserVoice subdomain is currently available!"}},
	{Name: "ngrok", Suffixes: []string{".ngrok.io", ".ngrok-free.app"}, Body: []string{"ngrok.io not found", "ERR_NGROK_3200"}},
	{Name: "WordPress.com", Suffixes: []string{".wordpress.com"}, Body: []string{"Do you want to register"}},
	{Name: "Tumblr", Suffixes: []string{"domains.tumblr.com"}, Body: []string{"Whatever you were looking for doesn't currently exist at this address"}},
	{Name: "Agile CRM", Suffixes: []string{".agilecrm.com"}, Body: []string{"Sorry, this page is no longer available."}},
	{Name: "Strikingly", Suffixes: []string{".s.strikinglydns.com"}, Body: []string{"PAGE NOT FOUND."}},
	{Name: "Unbounce", Suffixes: []string{".unbouncepages.com"}, Body: []string{"The requested URL was not found on this server."}},
	{Name: "Netlify", Suffixes: []string{".netlify.app", ".netlify.com"}, Dangling: true},
	{Name: "Vercel", Suffixes: []string{".vercel.app"}, Dangling: true},
}

// Tool is the subdomain takeover check.
type Tool struct {
	deps tools.Deps
	// HTTPPort overrides port 80 for the HTTP fingerprint check (tests).
	HTTPPort int
}

func (*Tool) ID() string                                { return "dns.takeover" }
func (*Tool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(60*time.Second)).Get("/dns/takeover", t.handle)
}

// Check is the verdict for one name.
type Check struct {
	Name  string   `json:"name"`
	Chain []string `json:"chain"`
	// Status: vulnerable, dangling, ok, no-cname, nxdomain or error.
	Status   string `json:"status"`
	Service  string `json:"service"`
	Evidence string `json:"evidence"`
}

// Result is the response payload.
type Result struct {
	Domain   string  `json:"domain"`
	Resolver string  `json:"resolver"`
	Common   bool    `json:"common"`
	Checks   []Check `json:"checks"`
}

func serviceFor(host string) *Service {
	for i := range Services {
		for _, s := range Services[i].Suffixes {
			if strings.HasSuffix(host, s) || host == strings.TrimPrefix(s, ".") {
				return &Services[i]
			}
		}
	}
	return nil
}

// follow returns the CNAME chain of name and the rcode of the final target's A lookup.
func follow(ctx context.Context, srv dnsx.Server, name string) ([]string, int, error) {
	chain := []string{}
	cur := name
	for range MaxChain {
		m, _, err := dnsx.Query(ctx, srv, cur, dns.TypeCNAME)
		if err != nil {
			return chain, 0, err //nolint:wrapcheck // mapped by caller
		}
		if m.Rcode == dns.RcodeNameError && len(chain) == 0 {
			return chain, dns.RcodeNameError, nil
		}
		next := ""
		for _, rr := range m.Answer {
			if c, ok := rr.(*dns.CNAME); ok && strings.EqualFold(strings.TrimSuffix(c.Hdr.Name, "."), cur) {
				next = strings.ToLower(strings.TrimSuffix(c.Target, "."))
			}
		}
		if next == "" || slices.Contains(chain, next) {
			break
		}
		chain = append(chain, next)
		cur = next
	}
	if len(chain) == 0 {
		return chain, dns.RcodeSuccess, nil
	}
	m, _, err := dnsx.Query(ctx, srv, cur, dns.TypeA)
	if err != nil {
		return chain, 0, err //nolint:wrapcheck // mapped by caller
	}
	return chain, m.Rcode, nil
}

func (t *Tool) fetch(ctx context.Context, name string) string {
	u := "http://" + name + "/"
	if t.HTTPPort != 0 {
		u = "http://" + name + ":" + strconv.Itoa(t.HTTPPort) + "/"
	}
	hops, err := (&httpcore.Client{Resolver: t.deps.Target}).Do(ctx, httpcore.Request{
		URL: u, Method: http.MethodGet, FollowRedirects: true, MaxRedirects: 3, Insecure: true,
		MaxBody: httpBody, PreviewBytes: httpBody, Timeout: 8 * time.Second, UserAgent: t.deps.UserAgent,
	})
	if err != nil || len(hops) == 0 {
		return ""
	}
	return hops[len(hops)-1].BodyPreview
}

func (t *Tool) check(ctx context.Context, srv dnsx.Server, name string, httpSlots chan struct{}) Check {
	c := Check{Name: name, Chain: []string{}}
	chain, rcode, err := follow(ctx, srv, name)
	c.Chain = chain
	switch {
	case err != nil:
		c.Status = "error"
		return c
	case rcode == dns.RcodeNameError && len(chain) == 0:
		c.Status = "nxdomain"
		return c
	case len(chain) == 0:
		c.Status = "no-cname"
		return c
	}
	final := chain[len(chain)-1]
	svc := serviceFor(final)
	if svc != nil {
		c.Service = svc.Name
	}
	if rcode == dns.RcodeNameError {
		c.Status, c.Evidence = "dangling", final+" does not exist (NXDOMAIN)"
		if svc != nil && svc.Dangling {
			c.Status = "vulnerable"
		}
		return c
	}
	c.Status = "ok"
	if svc == nil || len(svc.Body) == 0 {
		return c
	}
	select {
	case httpSlots <- struct{}{}:
	default:
		c.Evidence = "http-skipped"
		return c
	}
	body := t.fetch(ctx, name)
	for _, m := range svc.Body {
		if body != "" && strings.Contains(body, m) {
			c.Status, c.Evidence = "vulnerable", `page says "`+m+`"`
			break
		}
	}
	return c
}

// Run checks every name, a few at a time.
func (t *Tool) Run(ctx context.Context, srv dnsx.Server, names []string) []Check {
	out := make([]Check, len(names))
	sem := make(chan struct{}, 5)
	httpSlots := make(chan struct{}, MaxHTTPCheck) // filled, never drained: a budget
	var wg sync.WaitGroup
	for i, n := range names {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = t.check(ctx, srv, n, httpSlots)
		})
	}
	wg.Wait()
	return out
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	raw := q.String("domain", "", true, 253)
	list := q.String("names", "", false, 8192)
	resolver := q.String("resolver", "system", false, 300)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	domain, err := target.NormalizeHostname(strings.TrimSpace(raw))
	if err != nil || !strings.Contains(domain, ".") {
		httpapi.Error(w, r, httpapi.InvalidInput("domain", "enter a domain such as example.com"))
		return
	}
	res := Result{Domain: domain, Checks: []Check{}}
	labels := strings.FieldsFunc(list, func(c rune) bool { return c == ',' || c == '\n' || c == ' ' || c == ';' })
	if len(labels) == 0 {
		labels, res.Common = CommonNames, true
	}
	names := []string{}
	for _, l := range labels {
		n, err := target.NormalizeHostname(strings.TrimSpace(l))
		if err != nil {
			httpapi.Error(w, r, httpapi.InvalidInput("names", "not a valid name: "+l))
			return
		}
		if n != domain && !strings.HasSuffix(n, "."+domain) {
			n += "." + domain
		}
		if !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	if len(names) > MaxNames {
		httpapi.Error(w, r, httpapi.InvalidInput("names", "at most 50 names per check"))
		return
	}
	srv, err := dnsx.ResolveServer(r.Context(), t.deps.Target, resolver, dnsx.UDP, 0)
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	res.Resolver = srv.Label
	start := time.Now()
	res.Checks = t.Run(r.Context(), srv, names)
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start)), Target: &httpapi.TargetMeta{Input: raw, Used: srv.Addr.String()}})
}
