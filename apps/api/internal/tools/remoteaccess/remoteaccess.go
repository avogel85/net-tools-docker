// Package remoteaccess implements CON-23: RDP (NLA, TLS certificate), VNC (RFB version, security
// types) and SSH (banner) checks. No credentials are sent and no session is opened.
package remoteaccess

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tlsx"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnslookup"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// PerService bounds one service check.
const PerService = 10 * time.Second

// RDP protocol flags (MS-RDPBCGR 2.2.1.1.1).
const (
	protoRDP     = 0x0
	protoSSL     = 0x1
	protoHybrid  = 0x2
	protoRDSTLS  = 0x4
	protoHybrEx  = 0x8
	negRsp       = 0x02
	negFailure   = 0x03
	hybridNeeded = 0x05 // HYBRID_REQUIRED_BY_SERVER
)

// DefaultPorts per service.
var DefaultPorts = map[string]int{"rdp": 3389, "vnc": 5900, "ssh": 22}

// Tool is the remote access check.
type Tool struct{ deps tools.Deps }

func (*Tool) ID() string                                { return "con.remoteaccess" }
func (*Tool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(40*time.Second)).Get("/remote-access/check", t.handle)
}

// Service is the result for one service.
type Service struct {
	Service string `json:"service"`
	Address string `json:"address"`
	Open    bool   `json:"open"`
	Error   string `json:"error"`
	// RDP
	Protocol    string     `json:"protocol"`
	NLARequired *bool      `json:"nlaRequired,omitempty"`
	Cert        *tlsx.Cert `json:"cert,omitempty"`
	CertError   string     `json:"certError"`
	// VNC
	RFBVersion    string   `json:"rfbVersion"`
	SecurityTypes []string `json:"securityTypes"`
	// SSH
	Banner   string   `json:"banner"`
	Findings []string `json:"findings"`
}

// Result is the response payload.
type Result struct {
	Host     string    `json:"host"`
	Services []Service `json:"services"`
}

func dial(ctx context.Context, addr string) (net.Conn, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err //nolint:wrapcheck // summarised
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = c.SetDeadline(dl)
	}
	return c, nil
}

func errCode(err error) string {
	var ne net.Error
	switch {
	case errors.As(err, &ne) && ne.Timeout():
		return "timeout"
	case strings.Contains(err.Error(), "refused"):
		return "refused"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "closed"
	}
	return "protocol-error"
}

