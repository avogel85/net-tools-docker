package remoteaccess

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"io"
	"math/big"
	"net"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func listen(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))
				handle(c)
			}()
		}
	}()
	return ln.Addr().String()
}

func cert() tls.Certificate {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "WS01.corp.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// rdpServer answers like Windows: with nla it insists on CredSSP, otherwise it accepts TLS.
func rdpServer(nla, legacy bool) func(net.Conn) {
	crt := cert()
	return func(c net.Conn) {
		req := make([]byte, 19)
		if _, err := io.ReadFull(c, req); err != nil {
			return
		}
		requested := binary.LittleEndian.Uint32(req[15:])
		if legacy {
			_, _ = c.Write([]byte{0x03, 0x00, 0x00, 0x0b, 0x06, 0xd0, 0x00, 0x00, 0x12, 0x34, 0x00})
			return
		}
		resp := []byte{0x03, 0x00, 0x00, 0x13, 0x0e, 0xd0, 0x00, 0x00, 0x12, 0x34, 0x00, negRsp, 0x00, 0x08, 0x00, 0, 0, 0, 0}
		selected := uint32(protoSSL)
		switch {
		case requested&protoHybrid != 0:
			selected = protoHybrid
		case nla:
			resp[11] = negFailure
			selected = hybridNeeded
		}
		binary.LittleEndian.PutUint32(resp[15:], selected)
		_, _ = c.Write(resp)
		if resp[11] == negRsp {
			_ = tls.Server(c, &tls.Config{Certificates: []tls.Certificate{crt}}).Handshake()
		}
	}
}

func TestRDP(t *testing.T) {
	ctx := t.Context()
	s := CheckRDP(ctx, listen(t, rdpServer(true, false)), "")
	if !s.Open || s.Protocol != "credssp" || s.NLARequired == nil || !*s.NLARequired || s.Cert == nil || s.Cert.Subject != "CN=WS01.corp.test" || len(s.Findings) != 0 {
		t.Errorf("nla = %+v", s)
	}
	s = CheckRDP(ctx, listen(t, rdpServer(false, false)), "")
	if s.NLARequired == nil || *s.NLARequired || !slices.Contains(s.Findings, "rdp-nla-not-required") {
		t.Errorf("no nla = %+v", s)
	}
	s = CheckRDP(ctx, listen(t, rdpServer(false, true)), "")
	if s.Protocol != "rdp" || !slices.Contains(s.Findings, "rdp-legacy-security") {
		t.Errorf("legacy = %+v", s)
	}
	if s := CheckRDP(ctx, listen(t, func(c net.Conn) { _, _ = c.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n")) }), ""); s.Error != "protocol-error" {
		t.Errorf("http on 3389 = %+v", s)
	}
}

func vncServer(types ...byte) func(net.Conn) {
	return func(c net.Conn) {
		_, _ = c.Write([]byte("RFB 003.008\n"))
		buf := make([]byte, 12)
		if _, err := io.ReadFull(c, buf); err != nil {
			return
		}
		_, _ = c.Write(append([]byte{byte(len(types))}, types...))
	}
}

func TestVNC(t *testing.T) {
	ctx := t.Context()
	if s := CheckVNC(ctx, listen(t, vncServer(1))); !slices.Equal(s.SecurityTypes, []string{"none"}) || !slices.Contains(s.Findings, "vnc-no-auth") || s.RFBVersion != "003.008" {
		t.Errorf("none = %+v", s)
	}
	if s := CheckVNC(ctx, listen(t, vncServer(2, 99))); !slices.Equal(s.SecurityTypes, []string{"vnc-auth", "type-99"}) || !slices.Contains(s.Findings, "vnc-unencrypted") {
		t.Errorf("vnc-auth = %+v", s)
	}
	if s := CheckVNC(ctx, listen(t, vncServer(19))); len(s.Findings) != 0 {
		t.Errorf("vencrypt = %+v", s)
	}
}

func TestSSH(t *testing.T) {
	addr := listen(t, func(c net.Conn) { _, _ = c.Write([]byte("Welcome\r\nSSH-2.0-OpenSSH_9.6p1 Debian\r\n")) })
	if s := CheckSSH(t.Context(), addr); s.Banner != "SSH-2.0-OpenSSH_9.6p1 Debian" || len(s.Findings) != 0 {
		t.Errorf("ssh = %+v", s)
	}
	old := listen(t, func(c net.Conn) { _, _ = c.Write([]byte("SSH-1.5-old\n")) })
	if s := CheckSSH(t.Context(), old); !slices.Contains(s.Findings, "ssh-v1") {
		t.Errorf("ssh1 = %+v", s)
	}
}

func TestHandler(t *testing.T) {
	addr := listen(t, vncServer(2))
	env := tooltest.New(t, &Tool{}, tooltest.Options{AllowPrivate: true})
	var r Result
	env.Get("/api/v1/remote-access/check?service=vnc&host="+url.QueryEscape(addr)).Must(t).Decode(t, &r)
	if len(r.Services) != 1 || r.Services[0].Service != "vnc" || !r.Services[0].Open {
		t.Errorf("result = %+v", r)
	}
	tooltest.New(t, &Tool{}, tooltest.Options{}).Get("/api/v1/remote-access/check?host=192.168.1.10").ExpectError(t, "TARGET_PRIVATE_DENIED")
	tooltest.New(t, &Tool{}, tooltest.Options{}).Get("/api/v1/remote-access/check?host=x.test&service=telnet").ExpectError(t, "INVALID_INPUT")
}
