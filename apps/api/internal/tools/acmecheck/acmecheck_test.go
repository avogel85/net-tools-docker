package acmecheck

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/digibaro/net-tools-docker/apps/api/internal/dnsx/dnstest"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func TestACME(t *testing.T) {
	addr := dnstest.Zone(t,
		"www.example.test. 300 IN A 127.0.0.1",
		"example.test. 300 IN CAA 0 issue \"letsencrypt.org\"",
		"example.test. 300 IN CAA 0 issuewild \";\"",
		"_acme-challenge.www.example.test. 300 IN TXT \"old-token\"",
		"redir.example.test. 300 IN A 127.0.0.1",
		"broken.example.test. 300 IN A 127.0.0.1",
		"google.test. 300 IN A 127.0.0.1",
		"google.test. 300 IN CAA 0 issue \"pki.goog\"",
		"_acme-challenge.google.test. 300 IN CNAME acme.dns-provider.test.",
		"noaddr.example.test. 300 IN TXT \"x\"",
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.Host, "redir.") && !strings.HasPrefix(r.URL.Path, "/moved/"):
			http.Redirect(w, r, "/moved"+r.URL.Path, http.StatusMovedPermanently)
		case strings.HasPrefix(r.Host, "broken."):
			http.Error(w, "boom", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	env := tooltest.New(t, &Tool{HTTPPort: p}, tooltest.Options{AllowPrivate: true, Lookup: tooltest.StaticLookup("127.0.0.1")})
	q := "&resolver=" + url.QueryEscape(addr.String())
	get := func(name, ca string) Result {
		var r Result
		env.Get("/api/v1/tls/acme?name="+url.QueryEscape(name)+"&ca="+ca+q).Must(t).Decode(t, &r)
		return r
	}

	r := get("www.example.test", "letsencrypt")
	if !r.CAAAllowed || r.CAAName != "example.test" || r.HTTPResult != "ok" || len(r.Problems) != 0 || !slices.Equal(r.A, []string{"127.0.0.1"}) {
		t.Errorf("www = %+v", r)
	}
	if !slices.Contains(r.Warnings, "stale-txt") {
		t.Errorf("www warnings = %v", r.Warnings)
	}
	if r = get("www.example.test", "zerossl"); r.CAAAllowed || !slices.Contains(r.Problems, "caa-denies") {
		t.Errorf("zerossl = %+v", r)
	}
	if r = get("*.example.test", "letsencrypt"); r.CAAAllowed || !r.Wildcard || r.HTTPChecked || !slices.Contains(r.Warnings, "wildcard-dns01") {
		t.Errorf("wildcard = %+v", r)
	}
	if r = get("redir.example.test", "letsencrypt"); r.HTTPResult != "ok" || len(r.HTTPHops) != 2 {
		t.Errorf("redirect = %+v", r)
	}
	if r = get("broken.example.test", "letsencrypt"); r.HTTPResult != "server-error" || !slices.Contains(r.Problems, "http-server-error") {
		t.Errorf("broken = %+v", r)
	}
	if r = get("google.test", "google"); !r.CAAAllowed || r.ChallengeCNAME != "acme.dns-provider.test" {
		t.Errorf("google = %+v", r)
	}
	if r = get("noaddr.example.test", "letsencrypt"); !slices.Contains(r.Problems, "no-address") {
		t.Errorf("noaddr = %+v", r)
	}
}

func TestACMEValidation(t *testing.T) {
	env := tooltest.New(t, &Tool{}, tooltest.Options{})
	env.Get("/api/v1/tls/acme?name=nodot").ExpectError(t, "INVALID_INPUT")
	env.Get("/api/v1/tls/acme?name=example.com&ca=evilca").ExpectError(t, "INVALID_INPUT")
	env.Get("/api/v1/tls/acme?name=example.com&resolver=10.0.0.53").ExpectError(t, "TARGET_PRIVATE_DENIED")
}

func TestCAAAllows(t *testing.T) {
	if !caaAllows(nil, "letsencrypt.org") || !caaAllows([]string{"letsencrypt.org; accounturi=x"}, "letsencrypt.org") || caaAllows([]string{";"}, "letsencrypt.org") {
		t.Error("caaAllows")
	}
}
