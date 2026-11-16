package web

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func TestOriginCompare(t *testing.T) {
	// One server plays both roles: requests arriving on the "CDN" address get CDN headers. The test
	// resolves the host to 127.0.0.1 (CDN) and pins the origin to 127.0.0.2 on the same port.
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Skip(err)
	}
	srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		local := r.Context().Value(http.LocalAddrContextKey).(net.Addr).String()
		w.Header().Set("Server", "nginx")
		if host, _, _ := net.SplitHostPort(local); host == "127.0.0.1" {
			w.Header().Set("CF-Cache-Status", "HIT")
			w.Header().Set("Server", "cloudflare")
			_, _ = w.Write([]byte("cached page"))
			return
		}
		w.Header().Set("Set-Cookie", "session=1")
		_, _ = w.Write([]byte("fresh page"))
	})}}
	srv.Start()
	t.Cleanup(srv.Close)
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	env := tooltest.New(t, &OriginTool{}, tooltest.Options{AllowPrivate: true, Lookup: tooltest.StaticLookup("127.0.0.1")})
	var r OriginResult
	env.Get("/api/v1/http/origin-compare?origin=127.0.0.2&url="+url.QueryEscape("http://www.example.test:"+port+"/")).Must(t).Decode(t, &r)
	if r.CDN.Headers["Server"] != "cloudflare" || r.Origin.Headers["Server"] != "nginx" || r.CDN.CDNHeaders["CF-Cache-Status"] != "HIT" {
		t.Fatalf("sides = %+v / %+v", r.CDN, r.Origin)
	}
	for _, f := range []string{"origin-exposed", "content-differs"} {
		if !slices.Contains(r.Findings, f) {
			t.Errorf("findings %v miss %s", r.Findings, f)
		}
	}
	diffs := map[string]Diff{}
	for _, d := range r.Diffs {
		diffs[d.Field] = d
	}
	if diffs["Server"].Same || !diffs["status"].Same || diffs["cookies"].Origin != "session" {
		t.Errorf("diffs = %+v", diffs)
	}

	env.Get("/api/v1/http/origin-compare?origin=nope&url=http%3A%2F%2Fwww.example.test%2F").ExpectError(t, "INVALID_INPUT")
	tooltest.New(t, &OriginTool{}, tooltest.Options{Lookup: tooltest.StaticLookup("93.184.215.14")}).
		Get("/api/v1/http/origin-compare?origin=10.0.0.5&url=http%3A%2F%2Fwww.example.test%2F").ExpectError(t, "TARGET_PRIVATE_DENIED")
}
