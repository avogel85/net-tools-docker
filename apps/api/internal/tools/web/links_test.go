package web

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func TestLinksCrawl(t *testing.T) {
	var port string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "ext.test:"+port {
			if r.URL.Path == "/gone" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte("ok"))
			return
		}
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<a href="/about#team">About</a> <a href="/missing">x</a> <img src="/logo.png"> <img src="/broken.png">
<a href="http://ext.test:` + port + `/ok">ext</a> <a href="http://ext.test:` + port + `/gone">gone</a> <a href="mailto:a@b.c">m</a> <a href="/loop">loop</a>`))
		case "/about":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<a href="/">home</a> <a href="/deep">deep</a> <a href="/missing">again</a>`))
		case "/deep":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<a href="/deeper">deeper</a>`))
		case "/logo.png":
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			_, _ = w.Write([]byte("png"))
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	_, port, _ = net.SplitHostPort(srv.Listener.Addr().String())

	env := tooltest.New(t, &LinksTool{}, tooltest.Options{AllowPrivate: true, Lookup: tooltest.StaticLookup("127.0.0.1")})
	var r LinksResult
	env.Get("/api/v1/http/links?url="+url.QueryEscape("http://site.test:"+port+"/")).Must(t).Decode(t, &r)
	byURL := map[string]LinkCheck{}
	for _, l := range r.Links {
		byURL[l.URL] = l
	}
	base := "http://site.test:" + port
	want := map[string][2]any{
		base + "/about":                     {"page", false},
		base + "/missing":                   {"page", true},
		base + "/logo.png":                  {"asset", false}, // HEAD 405 → GET 200
		base + "/broken.png":                {"asset", true},
		"http://ext.test:" + port + "/ok":   {"external", false},
		"http://ext.test:" + port + "/gone": {"external", true},
		base + "/loop":                      {"page", true},
	}
	for u, w := range want {
		l, ok := byURL[u]
		if !ok || l.Kind != w[0] || l.Broken != w[1] {
			t.Errorf("%s = %+v (present %v), want %v", u, l, ok, w)
		}
	}
	if byURL[base+"/missing"].Refs != 2 || r.Broken != 5 || r.Links[0].Broken != true {
		t.Errorf("refs/broken: %+v broken=%d", byURL[base+"/missing"], r.Broken)
	}
	if _, ok := byURL[base+"/deeper"]; !ok {
		t.Error("depth-2 link not found")
	}
	for _, p := range r.Pages {
		if p == base+"/deeper" {
			t.Error("crawled beyond depth 2")
		}
	}

	var noExt LinksResult
	env.Get("/api/v1/http/links?external=false&url="+url.QueryEscape(base+"/")).Must(t).Decode(t, &noExt)
	for _, l := range noExt.Links {
		if l.Kind == "external" {
			t.Errorf("external link checked: %s", l.URL)
		}
	}
	tooltest.New(t, &LinksTool{}, tooltest.Options{}).Get("/api/v1/http/links?url="+url.QueryEscape("http://10.0.0.1/")).ExpectError(t, "TARGET_PRIVATE_DENIED")
}
