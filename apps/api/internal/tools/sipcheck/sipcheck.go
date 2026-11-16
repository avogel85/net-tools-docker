// Package sipcheck implements CON-21: SIP server discovery (NAPTR/SRV) and an OPTIONS ping over
// UDP, TCP and TLS.
package sipcheck

import (
	"bufio"
	"cmp"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/miekg/dns"

	"github.com/digibaro/net-tools-docker/apps/api/internal/dnsx"
	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tlsx"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnslookup"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// Limits.
const (
	PerAttempt = 4 * time.Second
	MaxReply   = 16 << 10
)

// Tool is the SIP server check.
type Tool struct {
	deps tools.Deps
	// Resolver overrides the system resolver for NAPTR/SRV discovery (tests).
	Resolver string
}

func (*Tool) ID() string                                { return "con.sip" }
func (*Tool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(40*time.Second)).Get("/sip/check", t.handle)
}

// SRV is one discovered SIP server.
type SRV struct {
	Service  string `json:"service"`
	Priority uint16 `json:"priority"`
	Weight   uint16 `json:"weight"`
	Port     uint16 `json:"port"`
	Target   string `json:"target"`
}

// Probe is the OPTIONS result for one transport.
type Probe struct {
	Transport string     `json:"transport"`
	Address   string     `json:"address"`
	Reachable bool       `json:"reachable"`
	Status    int        `json:"status"`
	Reason    string     `json:"reason"`
	Server    string     `json:"server"`
	Allow     []string   `json:"allow"`
	RTTMs     float64    `json:"rttMs"`
	Cert      *tlsx.Cert `json:"cert,omitempty"`
	CertError string     `json:"certError"`
	Error     string     `json:"error"`
}

// Result is the response payload.
type Result struct {
	Host   string   `json:"host"`
	NAPTR  []string `json:"naptr"`
	SRV    []SRV    `json:"srv"`
	Probes []Probe  `json:"probes"`
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// optionsRequest builds a minimal RFC 3261 OPTIONS request.
func optionsRequest(host, transport string, local net.Addr) string {
	via := strings.ToUpper(transport)
	scheme := "sip"
	if transport == "tls" {
		scheme = "sips"
	}
	return fmt.Sprintf("OPTIONS %[1]s:%[2]s SIP/2.0\r\n"+
		"Via: SIP/2.0/%[3]s %[4]s;branch=z9hG4bK%[5]s;rport\r\n"+
		"Max-Forwards: 70\r\n"+
		"From: <sip:nettools@nettools.invalid>;tag=%[6]s\r\n"+
		"To: <%[1]s:%[2]s>\r\n"+
		"Call-ID: %[7]s@nettools.invalid\r\n"+
		"CSeq: 1 OPTIONS\r\n"+
		"Contact: <sip:nettools@%[4]s>\r\n"+
		"Accept: application/sdp\r\n"+
		"User-Agent: NET-TOOLS\r\n"+
		"Content-Length: 0\r\n\r\n", scheme, host, via, local.String(), randHex(8), randHex(4), randHex(8))
}

// parseResponse reads the status line and the headers of interest.
func parseResponse(raw string, p *Probe) error {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	f := strings.SplitN(lines[0], " ", 3)
	if len(f) < 2 || !strings.HasPrefix(f[0], "SIP/2.0") {
		return errors.New("not a SIP response")
	}
	code, err := strconv.Atoi(f[1])
	if err != nil {
		return errors.New("not a SIP response")
	}
	p.Status = code
	if len(f) == 3 {
		p.Reason = strings.TrimSpace(f[2])
	}
	for _, l := range lines[1:] {
		if l == "" {
			break
		}
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "server", "user-agent":
			if p.Server == "" {
				p.Server = v
			}
		case "allow":
			for _, m := range strings.Split(v, ",") {
				if m = strings.TrimSpace(m); m != "" && len(p.Allow) < 20 {
					p.Allow = append(p.Allow, m)
				}
			}
		}
	}
	return nil
}

func probeUDP(ctx context.Context, host, addr string) Probe {
	p := Probe{Transport: "udp", Address: addr, Allow: []string{}}
	var d net.Dialer
	c, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		p.Error = "dial-failed"
		return p
	}
	defer c.Close()
	req := optionsRequest(host, "udp", c.LocalAddr())
	buf := make([]byte, MaxReply)
	for attempt := range 2 { // one retransmission, as SIP over UDP does
		start := time.Now()
		_ = c.SetDeadline(time.Now().Add(PerAttempt))
		if _, err := c.Write([]byte(req)); err != nil {
			p.Error = "send-failed"
			return p
		}
		n, err := c.Read(buf)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() && attempt == 0 && ctx.Err() == nil {
				continue
			}
			p.Error = "no-reply"
			return p
		}
		p.RTTMs = float64(time.Since(start).Microseconds()) / 1000
		p.Reachable = true
		if err := parseResponse(string(buf[:n]), &p); err != nil {
			p.Error = "not-sip"
		}
		return p
	}
	p.Error = "no-reply"
	return p
}

