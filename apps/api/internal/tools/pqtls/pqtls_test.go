package pqtls

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

// server starts a TLS server that accepts only the given groups.
func server(t *testing.T, groups []tls.CurveID) string {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.TLS = &tls.Config{CurvePreferences: groups, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return u.Host
}

func TestProbe(t *testing.T) {
	ctx := t.Context()
	pq := Probe(ctx, server(t, []tls.CurveID{tls.X25519MLKEM768, tls.X25519}), "")
	if pq.Error != "" || pq.Negotiated != "X25519MLKEM768" || !pq.NegotiatedPQ || pq.Version != "TLS 1.3" {
		t.Fatalf("pq = %+v", pq)
	}
	got := map[string]bool{}
	for _, g := range pq.Groups {
		got[g.Name] = g.Supported
	}
	if !got["X25519MLKEM768"] || !got["X25519"] || got["P-384"] || got["SecP384r1MLKEM1024"] {
		t.Errorf("groups = %v", got)
	}

	classic := Probe(ctx, server(t, []tls.CurveID{tls.CurveP256}), "")
	if classic.NegotiatedPQ || classic.Negotiated != "P-256" {
		t.Errorf("classic = %+v", classic)
	}

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	_ = ln.Close()
	if r := Probe(ctx, addr, ""); r.Error != "refused" {
		t.Errorf("closed port = %+v", r)
	}
}

func TestHandler(t *testing.T) {
	addr := server(t, nil)
	env := tooltest.New(t, &Tool{}, tooltest.Options{AllowPrivate: true})
	var r Result
	env.Get("/api/v1/tls/pq?host="+url.QueryEscape(addr)).Must(t).Decode(t, &r)
	if r.Negotiated == "" || len(r.Groups) != len(Groups) {
		t.Errorf("result = %+v", r)
	}
	tooltest.New(t, &Tool{}, tooltest.Options{}).Get("/api/v1/tls/pq?host=10.0.0.1").ExpectError(t, "TARGET_PRIVATE_DENIED")
	tooltest.New(t, &Tool{}, tooltest.Options{}).Get("/api/v1/tls/pq?host=").ExpectError(t, "INVALID_INPUT")
}
