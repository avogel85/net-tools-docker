// Package ikeprobe implements CON-20: an IPsec/IKE probe (IKEv2 IKE_SA_INIT and IKEv1 Main Mode
// first message). No authentication is attempted.
package ikeprobe

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnslookup"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// PerAttempt bounds one UDP request; each is sent at most twice. Var for tests.
var PerAttempt = 3 * time.Second

// Tool is the IKE probe.
type Tool struct{ deps tools.Deps }

func (*Tool) ID() string                                { return "con.ike" }
func (*Tool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(30*time.Second)).Get("/ike/probe", t.handle)
}

// V2Result is the IKEv2 outcome.
type V2Result struct {
	Answered      bool     `json:"answered"`
	Accepted      bool     `json:"accepted"`
	Encryption    string   `json:"encryption"`
	PRF           string   `json:"prf"`
	Integrity     string   `json:"integrity"`
	Group         string   `json:"group"`
	GroupWeak     bool     `json:"groupWeak"`
	NoProposal    bool     `json:"noProposal"`
	Cookie        bool     `json:"cookie"`
	Fragmentation bool     `json:"fragmentation"`
	SigHash       bool     `json:"sigHash"`
	VendorIDs     []string `json:"vendorIds"`
}

// V1Result is the IKEv1 outcome.
type V1Result struct {
	Answered   bool     `json:"answered"`
	Accepted   bool     `json:"accepted"`
	Encryption string   `json:"encryption"`
	Hash       string   `json:"hash"`
	Group      string   `json:"group"`
	GroupWeak  bool     `json:"groupWeak"`
	NoProposal bool     `json:"noProposal"`
	VendorIDs  []string `json:"vendorIds"`
}

// Result is the response payload.
type Result struct {
	Host     string   `json:"host"`
	Address  string   `json:"address"`
	IKEv2    V2Result `json:"ikev2"`
	IKEv1    V1Result `json:"ikev1"`
	Findings []string `json:"findings"`
}

// exchange sends req (with the non-ESP marker on port 4500) and returns the first reply.
func exchange(ctx context.Context, addr string, natt bool, req []byte) ([]byte, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return nil, err //nolint:wrapcheck // summarised
	}
	defer c.Close()
	if natt {
		req = append([]byte{0, 0, 0, 0}, req...)
	}
	buf := make([]byte, 8192)
	for range 2 {
		_ = c.SetDeadline(time.Now().Add(PerAttempt))
		if _, err := c.Write(req); err != nil {
			return nil, err //nolint:wrapcheck // summarised
		}
		n, err := c.Read(buf)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() && ctx.Err() == nil {
				continue
			}
			return nil, err //nolint:wrapcheck // summarised
		}
		b := buf[:n]
		if natt && len(b) >= 4 && b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 0 {
			b = b[4:]
		}
		return slices.Clone(b), nil
	}
	return nil, errors.New("no reply")
}

func groupName(g uint16) string {
	if g == 0 {
		return ""
	}
	if n, ok := GroupNames[g]; ok {
		return n
	}
	return "#" + strconv.Itoa(int(g))
}

func probeV2(ctx context.Context, addr string, natt bool) V2Result {
	res := V2Result{VendorIDs: []string{}}
	group := offeredGroups[0]
	for range 2 { // second round when the responder asks for another group
		spi := make([]byte, 8)
		_, _ = rand.Read(spi)
		req, err := SAInit(spi, group)
		if err != nil {
			return res
		}
		b, err := exchange(ctx, addr, natt, req)
		if err != nil {
			return res
		}
		r, err := ParseV2(b, spi)
		if err != nil {
			return res
		}
		res.Answered = true
		res.VendorIDs = append(res.VendorIDs, r.VendorIDs...)
		res.Fragmentation = res.Fragmentation || r.Fragmentation
		res.SigHash = res.SigHash || r.SigHash
		res.Cookie = res.Cookie || r.Cookie
		if r.WantsGroup != 0 && r.WantsGroup != group {
			if _, err := keyShare(r.WantsGroup); err == nil {
				group = r.WantsGroup
				continue
			}
			res.Group, res.GroupWeak = groupName(r.WantsGroup), WeakGroups[r.WantsGroup]
			return res
		}
		res.NoProposal = r.NoProposal
		if r.Encryption != "" {
			res.Accepted = true
			res.Encryption, res.PRF, res.Integrity = r.Encryption, r.PRF, r.Integrity
			res.Group, res.GroupWeak = groupName(r.Group), WeakGroups[r.Group]
		}
		return res
	}
	return res
}

func probeV1(ctx context.Context, addr string, natt bool) V1Result {
	res := V1Result{VendorIDs: []string{}}
	cookie := make([]byte, 8)
	_, _ = rand.Read(cookie)
	b, err := exchange(ctx, addr, natt, MainMode(cookie))
	if err != nil {
		return res
	}
	r, err := ParseV1(b, cookie)
	if err != nil {
		return res
	}
	res.Answered, res.NoProposal = true, r.NoProposal
	res.VendorIDs = append(res.VendorIDs, r.VendorIDs...)
	if r.Encryption != "" {
		res.Accepted = true
		res.Encryption, res.Hash = r.Encryption, r.Hash
		res.Group, res.GroupWeak = groupName(r.Group), WeakGroups[r.Group]
	}
	return res
}

// Probe runs both versions against addr.
func Probe(ctx context.Context, addr string, natt bool) Result {
	res := Result{Address: addr, Findings: []string{}}
	res.IKEv2 = probeV2(ctx, addr, natt)
	res.IKEv1 = probeV1(ctx, addr, natt)
	switch {
	case !res.IKEv2.Answered && !res.IKEv1.Answered:
		res.Findings = append(res.Findings, "no-answer")
	case res.IKEv1.Accepted:
		res.Findings = append(res.Findings, "ikev1-enabled")
	}
	if res.IKEv1.Accepted && (res.IKEv1.Hash == "SHA1" || res.IKEv1.Hash == "MD5" || res.IKEv1.Encryption == "3DES" || res.IKEv1.Encryption == "DES") {
		res.Findings = append(res.Findings, "ikev1-weak")
	}
	if res.IKEv2.GroupWeak || res.IKEv1.GroupWeak {
		res.Findings = append(res.Findings, "weak-group")
	}
	if res.IKEv2.Accepted && res.IKEv2.Encryption == "3DES" {
		res.Findings = append(res.Findings, "weak-cipher")
	}
	return res
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	host := q.String("host", "", true, 260)
	port := q.Enum("port", "500", "500", "4500")
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	tgt, err := t.deps.Target.Resolve(r.Context(), host, target.Opts{Field: "host"})
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	p := 500
	if port == "4500" {
		p = 4500
	}
	start := time.Now()
	res := Probe(r.Context(), tgt.AddrPort(p), p == 4500)
	res.Host = tgt.Host
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start)), Target: &httpapi.TargetMeta{Input: host, Used: res.Address}})
}