func probeStream(ctx context.Context, host, sni, addr string, useTLS bool) Probe {
	p := Probe{Transport: "tcp", Address: addr, Allow: []string{}}
	if useTLS {
		p.Transport = "tls"
	}
	ctx, cancel := context.WithTimeout(ctx, 2*PerAttempt)
	defer cancel()
	var d net.Dialer
	raw, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		p.Error = "refused"
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			p.Error = "timeout"
		}
		return p
	}
	defer raw.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(dl)
	}
	conn := raw
	if useTLS {
		tc := tls.Client(raw, &tls.Config{ServerName: sni, InsecureSkipVerify: true}) //nolint:gosec // verified below and reported
		if err := tc.HandshakeContext(ctx); err != nil {
			p.Error = "tls-failed"
			return p
		}
		st := tc.ConnectionState()
		if len(st.PeerCertificates) > 0 {
			c := tlsx.Summarize(st.PeerCertificates[0], time.Now())
			p.Cert = &c
			if err := tlsx.Verify(st.PeerCertificates, sni, time.Now()); err != nil {
				p.CertError = err.Error()
			}
		}
		conn = tc
	}
	start := time.Now()
	if _, err := io.WriteString(conn, optionsRequest(host, p.Transport, raw.LocalAddr())); err != nil {
		p.Error = "send-failed"
		return p
	}
	r := bufio.NewReader(io.LimitReader(conn, MaxReply))
	var sb strings.Builder
	for {
		line, err := r.ReadString('\n')
		sb.WriteString(line)
		if err != nil || line == "\r\n" || line == "\n" {
			break
		}
	}
	if sb.Len() == 0 {
		p.Error = "no-reply"
		return p
	}
	p.RTTMs = float64(time.Since(start).Microseconds()) / 1000
	p.Reachable = true
	if err := parseResponse(sb.String(), &p); err != nil {
		p.Error = "not-sip"
	}
	return p
}

func discover(ctx context.Context, srv dnsx.Server, domain string, res *Result) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Go(func() {
		m, _, err := dnsx.Query(ctx, srv, domain, dns.TypeNAPTR)
		if err != nil {
			return
		}
		for _, rr := range m.Answer {
			if n, ok := rr.(*dns.NAPTR); ok && strings.HasPrefix(strings.ToUpper(n.Service), "SIP") {
				mu.Lock()
				res.NAPTR = append(res.NAPTR, fmt.Sprintf("%d %d %q %s", n.Order, n.Preference, n.Service, strings.TrimSuffix(n.Replacement, ".")))
				mu.Unlock()
			}
		}
	})
	for _, svc := range []string{"_sip._udp.", "_sip._tcp.", "_sips._tcp."} {
		wg.Go(func() {
			m, _, err := dnsx.Query(ctx, srv, svc+domain, dns.TypeSRV)
			if err != nil {
				return
			}
			for _, rr := range m.Answer {
				if s, ok := rr.(*dns.SRV); ok {
					mu.Lock()
					res.SRV = append(res.SRV, SRV{Service: strings.TrimSuffix(svc, "."), Priority: s.Priority, Weight: s.Weight, Port: s.Port, Target: strings.TrimSuffix(strings.ToLower(s.Target), ".")})
					mu.Unlock()
				}
			}
		})
	}
	wg.Wait()
	slices.SortFunc(res.SRV, func(a, b SRV) int {
		return strings.Compare(a.Service, b.Service)*1000 + int(a.Priority) - int(b.Priority)
	})
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	host := q.String("host", "", true, 260)
	transport := q.Enum("transport", "all", "all", "udp", "tcp", "tls")
	port := q.Int("port", 0, 0, 65535)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	ctx := r.Context()
	res := Result{NAPTR: []string{}, SRV: []SRV{}, Probes: []Probe{}}
	// Discovery is only meaningful for a domain name.
	if name, err := target.NormalizeHostname(host); err == nil && strings.Contains(name, ".") {
		if srv, err := dnsx.ResolveServer(ctx, t.deps.Target, cmp.Or(t.Resolver, "system"), dnsx.UDP, 0); err == nil {
			discover(ctx, srv, name, &res)
		}
	}
	// Probe the first SRV target per transport if any, else the host itself.
	probeHost := host
	if len(res.SRV) > 0 && port == 0 {
		probeHost = res.SRV[0].Target
	}
	tgt, err := t.deps.Target.Resolve(ctx, probeHost, target.Opts{AllowPort: true, Field: "host"})
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	res.Host = tgt.Host
	sni := tgt.Host
	if tgt.IsIP {
		sni = ""
	}
	portFor := func(tr string) int {
		switch {
		case port != 0:
			return port
		case tgt.Port != 0:
			return tgt.Port
		}
		svc := "_sip._" + tr
		if tr == "tls" {
			svc = "_sips._tcp"
		}
		for _, s := range res.SRV {
			if s.Service == svc && s.Target == tgt.Host {
				return int(s.Port)
			}
		}
		if tr == "tls" {
			return 5061
		}
		return 5060
	}
	trs := []string{"udp", "tcp", "tls"}
	if transport != "all" {
		trs = []string{transport}
	}
	start := time.Now()
	res.Probes = make([]Probe, len(trs))
	var wg sync.WaitGroup
	for i, tr := range trs {
		wg.Go(func() {
			addr := tgt.AddrPort(portFor(tr))
			if tr == "udp" {
				res.Probes[i] = probeUDP(ctx, tgt.Host, addr)
			} else {
				res.Probes[i] = probeStream(ctx, tgt.Host, sni, addr, tr == "tls")
			}
		})
	}
	wg.Wait()
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start)), Target: &httpapi.TargetMeta{Input: host, Used: tgt.IP.String()}})
}
