package sipcheck

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/digibaro/net-tools-docker/apps/api/internal/dnsx/dnstest"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

const reply = "SIP/2.0 200 OK\r\nVia: x\r\nServer: Asterisk PBX 20.1\r\nAllow: INVITE, ACK, OPTIONS, BYE\r\nContent-Length: 0\r\n\r\n"

func selfSigned(t *testing.T) tls.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "sip.test"}, DNSNames: []string{"sip.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// servers starts UDP, TCP and TLS SIP responders on one port number where possible.
func servers(t *testing.T) (udpPort, tcpPort, tlsPort int) {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	go func() {
		buf := make([]byte, 4096)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			if strings.HasPrefix(string(buf[:n]), "OPTIONS sip:") {
				_, _ = pc.WriteTo([]byte(reply), from)
			}
		}
	}()
	serve := func(ln net.Listener) {
		t.Cleanup(func() { _ = ln.Close() })
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				go func() {
					defer c.Close()
					r := bufio.NewReader(c)
					for {
						l, err := r.ReadString('\n')
						if err != nil || l == "\r\n" {
							break
						}
					}
					_, _ = c.Write([]byte(reply))
				}()
			}
		}()
	}
	tcp, _ := net.Listen("tcp", "127.0.0.1:0")
	serve(tcp)
	tl, _ := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{selfSigned(t)}})
	serve(tl)
	return pc.LocalAddr().(*net.UDPAddr).Port, tcp.Addr().(*net.TCPAddr).Port, tl.Addr().(*net.TCPAddr).Port
}

func TestSIPProbes(t *testing.T) {
	u, tc, tl := servers(t)
	ctx := t.Context()
	if p := probeUDP(ctx, "sip.test", "127.0.0.1:"+strconv.Itoa(u)); !p.Reachable || p.Status != 200 || p.Server != "Asterisk PBX 20.1" || len(p.Allow) != 4 {
		t.Errorf("udp = %+v", p)
	}
	if p := probeStream(ctx, "sip.test", "sip.test", "127.0.0.1:"+strconv.Itoa(tc), false); p.Status != 200 || p.Transport != "tcp" {
		t.Errorf("tcp = %+v", p)
	}
	p := probeStream(ctx, "sip.test", "sip.test", "127.0.0.1:"+strconv.Itoa(tl), true)
	if p.Status != 200 || p.Cert == nil || p.Cert.Subject != "CN=sip.test" || p.CertError == "" {
		t.Errorf("tls = %+v", p)
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	closed := ln.Addr().String()
	_ = ln.Close()
	if p := probeStream(ctx, "sip.test", "", closed, false); p.Reachable || p.Error != "refused" {
		t.Errorf("closed = %+v", p)
	}
}

func TestSIPHandler(t *testing.T) {
	u, tc, _ := servers(t)
	zone := dnstest.Zone(t,
		"_sip._udp.example.test. 300 IN SRV 10 50 "+strconv.Itoa(u)+" pbx.example.test.",
		"_sip._tcp.example.test. 300 IN SRV 10 50 "+strconv.Itoa(tc)+" pbx.example.test.",
		"example.test. 300 IN NAPTR 10 50 \"s\" \"SIP+D2U\" \"\" _sip._udp.example.test.",
	)
	env := tooltest.New(t, &Tool{Resolver: zone.String()}, tooltest.Options{AllowPrivate: true, Lookup: tooltest.StaticLookup("127.0.0.1")})
	var r Result
	env.Get("/api/v1/sip/check?host=example.test&transport=all").Must(t).Decode(t, &r)
	if len(r.SRV) != 2 || len(r.NAPTR) != 1 || r.Host != "pbx.example.test" || len(r.Probes) != 3 {
		t.Fatalf("result = %+v", r)
	}
	if !r.Probes[0].Reachable || !r.Probes[1].Reachable || r.Probes[2].Reachable {
		t.Errorf("probes = %+v", r.Probes)
	}
	tooltest.New(t, &Tool{}, tooltest.Options{}).Get("/api/v1/sip/check?host=10.0.0.5").ExpectError(t, "TARGET_PRIVATE_DENIED")
	tooltest.New(t, &Tool{}, tooltest.Options{}).Get("/api/v1/sip/check?host=x&transport=sctp").ExpectError(t, "INVALID_INPUT")
}

func TestParseResponse(t *testing.T) {
	var p Probe
	if err := parseResponse("HTTP/1.1 200 OK\r\n\r\n", &p); err == nil {
		t.Error("HTTP accepted as SIP")
	}
	if err := parseResponse("SIP/2.0 403 Forbidden\r\nUser-Agent: FRITZ!OS\r\n\r\n", &p); err != nil || p.Status != 403 || p.Server != "FRITZ!OS" {
		t.Errorf("p = %+v err %v", p, err)
	}
}
