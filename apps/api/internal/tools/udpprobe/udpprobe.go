// Package udpprobe implements CON-19: service-aware UDP probes that tell open from open|filtered
// by sending a real request for the service and recognising its answer.
package udpprobe

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/miekg/dns"
	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnslookup"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/ikeprobe"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// PerAttempt bounds one probe packet; each is sent at most twice. Var for tests.
var PerAttempt = 2500 * time.Millisecond

// Probe describes one service: how to ask and how to recognise (and describe) the answer.
type Probe struct {
	Name string
	Port int
	// Request builds the packet; check validates a reply and returns a short description.
	Request func() ([]byte, func([]byte) (string, bool))
}

// Probes, in display order.
var Probes = []Probe{
	{"dns", 53, dnsProbe},
	{"ntp", 123, ntpProbe},
	{"snmp", 161, snmpProbe},
	{"sip", 5060, sipProbe},
	{"stun", 3478, stunProbe},
	{"ike", 500, ikeProbe},
	{"quic", 443, quicProbe},
	{"openvpn", 1194, openvpnProbe},
	{"netbios", 137, netbiosProbe},
	{"tftp", 69, tftpProbe},
}

func random(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func dnsProbe() ([]byte, func([]byte) (string, bool)) {
	m := new(dns.Msg)
	m.SetQuestion(".", dns.TypeNS)
	b, _ := m.Pack()
	return b, func(r []byte) (string, bool) {
		var resp dns.Msg
		if resp.Unpack(r) != nil || !resp.Response || resp.Id != m.Id {
			return "", false
		}
		return fmt.Sprintf("%s, recursion %s", dns.RcodeToString[resp.Rcode], map[bool]string{true: "available", false: "not available"}[resp.RecursionAvailable]), true
	}
}

func ntpProbe() ([]byte, func([]byte) (string, bool)) {
	b := make([]byte, 48)
	b[0] = 0x23 // LI 0, version 4, mode 3 (client)
	copy(b[40:], random(8))
	return b, func(r []byte) (string, bool) {
		if len(r) < 48 || r[0]&0x07 != 4 || string(r[24:32]) != string(b[40:48]) {
			return "", false
		}
		ref := string(r[12:16])
		if r[1] > 1 {
			ref = net.IP(r[12:16]).String()
		}
		return fmt.Sprintf("stratum %d, reference %s", r[1], strings.Trim(ref, "\x00")), true
	}
}

// snmpProbe is an SNMPv3 engine-discovery request: no community or user is guessed; agents answer
// with a REPORT carrying their engine ID.
func snmpProbe() ([]byte, func([]byte) (string, bool)) {
	id := int64(binary.BigEndian.Uint16(random(2)))
	var usm cryptobyte.Builder
	usm.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
		b.AddASN1OctetString(nil)
		b.AddASN1Int64(0)
		b.AddASN1Int64(0)
		b.AddASN1OctetString(nil)
		b.AddASN1OctetString(nil)
		b.AddASN1OctetString(nil)
	})
	var msg cryptobyte.Builder
	msg.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
		b.AddASN1Int64(3)
		b.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
			b.AddASN1Int64(id)
			b.AddASN1Int64(65507)
			b.AddASN1OctetString([]byte{0x04}) // reportable
			b.AddASN1Int64(3)                  // USM
		})
		b.AddASN1OctetString(usm.BytesOrPanic())
		b.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
			b.AddASN1OctetString(nil)
			b.AddASN1OctetString(nil)
			b.AddASN1(asn1.Tag(0xa0), func(b *cryptobyte.Builder) { // GetRequest-PDU
				b.AddASN1Int64(id)
				b.AddASN1Int64(0)
				b.AddASN1Int64(0)
				b.AddASN1(asn1.SEQUENCE, func(*cryptobyte.Builder) {})
			})
		})
	})
	return msg.BytesOrPanic(), func(r []byte) (string, bool) {
		in := cryptobyte.String(r)
		var m, global, secParams cryptobyte.String
		var ver int64
		if !in.ReadASN1(&m, asn1.SEQUENCE) || !m.ReadASN1Integer(&ver) {
			return "", false
		}
		if ver != 3 {
			return fmt.Sprintf("SNMP v%d reply", ver+1), true
		}
		if !m.ReadASN1(&global, asn1.SEQUENCE) || !m.ReadASN1(&secParams, asn1.OCTET_STRING) {
			return "SNMPv3", true
		}
		var usmSeq, engine cryptobyte.String
		if secParams.ReadASN1(&usmSeq, asn1.SEQUENCE) && usmSeq.ReadASN1(&engine, asn1.OCTET_STRING) && len(engine) >= 4 {
			ent := binary.BigEndian.Uint32(engine[:4]) &^ 0x80000000
			return fmt.Sprintf("SNMPv3, engine ID %x (enterprise %d)", []byte(engine), ent), true
		}
		return "SNMPv3", true
	}
}

