package web

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/httpcore"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// OriginTool is WEB-24: the same request through the CDN and directly to an origin address.
type OriginTool struct{ deps tools.Deps }

func (*OriginTool) ID() string                                { return "web.origin" }
func (*OriginTool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *OriginTool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(40*time.Second)).Get("/http/origin-compare", t.handle)
}

// compareHeaders are shown side by side.
var compareHeaders = []string{"Server", "Content-Type", "Content-Length", "Content-Encoding", "Cache-Control", "ETag", "Last-Modified", "Location", "Vary", "Strict-Transport-Security", "Content-Security-Policy", "X-Frame-Options", "Access-Control-Allow-Origin"}

// cdnHeaders typically appear only on the CDN side.
var cdnHeaders = []string{"Age", "Via", "X-Cache", "X-Cache-Hits", "CF-Ray", "CF-Cache-Status", "X-Served-By", "X-Amz-Cf-Id", "X-Amz-Cf-Pop", "Akamai-Grn", "X-Azure-Ref", "X-Fastly-Request-ID", "X-Vercel-Cache", "X-Nf-Request-Id"}

// Side is one of the two responses.
type Side struct {
	Address    string            `json:"address"`
	Status     int               `json:"status"`
	Headers    map[string]string `json:"headers"`
	CDNHeaders map[string]string `json:"cdnHeaders"`
	Cookies    []string          `json:"cookies"`
	BodyBytes  int               `json:"bodyBytes"`
	BodyHash   string            `json:"bodyHash"`
	TLS        *httpcore.TLSInfo `json:"tls,omitempty"`
	TotalMs    float64           `json:"totalMs"`
	TTFBMs     float64           `json:"ttfbMs"`
	Error      string            `json:"error"`
}

// Diff is one compared field.
type Diff struct {
	Field  string `json:"field"`
	CDN    string `json:"cdn"`
	Origin string `json:"origin"`
	Same   bool   `json:"same"`
}

// OriginResult is the response payload.
type OriginResult struct {
	URL      string   `json:"url"`
	CDN      Side     `json:"cdn"`
	Origin   Side     `json:"origin"`
	Diffs    []Diff   `json:"diffs"`
	Findings []string `json:"findings"`
}

func side(hops []httpcore.Hop, err error) Side {
	s := Side{Headers: map[string]string{}, CDNHeaders: map[string]string{}, Cookies: []string{}}
	if err != nil || len(hops) == 0 {
		s.Error = "unreachable"
		if err != nil {
			if e, ok := err.(interface{ APIMessage() string }); ok { //nolint:errorlint // message only
				s.Error = e.APIMessage()
			}
		}
		return s
	}
	h := hops[0]
	s.Address, s.Status, s.BodyBytes, s.TLS = h.RemoteAddr, h.Status, h.BodyBytes, h.TLS
	s.TotalMs, s.TTFBMs = h.Timing.TotalMs, h.Timing.TTFBMs
	sum := sha256.Sum256(h.Raw)
	s.BodyHash = hex.EncodeToString(sum[:8])
	for _, name := range compareHeaders {
		if v, ok := header(h, name); ok {
			s.Headers[name] = v
		}
	}
	for _, name := range cdnHeaders {
		if v, ok := header(h, name); ok {
			s.CDNHeaders[name] = v
		}
	}
	for _, c := range headers(h, "Set-Cookie") {
		name, _, _ := strings.Cut(c, "=")
		if !slices.Contains(s.Cookies, name) {
			s.Cookies = append(s.Cookies, name)
		}
	}
	return s
}

// Compare builds the diff list and findings.
func Compare(cdn, origin Side) ([]Diff, []string) {
	diffs := []Diff{}
	add := func(field, a, b string) {
		if a != "" || b != "" {
			diffs = append(diffs, Diff{Field: field, CDN: a, Origin: b, Same: a == b})
		}
	}
	itoa := func(n int) string {
		if n == 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	add("status", itoa(cdn.Status), itoa(origin.Status))
	for _, h := range compareHeaders {
		add(h, cdn.Headers[h], origin.Headers[h])
	}
	add("body (SHA-256 prefix)", cdn.BodyHash, origin.BodyHash)
	add("body bytes", itoa(cdn.BodyBytes), itoa(origin.BodyBytes))
	add("cookies", strings.Join(cdn.Cookies, ", "), strings.Join(origin.Cookies, ", "))
	if cdn.TLS != nil || origin.TLS != nil {
		cert := func(s Side) string {
			if s.TLS == nil {
				return ""
			}
			return s.TLS.Subject + " / " + s.TLS.Issuer
		}
		add("certificate", cert(cdn), cert(origin))
	}
	findings := []string{}
	switch {
	case origin.Error != "":
		findings = append(findings, "origin-unreachable")
	case origin.Status > 0 && origin.Status < 400:
		findings = append(findings, "origin-exposed")
	}
	if origin.Error == "" && origin.TLS != nil && origin.TLS.VerifyError != "" {
		findings = append(findings, "origin-cert-invalid")
	}
	if cdn.Error == "" && origin.Error == "" && cdn.Status == origin.Status && cdn.BodyHash != origin.BodyHash {
		findings = append(findings, "content-differs")
	}
	if len(cdn.CDNHeaders) == 0 && cdn.Error == "" {
		findings = append(findings, "no-cdn-headers")
	}
	return diffs, findings
}

func (t *OriginTool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	raw := q.String("url", "", true, 2048)
	originRaw := q.String("origin", "", true, 64)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	origin, err := netip.ParseAddr(strings.Trim(strings.TrimSpace(originRaw), "[]"))
	if err != nil || origin.Zone() != "" {
		httpapi.Error(w, r, httpapi.InvalidInput("origin", "enter the origin server's IP address"))
		return
	}
	// Check the origin against the target policy before sending anything.
	if _, err := t.deps.Target.Resolve(r.Context(), origin.String(), target.Opts{Field: "origin"}); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	base := httpcore.Request{URL: raw, Method: http.MethodGet, KeepRaw: true}
	type out struct {
		hops []httpcore.Hop
		err  error
	}
	ctx := r.Context()
	cdnCh, originCh := make(chan out, 1), make(chan out, 1)
	go func() {
		h, e := fetch(ctx, t.deps, base)
		cdnCh <- out{h, e}
	}()
	go func() {
		req := base
		req.ConnectTo = origin.Unmap()
		h, e := fetch(ctx, t.deps, req)
		originCh <- out{h, e}
	}()
	c, o := <-cdnCh, <-originCh
	if c.err != nil && len(c.hops) == 0 {
		var te interface{ APICode() string }
		if errors.As(c.err, &te) && strings.HasPrefix(te.APICode(), "TARGET_") {
			httpapi.Error(w, r, c.err)
			return
		}
	}
	if o.err != nil {
		var te interface{ APICode() string }
		if errors.As(o.err, &te) && strings.HasPrefix(te.APICode(), "TARGET_") {
			httpapi.Error(w, r, o.err)
			return
		}
	}
	res := OriginResult{URL: raw, CDN: side(c.hops, c.err), Origin: side(o.hops, o.err)}
	res.Diffs, res.Findings = Compare(res.CDN, res.Origin)
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", Target: &httpapi.TargetMeta{Input: raw, Used: origin.String()}})
}
