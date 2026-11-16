// Package ldapcheck implements CON-22: anonymous rootDSE read over LDAP (with StartTLS) and LDAPS.
// No bind is made and no credentials are sent.
package ldapcheck

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"

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
	PerEndpoint = 10 * time.Second
	MaxMessage  = 256 << 10
	maxValues   = 30
)

// StartTLSOID is the LDAP StartTLS extended operation (RFC 4511 §4.14).
const StartTLSOID = "1.3.6.1.4.1.1466.20037"

// RootDSEAttributes are requested from the rootDSE.
var RootDSEAttributes = []string{
	"namingContexts", "defaultNamingContext", "rootDomainNamingContext", "configurationNamingContext", "dnsHostName", "serverName",
	"supportedLDAPVersion", "supportedSASLMechanisms", "supportedExtension", "vendorName", "vendorVersion",
	"domainFunctionality", "forestFunctionality", "domainControllerFunctionality", "isGlobalCatalogReady", "isSynchronized", "currentTime",
}

// LDAP application tags.
const (
	tagSearchRequest  = asn1.Tag(0x63) // [APPLICATION 3] constructed
	tagSearchEntry    = asn1.Tag(0x64)
	tagSearchDone     = asn1.Tag(0x65)
	tagExtendedReq    = asn1.Tag(0x77) // [APPLICATION 23]
	tagExtendedResp   = asn1.Tag(0x78)
	tagFilterPresent  = asn1.Tag(0x87) // [7] primitive
	tagExtRequestName = asn1.Tag(0x80) // [0] primitive
)

// Tool is the LDAP check.
type Tool struct{ deps tools.Deps }

func (*Tool) ID() string                                { return "con.ldap" }
func (*Tool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(30*time.Second)).Get("/ldap/check", t.handle)
}

// Endpoint is the result for LDAP (389) or LDAPS (636).
type Endpoint struct {
	Mode      string `json:"mode"`
	Address   string `json:"address"`
	Reachable bool   `json:"reachable"`
	// StartTLS: supported (true) on plain LDAP; TLS info is then from the upgraded connection.
	StartTLS  bool                `json:"startTls"`
	TLS       *tlsx.Session       `json:"tls,omitempty"`
	RootDSE   map[string][]string `json:"rootDse"`
	Error     string              `json:"error"`
	ResultMsg string              `json:"resultMessage"`
}

// Result is the response payload.
type Result struct {
	Host      string     `json:"host"`
	Endpoints []Endpoint `json:"endpoints"`
	Warnings  []string   `json:"warnings"`
}

// --- BER encoding / decoding -------------------------------------------------------------------

func searchRootDSE(id int64) []byte {
	var b cryptobyte.Builder
	b.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
		b.AddASN1Int64(id)
		b.AddASN1(tagSearchRequest, func(b *cryptobyte.Builder) {
			b.AddASN1OctetString(nil) // baseObject ""
			b.AddASN1Enum(0)          // scope baseObject
			b.AddASN1Enum(0)          // derefAliases never
			b.AddASN1Int64(0)         // sizeLimit
			b.AddASN1Int64(10)        // timeLimit
			b.AddASN1Boolean(false)   // typesOnly
			b.AddASN1(tagFilterPresent, func(b *cryptobyte.Builder) { b.AddBytes([]byte("objectClass")) })
			b.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
				for _, a := range RootDSEAttributes {
					b.AddASN1OctetString([]byte(a))
				}
			})
		})
	})
	return b.BytesOrPanic()
}

func startTLSRequest(id int64) []byte {
	var b cryptobyte.Builder
	b.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
		b.AddASN1Int64(id)
		b.AddASN1(tagExtendedReq, func(b *cryptobyte.Builder) {
			b.AddASN1(tagExtRequestName, func(b *cryptobyte.Builder) { b.AddBytes([]byte(StartTLSOID)) })
		})
	})
	return b.BytesOrPanic()
}