// rdpRequest is a TPKT + X.224 Connection Request with an RDP Negotiation Request.
func rdpRequest(protocols uint32) []byte {
	b := []byte{0x03, 0x00, 0x00, 0x13, 0x0e, 0xe0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x08, 0x00, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(b[15:], protocols)
	return b
}

// rdpNegotiate returns the negotiation response type and its value (selected protocol or failure code).
func rdpNegotiate(c net.Conn, protocols uint32) (byte, uint32, error) {
	if _, err := c.Write(rdpRequest(protocols)); err != nil {
		return 0, 0, err //nolint:wrapcheck // mapped by caller
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(c, head); err != nil {
		return 0, 0, err //nolint:wrapcheck // mapped by caller
	}
	if head[0] != 0x03 {
		return 0, 0, errors.New("not RDP")
	}
	n := int(binary.BigEndian.Uint16(head[2:]))
	if n < 11 || n > 512 {
		return 0, 0, errors.New("not RDP")
	}
	body := make([]byte, n-4)
	if _, err := io.ReadFull(c, body); err != nil {
		return 0, 0, err //nolint:wrapcheck // mapped by caller
	}
	// body: X.224 CC (7 bytes: LI, 0xD0, dst, src, class) then the 8-byte negotiation structure.
	if len(body) < 7 || body[1]&0xf0 != 0xd0 {
		return 0, 0, errors.New("not RDP")
	}
	if len(body) < 15 {
		return negRsp, protoRDP, nil // old servers: no negotiation data = standard RDP security
	}
	return body[7], binary.LittleEndian.Uint32(body[11:15]), nil
}

func protocolName(p uint32) string {
	switch p {
	case protoRDP:
		return "rdp"
	case protoSSL:
		return "tls"
	case protoHybrid:
		return "credssp"
	case protoRDSTLS:
		return "rdstls"
	case protoHybrEx:
		return "credssp-ex"
	}
	return "unknown"
}

// CheckRDP negotiates twice: once offering everything (what is selected, the certificate) and once
// offering TLS only, to see whether the server insists on NLA (CredSSP).
func CheckRDP(ctx context.Context, addr, sni string) Service {
	s := Service{Service: "rdp", Address: addr, SecurityTypes: []string{}, Findings: []string{}}
	ctx, cancel := context.WithTimeout(ctx, PerService)
	defer cancel()
	c, err := dial(ctx, addr)
	if err != nil {
		s.Error = errCode(err)
		return s
	}
	defer c.Close()
	s.Open = true
	typ, val, err := rdpNegotiate(c, protoSSL|protoHybrid|protoHybrEx)
	if err != nil {
		s.Error = errCode(err)
		return s
	}
	if typ == negFailure {
		s.Error = "negotiation-failed"
		return s
	}
	s.Protocol = protocolName(val)
	if val == protoRDP {
		s.Findings = append(s.Findings, "rdp-legacy-security")
	}
	if val != protoRDP {
		tc := tls.Client(c, &tls.Config{ServerName: sni, InsecureSkipVerify: true}) //nolint:gosec // reported, not trusted
		if err := tc.HandshakeContext(ctx); err == nil {
			st := tc.ConnectionState()
			if len(st.PeerCertificates) > 0 {
				cert := tlsx.Summarize(st.PeerCertificates[0], time.Now())
				s.Cert = &cert
				if err := tlsx.Verify(st.PeerCertificates, sni, time.Now()); err != nil {
					s.CertError = err.Error()
				}
			}
		}
	}
	// Second connection, TLS only.
	c2, err := dial(ctx, addr)
	if err != nil {
		return s
	}
	defer c2.Close()
	typ2, val2, err := rdpNegotiate(c2, protoSSL)
	if err != nil {
		return s
	}
	required := typ2 == negFailure && val2 == hybridNeeded
	s.NLARequired = &required
	if !required {
		s.Findings = append(s.Findings, "rdp-nla-not-required")
	}
	return s
}

var vncTypes = map[byte]string{1: "none", 2: "vnc-auth", 5: "ra2", 6: "ra2ne", 16: "tight", 18: "tls", 19: "vencrypt", 20: "sasl", 21: "md5", 22: "xvp", 30: "apple-ard", 113: "mslogon2"}

// CheckVNC reads the RFB version and the offered security types.
func CheckVNC(ctx context.Context, addr string) Service {
	s := Service{Service: "vnc", Address: addr, SecurityTypes: []string{}, Findings: []string{}}
	ctx, cancel := context.WithTimeout(ctx, PerService)
	defer cancel()
	c, err := dial(ctx, addr)
	if err != nil {
		s.Error = errCode(err)
		return s
	}
	defer c.Close()
	s.Open = true
	ver := make([]byte, 12)
	if _, err := io.ReadFull(c, ver); err != nil || !strings.HasPrefix(string(ver), "RFB ") {
		s.Error = "protocol-error"
		return s
	}
	s.RFBVersion = strings.TrimSpace(string(ver[4:]))
	if s.RFBVersion < "003.007" {
		// RFB 3.3: the server picks one type (uint32) without a list.
		if _, err := c.Write(ver); err != nil {
			return s
		}
		b := make([]byte, 4)
		if _, err := io.ReadFull(c, b); err == nil {
			t := binary.BigEndian.Uint32(b)
			if t < 256 {
				if name, ok := vncTypes[byte(t)]; ok { //nolint:gosec // t < 256 checked
					s.SecurityTypes = append(s.SecurityTypes, name)
				}
			}
		}
	} else {
		if _, err := c.Write([]byte("RFB 003.008\n")); err != nil {
			return s
		}
		n := make([]byte, 1)
		if _, err := io.ReadFull(c, n); err != nil {
			s.Error = errCode(err)
			return s
		}
		types := make([]byte, int(n[0]))
		if _, err := io.ReadFull(c, types); err != nil {
			s.Error = errCode(err)
			return s
		}
		for _, t := range types {
			name, ok := vncTypes[t]
			if !ok {
				name = "type-" + strconv.Itoa(int(t))
			}
			s.SecurityTypes = append(s.SecurityTypes, name)
		}
	}
	if slices.Contains(s.SecurityTypes, "none") {
		s.Findings = append(s.Findings, "vnc-no-auth")
	} else if len(s.SecurityTypes) > 0 && !slices.ContainsFunc(s.SecurityTypes, func(x string) bool { return x == "tls" || x == "vencrypt" || x == "ra2" || x == "ra2ne" }) {
		s.Findings = append(s.Findings, "vnc-unencrypted")
	}
	return s
}

// CheckSSH reads the server's identification string.
func CheckSSH(ctx context.Context, addr string) Service {
	s := Service{Service: "ssh", Address: addr, SecurityTypes: []string{}, Findings: []string{}}
	ctx, cancel := context.WithTimeout(ctx, PerService)
	defer cancel()
	c, err := dial(ctx, addr)
	if err != nil {
		s.Error = errCode(err)
		return s
	}
	defer c.Close()
	s.Open = true
	r := bufio.NewReader(io.LimitReader(c, 4096))
	for range 10 { // servers may send other lines before the identification (RFC 4253 §4.2)
		line, err := r.ReadString('\n')
		if err != nil {
			s.Error = errCode(err)
			return s
		}
		if strings.HasPrefix(line, "SSH-") {
			s.Banner = strings.TrimSpace(line)
			if strings.HasPrefix(s.Banner, "SSH-1.") && !strings.HasPrefix(s.Banner, "SSH-1.99") {
				s.Findings = append(s.Findings, "ssh-v1")
			}
			return s
		}
	}
	s.Error = "protocol-error"
	return s
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	host := q.String("host", "", true, 260)
	service := q.Enum("service", "all", "all", "rdp", "vnc", "ssh")
	port := q.Int("port", 0, 0, 65535)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	tgt, err := t.deps.Target.Resolve(r.Context(), host, target.Opts{AllowPort: true, Field: "host"})
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	if port == 0 {
		port = tgt.Port
	}
	sni := tgt.Host
	if tgt.IsIP {
		sni = ""
	}
	services := []string{"rdp", "vnc", "ssh"}
	if service != "all" {
		services = []string{service}
	}
	ctx := r.Context()
	start := time.Now()
	res := Result{Host: tgt.Host, Services: make([]Service, len(services))}
	var wg sync.WaitGroup
	for i, svc := range services {
		wg.Go(func() {
			p := DefaultPorts[svc]
			if port != 0 && len(services) == 1 {
				p = port
			}
			addr := tgt.AddrPort(p)
			switch svc {
			case "rdp":
				res.Services[i] = CheckRDP(ctx, addr, sni)
			case "vnc":
				res.Services[i] = CheckVNC(ctx, addr)
			default:
				res.Services[i] = CheckSSH(ctx, addr)
			}
		})
	}
	wg.Wait()
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start)), Target: &httpapi.TargetMeta{Input: host, Used: tgt.IP.String()}})
}
