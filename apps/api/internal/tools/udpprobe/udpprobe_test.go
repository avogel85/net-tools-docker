package udpprobe

import (
	"encoding/binary"
	"net"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/digibaro/net-tools-docker/apps/api/internal/dnsx/dnstest"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func udpServer(t *testing.T, reply func(req []byte) []byte) int {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			if out := reply(buf[:n]); out != nil {
				_, _ = pc.WriteTo(out, from)
			}
		}
	}()
	return pc.LocalAddr().(*net.UDPAddr).Port
}

func probe(name string) Probe {
	for _, p := range Probes {
		if p.Name == name {
			return p
		}
	}
	panic(name)
}

func TestProbes(t *testing.T) {
	PerAttempt = 300 * time.Millisecond
	t.Cleanup(func() { PerAttempt = 2500 * time.Millisecond })
	ctx := t.Context()
	at := func(port int) string { return "127.0.0.1:" + strconv.Itoa(port) }

	dnsAddr := dnstest.Zone(t, ". 300 IN NS a.root-servers.net.")
	if r := Run(ctx, probe("dns"), dnsAddr.String(), int(dnsAddr.Port())); r.State != "open" || !strings.HasPrefix(r.Detail, "NOERROR") {
		t.Errorf("dns = %+v", r)
	}

	ntp := udpServer(t, func(req []byte) []byte {
		out := make([]byte, 48)
		out[0], out[1] = 0x24, 2
		copy(out[12:], []byte{192, 0, 2, 1})
		copy(out[24:32], req[40:48])
		return out
	})
	if r := Run(ctx, probe("ntp"), at(ntp), ntp); r.State != "open" || r.Detail != "stratum 2, reference 192.0.2.1" {
		t.Errorf("ntp = %+v", r)
	}

	stun := udpServer(t, func(req []byte) []byte {
		out := append([]byte{0x01, 0x01, 0, 12}, req[4:20]...)
		attr := []byte{0x00, 0x20, 0x00, 0x08, 0x00, 0x01, 0x00, 0x00}
		for i, b := range []byte{198, 51, 100, 7} {
			attr = append(attr, b^[]byte{0x21, 0x12, 0xa4, 0x42}[i])
		}
		return append(out, attr...)
	})
	if r := Run(ctx, probe("stun"), at(stun), stun); r.State != "open" || !strings.Contains(r.Detail, "198.51.100.7") {
		t.Errorf("stun = %+v", r)
	}

	quic := udpServer(t, func(req []byte) []byte {
		if len(req) < 1200 {
			return nil
		}
		out := []byte{0x80, 0, 0, 0, 0, 0, 8}
		out = append(out, req[6:14]...)
		out = binary.BigEndian.AppendUint32(out, 1)
		return binary.BigEndian.AppendUint32(out, 0x6b3343cf)
	})
	if r := Run(ctx, probe("quic"), at(quic), quic); r.State != "open" || r.Detail != "QUIC v1, v2" {
		t.Errorf("quic = %+v", r)
	}

	tftp := udpServer(t, func([]byte) []byte { return append([]byte{0, 5, 0, 1}, []byte("File not found\x00")...) })
	if r := Run(ctx, probe("tftp"), at(tftp), tftp); r.State != "open" || !strings.Contains(r.Detail, "File not found") {
		t.Errorf("tftp = %+v", r)
	}

	junk := udpServer(t, func([]byte) []byte { return []byte("hello") })
	if r := Run(ctx, probe("ntp"), at(junk), junk); r.State != "unexpected" {
		t.Errorf("junk = %+v", r)
	}
	silent := udpServer(t, func([]byte) []byte { return nil })
	if r := Run(ctx, probe("dns"), at(silent), silent); r.State != "open|filtered" {
		t.Errorf("silent = %+v", r)
	}

	pc, _ := net.ListenPacket("udp", "127.0.0.1:0")
	closed := pc.LocalAddr().(*net.UDPAddr).Port
	_ = pc.Close()
	if r := Run(ctx, probe("dns"), at(closed), closed); r.State != "closed" {
		t.Errorf("closed = %+v", r)
	}
}

func TestSNMPRequestParses(t *testing.T) {
	req, check := snmpProbe()
	if req[0] != 0x30 {
		t.Fatalf("not a sequence: %x", req)
	}
	// An agent's REPORT echoes the message layout with its engine ID filled in; feeding our own
	// request back exercises the parser (empty engine ID → plain "SNMPv3").
	if d, ok := check(req); !ok || d != "SNMPv3" {
		t.Errorf("check(own request) = %q %v", d, ok)
	}
	if _, ok := check([]byte("garbage")); ok {
		t.Error("garbage accepted")
	}
}

func TestHandler(t *testing.T) {
	dnsAddr := dnstest.Zone(t, ". 300 IN NS a.root-servers.net.")
	env := tooltest.New(t, &Tool{}, tooltest.Options{AllowPrivate: true})
	var r Response
	env.Get("/api/v1/udp/probe?host=127.0.0.1&service=dns&port="+strconv.Itoa(int(dnsAddr.Port()))).Must(t).Decode(t, &r)
	if len(r.Results) != 1 || r.Results[0].State != "open" {
		t.Errorf("result = %+v", r)
	}
	env.Get("/api/v1/udp/probe?host=127.0.0.1&service=custom").ExpectError(t, "INVALID_INPUT")
	env.Get("/api/v1/udp/probe?host=127.0.0.1&service=chargen").ExpectError(t, "INVALID_INPUT")
	tooltest.New(t, &Tool{}, tooltest.Options{}).Get("/api/v1/udp/probe?host="+url.QueryEscape("10.0.0.1")).ExpectError(t, "TARGET_PRIVATE_DENIED")
}