// readMessage reads one BER element (definite length) from r.
func readMessage(r *bufio.Reader) ([]byte, error) {
	head := make([]byte, 2)
	if _, err := io.ReadFull(r, head); err != nil {
		return nil, err //nolint:wrapcheck // mapped by caller
	}
	if head[0] != 0x30 {
		return nil, errors.New("not an LDAP message")
	}
	n := int(head[1])
	lenBytes := []byte{}
	if n&0x80 != 0 {
		k := n & 0x7f
		if k == 0 || k > 4 {
			return nil, errors.New("bad length")
		}
		lenBytes = make([]byte, k)
		if _, err := io.ReadFull(r, lenBytes); err != nil {
			return nil, err //nolint:wrapcheck // mapped by caller
		}
		n = 0
		for _, c := range lenBytes {
			n = n<<8 | int(c)
		}
	}
	if n > MaxMessage {
		return nil, errors.New("message too large")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err //nolint:wrapcheck // mapped by caller
	}
	return append(append(head, lenBytes...), body...), nil
}

// parsed is the protocol op of one message.
type parsed struct {
	tag     asn1.Tag
	entry   map[string][]string
	code    int64
	message string
}

func parseMessage(raw []byte) (parsed, error) {
	var p parsed
	in := cryptobyte.String(raw)
	var msg, op cryptobyte.String
	var id int64
	var tag asn1.Tag
	if !in.ReadASN1(&msg, asn1.SEQUENCE) || !msg.ReadASN1Integer(&id) || !msg.ReadAnyASN1(&op, &tag) {
		return p, errors.New("malformed LDAP message")
	}
	p.tag = tag
	switch tag {
	case tagSearchEntry:
		var dn, attrs cryptobyte.String
		if !op.ReadASN1(&dn, asn1.OCTET_STRING) || !op.ReadASN1(&attrs, asn1.SEQUENCE) {
			return p, errors.New("malformed search entry")
		}
		p.entry = map[string][]string{}
		for !attrs.Empty() {
			var attr, typ, vals cryptobyte.String
			if !attrs.ReadASN1(&attr, asn1.SEQUENCE) || !attr.ReadASN1(&typ, asn1.OCTET_STRING) || !attr.ReadASN1(&vals, asn1.SET) {
				return p, errors.New("malformed attribute")
			}
			name := string(typ)
			for !vals.Empty() {
				var v cryptobyte.String
				if !vals.ReadASN1(&v, asn1.OCTET_STRING) {
					break
				}
				if len(p.entry[name]) < maxValues {
					p.entry[name] = append(p.entry[name], strings.ToValidUTF8(string(v), "?"))
				}
			}
		}
	case tagSearchDone, tagExtendedResp:
		var code int
		var dn, diag cryptobyte.String
		if !op.ReadASN1Enum(&code) || !op.ReadASN1(&dn, asn1.OCTET_STRING) || !op.ReadASN1(&diag, asn1.OCTET_STRING) {
			return p, errors.New("malformed result")
		}
		p.code, p.message = int64(code), string(diag)
	}
	return p, nil
}

// --- probing -----------------------------------------------------------------------------------

func readRootDSE(conn net.Conn, r *bufio.Reader, ep *Endpoint) error {
	if _, err := conn.Write(searchRootDSE(1)); err != nil {
		return err //nolint:wrapcheck // mapped by caller
	}
	for range 10 {
		raw, err := readMessage(r)
		if err != nil {
			return err
		}
		p, err := parseMessage(raw)
		if err != nil {
			return err
		}
		switch p.tag {
		case tagSearchEntry:
			ep.RootDSE = p.entry
		case tagSearchDone:
			if p.code != 0 {
				ep.ResultMsg = p.message
				return errors.New("search refused")
			}
			return nil
		}
	}
	return errors.New("no search result")
}

func session(st tls.ConnectionState, sni string) *tlsx.Session {
	s := &tlsx.Session{Version: tls.VersionName(st.Version), Cipher: tls.CipherSuiteName(st.CipherSuite), ServerName: sni, Chain: []tlsx.Cert{}}
	for _, c := range st.PeerCertificates {
		s.Chain = append(s.Chain, tlsx.Summarize(c, time.Now()))
	}
	if err := tlsx.Verify(st.PeerCertificates, sni, time.Now()); err != nil {
		s.VerifyError = err.Error()
	} else {
		s.Verified = true
	}
	return s
}