func sipProbe() ([]byte, func([]byte) (string, bool)) {
	call := fmt.Sprintf("%x", random(8))
	req := "OPTIONS sip:nettools@invalid SIP/2.0\r\nVia: SIP/2.0/UDP 0.0.0.0:5060;branch=z9hG4bK" + call + "\r\nMax-Forwards: 70\r\n" +
		"From: <sip:nettools@nettools.invalid>;tag=" + call[:8] + "\r\nTo: <sip:nettools@invalid>\r\nCall-ID: " + call + "@nettools.invalid\r\nCSeq: 1 OPTIONS\r\nContent-Length: 0\r\n\r\n"
	return []byte(req), func(r []byte) (string, bool) {
		s := string(r)
		if !strings.HasPrefix(s, "SIP/2.0 ") {
			return "", false
		}
		line, _, _ := strings.Cut(s, "\r\n")
		for _, l := range strings.Split(s, "\r\n") {
			if k, v, ok := strings.Cut(l, ":"); ok && (strings.EqualFold(k, "server") || strings.EqualFold(k, "user-agent")) {
				return strings.TrimPrefix(line, "SIP/2.0 ") + ", " + strings.TrimSpace(v), true
			}
		}
		return strings.TrimPrefix(line, "SIP/2.0 "), true
	}
}

func stunProbe() ([]byte, func([]byte) (string, bool)) {
	tid := random(12)
	b := []byte{0x00, 0x01, 0x00, 0x00, 0x21, 0x12, 0xa4, 0x42}
	b = append(b, tid...)
	return b, func(r []byte) (string, bool) {
		if len(r) < 20 || r[0] != 0x01 || r[1] != 0x01 || string(r[8:20]) != string(tid) {
			return "", false
		}
		desc := "binding success"
		attrs := r[20:]
		for len(attrs) >= 4 {
			t, n := binary.BigEndian.Uint16(attrs), int(binary.BigEndian.Uint16(attrs[2:]))
			if 4+n > len(attrs) {
				break
			}
			v := attrs[4 : 4+n]
			switch {
			case t == 0x0020 && n >= 8 && v[1] == 1: // XOR-MAPPED-ADDRESS, IPv4
				ip := make(net.IP, 4)
				for i := range 4 {
					ip[i] = v[4+i] ^ []byte{0x21, 0x12, 0xa4, 0x42}[i]
				}
				desc += ", sees us as " + ip.String()
			case t == 0x8022: // SOFTWARE
				desc += ", " + strings.TrimSpace(string(v))
			}
			attrs = attrs[4+(n+3)&^3:]
		}
		return desc, true
	}
}

func ikeProbe() ([]byte, func([]byte) (string, bool)) {
	spi := random(8)
	req, err := ikeprobe.SAInit(spi, 19)
	if err != nil {
		return nil, func([]byte) (string, bool) { return "", false }
	}
	return req, func(r []byte) (string, bool) {
		resp, err := ikeprobe.ParseV2(r, spi)
		if err != nil {
			return "", false
		}
		switch {
		case resp.Encryption != "":
			return "IKEv2, accepts " + resp.Encryption, true
		case resp.WantsGroup != 0:
			return "IKEv2, wants another DH group", true
		}
		return "IKEv2 answer", true
	}
}

// quicProbe sends a long-header packet with a reserved version; servers answer with Version
// Negotiation listing the versions they support (RFC 9000 §6).
func quicProbe() ([]byte, func([]byte) (string, bool)) {
	dcid := random(8)
	b := []byte{0xc0, 0x1a, 0x2a, 0x3a, 0x4a, 8}
	b = append(b, dcid...)
	b = append(b, 0) // SCID length
	b = append(b, make([]byte, 1200-len(b))...)
	return b, func(r []byte) (string, bool) {
		if len(r) < 7 || r[0]&0x80 == 0 || binary.BigEndian.Uint32(r[1:5]) != 0 {
			return "", false
		}
		i := 5
		dl := int(r[i])
		i += 1 + dl
		if i >= len(r) {
			return "", false
		}
		sl := int(r[i])
		i += 1 + sl
		if i > len(r) || dl != 0 && sl != 8 && dl != 8 {
			return "", false
		}
		var vs []string
		for ; i+4 <= len(r); i += 4 {
			switch v := binary.BigEndian.Uint32(r[i:]); {
			case v == 1:
				vs = append(vs, "v1")
			case v == 0x6b3343cf:
				vs = append(vs, "v2")
			case v&0x0f0f0f0f == 0x0a0a0a0a:
				// greased, skip
			default:
				vs = append(vs, fmt.Sprintf("0x%08x", v))
			}
		}
		return "QUIC " + strings.Join(vs, ", "), true
	}
}

func openvpnProbe() ([]byte, func([]byte) (string, bool)) {
	sid := random(8)
	b := append([]byte{0x38}, sid...)
	b = append(b, 0, 0, 0, 0, 0) // no ACKs, packet ID 0
	return b, func(r []byte) (string, bool) {
		if len(r) < 9 || r[0]>>3 != 8 { // P_CONTROL_HARD_RESET_SERVER_V2
			return "", false
		}
		return "OpenVPN server (no tls-auth/tls-crypt)", true
	}
}

