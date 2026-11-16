// Package acmecheck implements TLS-17: is a name ready for an ACME certificate (HTTP-01 / DNS-01)?
package acmecheck

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/miekg/dns"

	"github.com/digibaro/net-tools-docker/apps/api/internal/dnsx"
	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnscaa"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnslookup"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/httpcore"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// CAs and their CAA identifiers.
var CAs = map[string]string{"letsencrypt": "letsencrypt.org", "zerossl": "sectigo.com", "google": "pki.goog", "buypass": "buypass.com", "sslcom": "ssl.com"}

// Tool is the ACME readiness check.
type Tool struct {
	deps tools.Deps
	// HTTPPort overrides port 80 (tests).
	HTTPPort int
}

func (*Tool) ID() string                                { return "tls.acme" }
func (*Tool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(40*time.Second)).Get("/tls/acme", t.handle)
}

// Hop is one step of the HTTP-01 request.
type Hop struct {
	URL    string `json:"url"`
	Status int    `json:"status"`
	Error  string `json:"error"`
}

// Result is the response payload.
type Result struct {
	Name     string   `json:"name"`
	CA       string   `json:"ca"`
	Wildcard bool     `json:"wildcard"`
	Resolver string   `json:"resolver"`
	A        []string `json:"a"`
	AAAA     []string `json:"aaaa"`
	// CAA: where the relevant set was found and whether it allows the CA (for this name, or wildcards).
	CAAName    string   `json:"caaName"`
	CAAIssue   []string `json:"caaIssue"`
	CAAWild    []string `json:"caaWild"`
	CAAAllowed bool     `json:"caaAllowed"`
	// HTTP-01 probe of a random token (expect 404 from the web server).
	HTTPChecked bool   `json:"httpChecked"`
	HTTPHops    []Hop  `json:"httpHops"`
	HTTPResult  string `json:"httpResult"`
	// DNS-01
	ChallengeCNAME string   `json:"challengeCname"`
	ChallengeTXT   []string `json:"challengeTxt"`
	Problems       []string `json:"problems"`
	Warnings       []string `json:"warnings"`
}

func caaAllows(values []string, ca string) bool {
	if len(values) == 0 {
		return true
	}
	for _, v := range values {
		dom := strings.ToLower(strings.TrimSpace(strings.SplitN(v, ";", 2)[0]))
		if dom == ca {
			return true
		}
	}
	return false
}

func (t *Tool) caa(ctx context.Context, srv dnsx.Server, name string, res *Result) error {
	for _, n := range dnscaa.Climb(name) {
		m, _, err := dnsx.Query(ctx, srv, n, dns.TypeCAA)
		if err != nil {
			return err //nolint:wrapcheck // mapped by caller
		}
		found := false
		for _, rr := range m.Answer {
			c, ok := rr.(*dns.CAA)
			if !ok || !strings.EqualFold(strings.TrimSuffix(c.Hdr.Name, "."), n) {
				continue
			}
			found = true
			switch strings.ToLower(c.Tag) {
			case "issue":
				res.CAAIssue = append(res.CAAIssue, c.Value)
			case "issuewild":
				res.CAAWild = append(res.CAAWild, c.Value)
			}
		}
		if found {
			res.CAAName = n
			return nil
		}
		if m.Rcode == dns.RcodeServerFailure {
			res.Warnings = append(res.Warnings, "caa-servfail")
			return nil
		}
	}
	return nil
}

func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "nettools-check-" + hex.EncodeToString(b)
}

// probe requests a random challenge path the way ACME validators do: port 80, redirects followed
// only to http/https on ports 80/443.
func (t *Tool) probe(ctx context.Context, name string, res *Result) {
	res.HTTPChecked = true
	host := name
	if t.HTTPPort != 0 {
		host = name + ":" + strconv.Itoa(t.HTTPPort)
	}
	u := "http://" + host + "/.well-known/acme-challenge/" + randomToken()
	for range 10 {
		hops, err := (&httpcore.Client{Resolver: t.deps.Target}).Do(ctx, httpcore.Request{URL: u, Method: http.MethodGet, MaxBody: 4 << 10, Timeout: 10 * time.Second, Insecure: true, UserAgent: t.deps.UserAgent})
		if err != nil {
			msg := err.Error()
			if e, ok := err.(interface{ APIMessage() string }); ok {
				msg = e.APIMessage()
			}
			res.HTTPHops = append(res.HTTPHops, Hop{URL: u, Error: msg})
			res.HTTPResult = "unreachable"
			return
		}
		h := hops[0]
		res.HTTPHops = append(res.HTTPHops, Hop{URL: u, Status: h.Status, Error: h.Error})
		if h.Status < 300 || h.Status >= 400 || h.Location == "" {
			switch {
			case h.Status == http.StatusNotFound:
				res.HTTPResult = "ok"
			case h.Status >= 500:
				res.HTTPResult = "server-error"
			case h.Status == http.StatusOK:
				res.HTTPResult = "answers-200"
			default:
				res.HTTPResult = "status-" + strconv.Itoa(h.Status)
			}
			return
		}
		next, err := url.Parse(h.Location)
		if err != nil {
			res.HTTPResult = "bad-redirect"
			return
		}
		base, _ := url.Parse(u)
		next = base.ResolveReference(next)
		if (next.Scheme != "http" && next.Scheme != "https") || (next.Port() != "" && next.Port() != "80" && next.Port() != "443" && t.HTTPPort == 0) {
			res.HTTPHops = append(res.HTTPHops, Hop{URL: next.String(), Error: "redirect to a port other than 80/443"})
			res.HTTPResult = "bad-redirect"
			return
		}
		u = next.String()
	}
	res.HTTPResult = "too-many-redirects"
}

