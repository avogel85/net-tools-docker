// Package webhook implements WEB-21: temporary request bins that capture incoming HTTP requests
// (method, path, headers, body) for debugging webhooks. Bins live in memory only.
package webhook

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/middleware"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
)

// Limits (docs/SECURITY.md).
const (
	BinTTL         = 15 * time.Minute
	MaxRequests    = 50
	MaxBody        = 64 << 10
	MaxHeaders     = 100
	MaxBins        = 200
	MaxBinsPerUser = 3
)

// Captured is one stored request.
type Captured struct {
	Time        string   `json:"time"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Query       string   `json:"query"`
	Headers     []Header `json:"headers"`
	Body        string   `json:"body"`
	BodyBytes   int      `json:"bodyBytes"`
	Truncated   bool     `json:"truncated"`
	Binary      bool     `json:"binary"`
	RemoteIP    string   `json:"remoteIp"`
	ContentType string   `json:"contentType"`
}

// Header is one request header.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type bin struct {
	id, token string
	owner     netip.Addr
	expires   time.Time
	requests  []Captured
	dropped   int
}

// Store holds the bins; shared by the management and capture tools.
type Store struct {
	mu   sync.Mutex
	bins map[string]*bin
	now  func() time.Time
}

// NewStore returns an empty store.
func NewStore() *Store { return &Store{bins: map[string]*bin{}, now: time.Now} }

func (s *Store) expire() {
	now := s.now()
	for id, b := range s.bins {
		if now.After(b.expires) {
			delete(s.bins, id)
		}
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Create makes a bin for owner.
func (s *Store) Create(owner netip.Addr) (BinInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	if len(s.bins) >= MaxBins {
		return BinInfo{}, httpapi.NewError(httpapi.CodeBusy, "too many request bins are active on this server; try again in a few minutes")
	}
	n := 0
	for _, b := range s.bins {
		if b.owner == owner {
			n++
		}
	}
	if n >= MaxBinsPerUser {
		return BinInfo{}, httpapi.NewError(httpapi.CodeBusy, "you already have 3 active request bins; delete one or wait until it expires")
	}
	b := &bin{id: randHex(12), token: randHex(16), owner: owner, expires: s.now().Add(BinTTL), requests: []Captured{}}
	s.bins[b.id] = b
	return BinInfo{ID: b.id, Token: b.token, Path: "/api/v1/hook/" + b.id, Expires: b.expires.UTC().Format(time.RFC3339)}, nil
}

// get returns the bin when token matches (constant time).
func (s *Store) get(id, token string) *bin {
	s.expire()
	b := s.bins[id]
	if b == nil || subtle.ConstantTimeCompare([]byte(b.token), []byte(token)) != 1 {
		return nil
	}
	return b
}

// Capture stores r in bin id; false when the bin does not exist.
func (s *Store) Capture(id string, r *http.Request) bool {
	body, _ := io.ReadAll(io.LimitReader(r.Body, MaxBody+1))
	c := Captured{
		Time: s.now().UTC().Format(time.RFC3339Nano), Method: r.Method, Query: r.URL.RawQuery,
		Headers: []Header{}, BodyBytes: len(body), ContentType: r.Header.Get("Content-Type"),
		RemoteIP: middleware.GetClientIP(r.Context()).String(),
	}
	c.Path = "/" + strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	if len(body) > MaxBody {
		body, c.Truncated, c.BodyBytes = body[:MaxBody], true, MaxBody
	}
	if utf8.Valid(body) {
		c.Body = string(body)
	} else {
		c.Binary = true
	}
	names := make([]string, 0, len(r.Header))
	for k := range r.Header {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		for _, v := range r.Header[k] {
			if len(c.Headers) < MaxHeaders {
				c.Headers = append(c.Headers, Header{Name: k, Value: v})
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	b := s.bins[id]
	if b == nil {
		return false
	}
	if len(b.requests) >= MaxRequests {
		b.dropped++
		return true
	}
	b.requests = append(b.requests, c)
	return true
}

// BinTool is WEB-21's management API: create, read and delete bins.
type BinTool struct {
	Store *Store
	deps  tools.Deps
}

func (*BinTool) ID() string                                { return "web.webhook" }
func (*BinTool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *BinTool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy)).Post("/webhook/bins", t.create)
	r.With(d.RateLimit(limits.Light)).Get("/webhook/bins/{id}", t.read)
	r.With(d.RateLimit(limits.Light)).Delete("/webhook/bins/{id}", t.remove)
}

// BinInfo is returned on create.
type BinInfo struct {
	ID      string `json:"id"`
	Token   string `json:"token"`
	Path    string `json:"path"`
	Expires string `json:"expires"`
}

// BinContent is returned on read.
type BinContent struct {
	ID       string     `json:"id"`
	Expires  string     `json:"expires"`
	Dropped  int        `json:"dropped"`
	Requests []Captured `json:"requests"`
}

func (t *BinTool) create(w http.ResponseWriter, r *http.Request) {
	b, err := t.Store.Create(middleware.GetClientIP(r.Context()))
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	httpapi.JSON(w, r, b, &httpapi.Meta{Tool: t.ID(), Vantage: "server"})
}

func (t *BinTool) read(w http.ResponseWriter, r *http.Request) {
	t.Store.mu.Lock()
	b := t.Store.get(chi.URLParam(r, "id"), r.Header.Get("X-Bin-Token"))
	var out BinContent
	if b != nil {
		out = BinContent{ID: b.id, Expires: b.expires.UTC().Format(time.RFC3339), Dropped: b.dropped, Requests: append([]Captured{}, b.requests...)}
	}
	t.Store.mu.Unlock()
	if b == nil {
		httpapi.Error(w, r, httpapi.NewError(httpapi.CodeNotFound, "this request bin does not exist or has expired"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpapi.JSON(w, r, out, &httpapi.Meta{Tool: t.ID(), Vantage: "server"})
}

func (t *BinTool) remove(w http.ResponseWriter, r *http.Request) {
	t.Store.mu.Lock()
	b := t.Store.get(chi.URLParam(r, "id"), r.Header.Get("X-Bin-Token"))
	if b != nil {
		delete(t.Store.bins, b.id)
	}
	t.Store.mu.Unlock()
	httpapi.JSON(w, r, map[string]bool{"deleted": b != nil}, &httpapi.Meta{Tool: t.ID(), Vantage: "server"})
}

// CaptureTool receives requests at /hook/{id}/… (any method). Separate from BinTool so access
// rules can allow anonymous senders while bin management needs a role.
type CaptureTool struct {
	Store *Store
	deps  tools.Deps
}

func (*CaptureTool) ID() string                                { return "web.hook" }
func (*CaptureTool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *CaptureTool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	h := http.HandlerFunc(t.capture)
	r.With(d.RateLimit(limits.Light)).Handle("/hook/{id}", h)
	r.With(d.RateLimit(limits.Light)).Handle("/hook/{id}/*", h)
}

func (t *CaptureTool) capture(w http.ResponseWriter, r *http.Request) {
	if !t.Store.Capture(chi.URLParam(r, "id"), r) {
		httpapi.Error(w, r, httpapi.NewError(httpapi.CodeNotFound, "no such request bin"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}` + "\n"))
}