func netbiosProbe() ([]byte, func([]byte) (string, bool)) {
	id := random(2)
	b := append(slices.Clone(id), 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 'C', 'K')
	b = append(b, []byte(strings.Repeat("A", 30))...)
	b = append(b, 0x00, 0x00, 0x21, 0x00, 0x01) // NBSTAT, IN
	return b, func(r []byte) (string, bool) {
		if len(r) < 57 || r[0] != id[0] || r[1] != id[1] || r[2]&0x80 == 0 {
			return "", false
		}
		n := int(r[56])
		var names []string
		for i := 0; i < n && 57+i*18+18 <= len(r); i++ {
			e := r[57+i*18:]
			name := strings.TrimSpace(string(e[:15]))
			if e[15] == 0x00 && name != "" && !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
		return "NetBIOS names: " + strings.Join(names, ", "), true
	}
}

func tftpProbe() ([]byte, func([]byte) (string, bool)) {
	b := append([]byte{0, 1}, []byte(fmt.Sprintf("nettools-probe-%x", random(4)))...)
	b = append(b, 0)
	b = append(b, []byte("octet")...)
	b = append(b, 0)
	return b, func(r []byte) (string, bool) {
		if len(r) < 4 || r[0] != 0 || (r[1] != 5 && r[1] != 3) {
			return "", false
		}
		if r[1] == 3 {
			return "TFTP server (served our random file name!)", true
		}
		return "TFTP server, error " + strings.TrimRight(string(r[4:]), "\x00"), true
	}
}

// Result for one probe.
type Result struct {
	Service string `json:"service"`
	Port    int    `json:"port"`
	// State: open (a valid answer), closed (ICMP port unreachable), open|filtered (no answer),
	// or unexpected (an answer that is not this service).
	State  string  `json:"state"`
	Detail string  `json:"detail"`
	RTTMs  float64 `json:"rttMs"`
}

// Run sends one probe to addr (ip:port).
func Run(ctx context.Context, p Probe, addr string, port int) Result {
	res := Result{Service: p.Name, Port: port, State: "open|filtered"}
	req, check := p.Request()
	if p.Name == "custom" {
		req = []byte{}
	}
	var d net.Dialer
	c, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		res.State = "error"
		return res
	}
	defer c.Close()
	buf := make([]byte, 4096)
	for range 2 {
		start := time.Now()
		_ = c.SetDeadline(time.Now().Add(PerAttempt))
		if _, err := c.Write(req); err != nil {
			if errors.Is(err, syscall.ECONNREFUSED) {
				res.State = "closed"
			}
			return res
		}
		n, err := c.Read(buf)
		if err != nil {
			if errors.Is(err, syscall.ECONNREFUSED) {
				res.State = "closed"
				return res
			}
			if ctx.Err() != nil {
				return res
			}
			continue
		}
		res.RTTMs = float64(time.Since(start).Microseconds()) / 1000
		if check == nil {
			res.State, res.Detail = "open", fmt.Sprintf("%d bytes", n)
			return res
		}
		if desc, ok := check(buf[:n]); ok {
			res.State, res.Detail = "open", desc
		} else {
			res.State, res.Detail = "unexpected", fmt.Sprintf("%d bytes that are not %s", n, p.Name)
		}
		return res
	}
	return res
}

// Tool is the UDP service probe.
type Tool struct{ deps tools.Deps }

func (*Tool) ID() string                                { return "con.udpprobe" }
func (*Tool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(30*time.Second)).Get("/udp/probe", t.handle)
}

// Response is the response payload.
type Response struct {
	Host    string   `json:"host"`
	Address string   `json:"address"`
	Results []Result `json:"results"`
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	names := []string{"all", "custom"}
	for _, p := range Probes {
		names = append(names, p.Name)
	}
	q := params.New(r.URL.Query())
	host := q.String("host", "", true, 260)
	service := q.Enum("service", "all", names...)
	port := q.Int("port", 0, 0, 65535)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	if service == "custom" && port == 0 {
		httpapi.Error(w, r, httpapi.InvalidInput("port", "a custom probe needs a port"))
		return
	}
	tgt, err := t.deps.Target.Resolve(r.Context(), host, target.Opts{Field: "host"})
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	var run []Probe
	switch service {
	case "all":
		run = Probes
	case "custom":
		run = []Probe{{Name: "custom", Port: port, Request: func() ([]byte, func([]byte) (string, bool)) { return nil, nil }}}
	default:
		for _, p := range Probes {
			if p.Name == service {
				run = []Probe{p}
			}
		}
	}
	ctx := r.Context()
	start := time.Now()
	resp := Response{Host: tgt.Host, Address: tgt.IP.String(), Results: make([]Result, len(run))}
	var wg sync.WaitGroup
	for i, p := range run {
		wg.Go(func() {
			pt := p.Port
			if port != 0 && len(run) == 1 {
				pt = port
			}
			resp.Results[i] = Run(ctx, p, tgt.AddrPort(pt), pt)
		})
	}
	wg.Wait()
	httpapi.JSON(w, r, resp, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start)), Target: &httpapi.TargetMeta{Input: host, Used: resp.Address}})
}