// Check runs every test for name.
func (t *Tool) Check(ctx context.Context, srv dnsx.Server, name, ca string, wildcard bool) (Result, error) {
	res := Result{Name: name, CA: ca, Wildcard: wildcard, Resolver: srv.Label, A: []string{}, AAAA: []string{}, CAAIssue: []string{}, CAAWild: []string{}, HTTPHops: []Hop{}, ChallengeTXT: []string{}, Problems: []string{}, Warnings: []string{}}
	addrs, err := dnsx.Addresses(ctx, srv, name)
	if err != nil {
		return res, dnslookup.Upstream(err)
	}
	for _, a := range addrs {
		if a.Is4() {
			res.A = append(res.A, a.String())
		} else {
			res.AAAA = append(res.AAAA, a.String())
		}
	}
	if err := t.caa(ctx, srv, name, &res); err != nil {
		return res, dnslookup.Upstream(err)
	}
	id := CAs[ca]
	if wildcard {
		vals := res.CAAWild
		if len(vals) == 0 {
			vals = res.CAAIssue // issuewild falls back to issue (RFC 8659 §4.3)
		}
		res.CAAAllowed = caaAllows(vals, id)
	} else {
		// Without an issue property any CA may issue for non-wildcard names (RFC 8659 §4.2).
		res.CAAAllowed = caaAllows(res.CAAIssue, id)
	}
	if !res.CAAAllowed {
		res.Problems = append(res.Problems, "caa-denies")
	}

	// DNS-01 delegation and leftovers.
	ch := "_acme-challenge." + name
	if m, _, err := dnsx.Query(ctx, srv, ch, dns.TypeTXT); err == nil {
		for _, rr := range m.Answer {
			switch v := rr.(type) {
			case *dns.CNAME:
				if res.ChallengeCNAME == "" {
					res.ChallengeCNAME = strings.TrimSuffix(strings.ToLower(v.Target), ".")
				}
			case *dns.TXT:
				res.ChallengeTXT = append(res.ChallengeTXT, strings.Join(v.Txt, ""))
			}
		}
	}
	if len(res.ChallengeTXT) > 0 {
		res.Warnings = append(res.Warnings, "stale-txt")
	}

	if wildcard {
		res.Warnings = append(res.Warnings, "wildcard-dns01")
		return res, nil
	}
	if len(addrs) == 0 {
		res.Problems = append(res.Problems, "no-address")
		return res, nil
	}
	t.probe(ctx, name, &res)
	switch res.HTTPResult {
	case "ok":
	case "answers-200", "status-401", "status-403":
		res.Warnings = append(res.Warnings, "http-"+res.HTTPResult)
	default:
		res.Problems = append(res.Problems, "http-"+res.HTTPResult)
	}
	if len(res.AAAA) > 0 {
		res.Warnings = append(res.Warnings, "ipv6-used")
	}
	slices.Sort(res.Warnings)
	return res, nil
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	raw := q.String("name", "", true, 253)
	ca := q.Enum("ca", "letsencrypt", "letsencrypt", "zerossl", "google", "buypass", "sslcom")
	resolver := q.String("resolver", "system", false, 300)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	raw = strings.TrimSpace(raw)
	wildcard := strings.HasPrefix(raw, "*.")
	name, err := target.NormalizeHostname(strings.TrimPrefix(raw, "*."))
	if err != nil || !strings.Contains(name, ".") {
		httpapi.Error(w, r, httpapi.InvalidInput("name", "enter the host name of the certificate, such as www.example.com"))
		return
	}
	srv, err := dnsx.ResolveServer(r.Context(), t.deps.Target, resolver, dnsx.UDP, 0)
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	start := time.Now()
	res, err := t.Check(r.Context(), srv, name, ca, wildcard)
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start)), Target: &httpapi.TargetMeta{Input: raw}})
}
