// Package pqtls implements TLS-18: does a server negotiate hybrid post-quantum key exchange?
package pqtls

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnslookup"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// PerHandshake bounds one TLS handshake.
const PerHandshake = 6 * time.Second

// Group is one key-exchange group that is probed.
type Group struct {
	ID   tls.CurveID
	Name string
	PQ   bool
}

// Groups are probed one at a time (only that group offered).
var Groups = []Group{
	{tls.X25519MLKEM768, "X25519MLKEM768", true},
	{tls.SecP256r1MLKEM768, "SecP256r1MLKEM768", true},
	{tls.SecP384r1MLKEM1024, "SecP384r1MLKEM1024", true},
	{tls.X25519, "X25519", false},
	{tls.CurveP256, "P-256", false},
	{tls.CurveP384, "P-384", false},
}

// GroupResult is the probe result for one group.
type GroupResult struct {
	Name      string `json:"name"`
	PQ        bool   `json:"pq"`
	Supported bool   `json:"supported"`
}

// Result is the response payload.
type Result struct {
	Host    string `json:"host"`
	Address string `json:"address"`
	// Negotiated is the group chosen when offering what a current browser offers.
	Negotiated   string        `json:"negotiated"`
	NegotiatedPQ bool          `json:"negotiatedPq"`
	Version      string        `json:"version"`
	Groups       []GroupResult `json:"groups"`
	Error        string        `json:"error"`
}

// Tool is the post-quantum TLS check.
type Tool struct{ deps tools.Deps }

func (*Tool) ID() string                                { return "tls.pq" }
func (*Tool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(40*time.Second)).Get("/tls/pq", t.handle)
}

func groupName(id tls.CurveID) (string, bool) {
	for _, g := range Groups {
		if g.ID == id {
			return g.Name, g.PQ
		}
	}
	if id == 0 {
		return "", false
	}
	return id.String(), false
}

func versionName(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	}
	return ""
}

func handshake(ctx context.Context, addr, sni string, groups []tls.CurveID) (tls.ConnectionState, error) {
	ctx, cancel := context.WithTimeout(ctx, PerHandshake)
	defer cancel()
	var d net.Dialer
	raw, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return tls.ConnectionState{}, err //nolint:wrapcheck // summarised for the client
	}
	defer raw.Close()
	cfg := &tls.Config{ServerName: sni, InsecureSkipVerify: true, CurvePreferences: groups} //nolint:gosec // capability probe, not trust
	c := tls.Client(raw, cfg)
	if err := c.HandshakeContext(ctx); err != nil {
		return tls.ConnectionState{}, err //nolint:wrapcheck // summarised for the client
	}
	return c.ConnectionState(), nil
}

// Probe runs the default handshake and one per group.
func Probe(ctx context.Context, addr, sni string) Result {
	res := Result{Address: addr, Groups: []GroupResult{}}
	st, err := handshake(ctx, addr, sni, nil) // Go defaults: hybrid PQ first, like current browsers
	if err != nil {
		res.Error = simplify(err)
		return res
	}
	res.Negotiated, res.NegotiatedPQ = groupName(st.CurveID)
	res.Version = versionName(st.Version)
	res.Groups = make([]GroupResult, len(Groups))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3)
	for i, g := range Groups {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			_, err := handshake(ctx, addr, sni, []tls.CurveID{g.ID})
			res.Groups[i] = GroupResult{Name: g.Name, PQ: g.PQ, Supported: err == nil}
		})
	}
	wg.Wait()
	return res
}

func simplify(err error) string {
	var ne net.Error
	switch {
	case errors.As(err, &ne) && ne.Timeout():
		return "timeout"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "refused"
	default:
		return "handshake-failed"
	}
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	host := q.String("host", "", true, 260)
	port := q.Int("port", 443, 1, 65535)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	tgt, err := t.deps.Target.Resolve(r.Context(), host, target.Opts{AllowPort: true, Field: "host"})
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	if tgt.Port != 0 {
		port = tgt.Port
	}
	sni := tgt.Host
	if tgt.IsIP {
		sni = ""
	}
	start := time.Now()
	res := Probe(r.Context(), tgt.AddrPort(port), sni)
	res.Host = tgt.Host
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start)), Target: &httpapi.TargetMeta{Input: host, Used: res.Address}})
}