func errCode(err error) string {
	var ne net.Error
	switch {
	case errors.As(err, &ne) && ne.Timeout():
		return "timeout"
	case strings.Contains(err.Error(), "refused"):
		return "refused"
	case strings.Contains(err.Error(), "search refused"):
		return "search-refused"
	case errors.Is(err, io.EOF):
		return "closed"
	}
	return "protocol-error"
}

// Probe checks one endpoint: "ldap" (plain, then StartTLS) or "ldaps".
func Probe(ctx context.Context, addr, sni, mode string) Endpoint {
	ep := Endpoint{Mode: mode, Address: addr, RootDSE: map[string][]string{}}
	ctx, cancel := context.WithTimeout(ctx, PerEndpoint)
	defer cancel()
	var d net.Dialer
	raw, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		ep.Error = errCode(err)
		return ep
	}
	defer raw.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(dl)
	}
	ep.Reachable = true
	cfg := &tls.Config{ServerName: sni, InsecureSkipVerify: true} //nolint:gosec // verified separately and reported
	conn := raw
	if mode == "ldaps" {
		tc := tls.Client(raw, cfg)
		if err := tc.HandshakeContext(ctx); err != nil {
			ep.Error = "tls-failed"
			return ep
		}
		ep.TLS = session(tc.ConnectionState(), sni)
		conn = tc
	}
	r := bufio.NewReader(conn)
	if err := readRootDSE(conn, r, &ep); err != nil {
		ep.Error = errCode(err)
		return ep
	}
	if mode != "ldap" {
		return ep
	}
	// StartTLS on a fresh connection, so the rootDSE above reflects the plain listener.
	raw2, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return ep
	}
	defer raw2.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = raw2.SetDeadline(dl)
	}
	if _, err := raw2.Write(startTLSRequest(1)); err != nil {
		return ep
	}
	resp, err := readMessage(bufio.NewReader(raw2))
	if err != nil {
		return ep
	}
	if p, err := parseMessage(resp); err == nil && p.tag == tagExtendedResp && p.code == 0 {
		tc := tls.Client(raw2, cfg)
		if err := tc.HandshakeContext(ctx); err == nil {
			ep.StartTLS = true
			ep.TLS = session(tc.ConnectionState(), sni)
		}
	}
	return ep
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	host := q.String("host", "", true, 260)
	mode := q.Enum("mode", "both", "both", "ldap", "ldaps")
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
	sni := tgt.Host
	if tgt.IsIP {
		sni = ""
	}
	modes := []string{"ldap", "ldaps"}
	if mode != "both" {
		modes = []string{mode}
	}
	if port == 0 {
		port = tgt.Port
	}
	ctx := r.Context()
	start := time.Now()
	res := Result{Host: tgt.Host, Endpoints: make([]Endpoint, len(modes)), Warnings: []string{}}
	var wg sync.WaitGroup
	for i, m := range modes {
		wg.Go(func() {
			p := port
			if p == 0 || len(modes) > 1 {
				p = map[string]int{"ldap": 389, "ldaps": 636}[m]
			}
			res.Endpoints[i] = Probe(ctx, tgt.AddrPort(p), sni, m)
		})
	}
	wg.Wait()
	for _, ep := range res.Endpoints {
		if ep.Mode == "ldap" && ep.Reachable && ep.Error == "" && !ep.StartTLS {
			res.Warnings = append(res.Warnings, "no-starttls")
		}
		if ep.TLS != nil && !ep.TLS.Verified {
			res.Warnings = append(res.Warnings, "cert-untrusted")
		}
		if ep.TLS != nil && len(ep.TLS.Chain) > 0 && ep.TLS.Chain[0].DaysLeft < 30 {
			res.Warnings = append(res.Warnings, "cert-expiring")
		}
	}
	slices.Sort(res.Warnings)
	res.Warnings = slices.Compact(res.Warnings)
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start)), Target: &httpapi.TargetMeta{Input: host, Used: tgt.IP.String()}})
}
