package takeover

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/digibaro/net-tools-docker/apps/api/internal/dnsx/dnstest"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func TestTakeover(t *testing.T) {
	addr := dnstest.Zone(t,
		"www.example.test. 300 IN CNAME gone.azurewebsites.net.",
		"old.example.test. 300 IN CNAME foo.unknown.test.",
		"pages.example.test. 300 IN CNAME me.github.io.",
		"fine.example.test. 300 IN CNAME ok.github.io.",
		"me.github.io. 300 IN A 127.0.0.1",
		"ok.github.io. 300 IN A 127.0.0.1",
		"plain.example.test. 300 IN A 192.0.2.1",
		"multi.example.test. 300 IN CNAME a.example.test.",
		"a.example.test. 300 IN CNAME app.herokudns.com.",
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Host, "pages.") {
			http.Error(w, "<h1>404</h1><p>There isn't a GitHub Pages site here.</p>", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("<h1>Welcome</h1>"))
	}))
	t.Cleanup(srv.Close)
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)

	env := tooltest.New(t, &Tool{HTTPPort: p}, tooltest.Options{AllowPrivate: true, Lookup: tooltest.StaticLookup("127.0.0.1")})
	var r Result
	env.Get("/api/v1/dns/takeover?domain=example.test&names=www,old,pages,fine,plain,missing,multi&resolver="+url.QueryEscape(addr.String())).Must(t).Decode(t, &r)
	got := map[string]Check{}
	for _, c := range r.Checks {
		got[strings.TrimSuffix(c.Name, ".example.test")] = c
	}
	want := map[string][2]string{
		"www": {"vulnerable", "Microsoft Azure"}, "old": {"dangling", ""}, "pages": {"vulnerable", "GitHub Pages"},
		"fine": {"ok", "GitHub Pages"}, "plain": {"no-cname", ""}, "missing": {"nxdomain", ""}, "multi": {"vulnerable", "Heroku"},
	}
	for k, w := range want {
		if got[k].Status != w[0] || got[k].Service != w[1] {
			t.Errorf("%s = %+v, want %v", k, got[k], w)
		}
	}
	if len(got["multi"].Chain) != 2 || r.Common {
		t.Errorf("chain %v common %v", got["multi"].Chain, r.Common)
	}
}

func TestTakeoverValidation(t *testing.T) {
	env := tooltest.New(t, &Tool{}, tooltest.Options{})
	env.Get("/api/v1/dns/takeover?domain=nodot").ExpectError(t, "INVALID_INPUT")
	env.Get("/api/v1/dns/takeover?domain=example.com&names=bad_label!").ExpectError(t, "INVALID_INPUT")
	many := strings.Repeat("a,", 51)
	for i := range 51 {
		many = strings.Replace(many, "a,", "n"+strconv.Itoa(i)+";", 1)
	}
	env.Get("/api/v1/dns/takeover?domain=example.com&names="+url.QueryEscape(many)).ExpectError(t, "INVALID_INPUT")
}

func TestServiceFor(t *testing.T) {
	if s := serviceFor("foo.github.io"); s == nil || s.Name != "GitHub Pages" {
		t.Errorf("github = %v", s)
	}
	if s := serviceFor("evilgithub.io"); s != nil {
		t.Errorf("evilgithub.io matched %v", s.Name)
	}
}
