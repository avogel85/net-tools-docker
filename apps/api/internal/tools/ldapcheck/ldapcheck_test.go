package ldapcheck

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
	"net/url"
	"slices"
	"testing"
	"time"

	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func cert(t *testing.T) tls.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "dc1.corp.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(240 * time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func reply(id int64, tag asn1.Tag, body func(*cryptobyte.Builder)) []byte {
	var b cryptobyte.Builder
	b.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
		b.AddASN1Int64(id)
		b.AddASN1(tag, body)
	})
	return b.BytesOrPanic()
}

func result(code int64) func(*cryptobyte.Builder) {
	return func(b *cryptobyte.Builder) {
		b.AddASN1Enum(code)
		b.AddASN1OctetString(nil)
		b.AddASN1OctetString(nil)
	}
}

// fakeLDAP answers rootDSE searches and (optionally) StartTLS.
func fakeLDAP(t *testing.T, ln net.Listener, startTLS bool, crt tls.Certificate) {
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
					raw, err := readMessage(r)
					if err != nil {
						return
					}
					in := cryptobyte.String(raw)
					var msg, op cryptobyte.String
					var id int64
					var tag asn1.Tag
					in.ReadASN1(&msg, asn1.SEQUENCE)
					msg.ReadASN1Integer(&id)
					msg.ReadAnyASN1(&op, &tag)
					switch tag {
					case tagSearchRequest:
						_, _ = c.Write(reply(id, tagSearchEntry, func(b *cryptobyte.Builder) {
							b.AddASN1OctetString(nil)
							b.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
								for k, vs := range map[string][]string{"defaultNamingContext": {"DC=corp,DC=test"}, "supportedLDAPVersion": {"3", "2"}, "dnsHostName": {"dc1.corp.test"}} {
									b.AddASN1(asn1.SEQUENCE, func(b *cryptobyte.Builder) {
										b.AddASN1OctetString([]byte(k))
										b.AddASN1(asn1.SET, func(b *cryptobyte.Builder) {
											for _, v := range vs {
												b.AddASN1OctetString([]byte(v))
											}
										})
									})
								}
							})
						}))
						_, _ = c.Write(reply(id, tagSearchDone, result(0)))
					case tagExtendedReq:
						if !startTLS {
							_, _ = c.Write(reply(id, tagExtendedResp, result(2)))
							continue
						}
						_, _ = c.Write(reply(id, tagExtendedResp, result(0)))
						tc := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{crt}})
						_ = tc.Handshake()
						return
					}
				}
			}()
		}
	}()
}

func TestProbe(t *testing.T) {
	crt := cert(t)
	ctx := t.Context()

	plain, _ := net.Listen("tcp", "127.0.0.1:0")
	fakeLDAP(t, plain, true, crt)
	ep := Probe(ctx, plain.Addr().String(), "dc1.corp.test", "ldap")
	if ep.Error != "" || !ep.StartTLS || ep.TLS == nil || ep.TLS.Chain[0].Subject != "CN=dc1.corp.test" || ep.TLS.Verified {
		t.Fatalf("ldap = %+v", ep)
	}
	if !slices.Equal(ep.RootDSE["supportedLDAPVersion"], []string{"3", "2"}) || ep.RootDSE["defaultNamingContext"][0] != "DC=corp,DC=test" {
		t.Errorf("rootDSE = %v", ep.RootDSE)
	}

	nostls, _ := net.Listen("tcp", "127.0.0.1:0")
	fakeLDAP(t, nostls, false, crt)
	if ep := Probe(ctx, nostls.Addr().String(), "", "ldap"); ep.StartTLS || ep.Error != "" {
		t.Errorf("no starttls = %+v", ep)
	}

	inner, _ := net.Listen("tcp", "127.0.0.1:0")
	tl := tls.NewListener(inner, &tls.Config{Certificates: []tls.Certificate{crt}})
	fakeLDAP(t, tl, false, crt)
	if ep := Probe(ctx, inner.Addr().String(), "dc1.corp.test", "ldaps"); ep.Error != "" || ep.TLS == nil || ep.RootDSE["dnsHostName"][0] != "dc1.corp.test" {
		t.Errorf("ldaps = %+v", ep)
	}

	closed, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := closed.Addr().String()
	_ = closed.Close()
	if ep := Probe(ctx, addr, "", "ldap"); ep.Reachable || ep.Error != "refused" {
		t.Errorf("closed = %+v", ep)
	}
}

func TestHandler(t *testing.T) {
	plain, _ := net.Listen("tcp", "127.0.0.1:0")
	fakeLDAP(t, plain, false, cert(t))
	env := tooltest.New(t, &Tool{}, tooltest.Options{AllowPrivate: true})
	var r Result
	env.Get("/api/v1/ldap/check?mode=ldap&host="+url.QueryEscape(plain.Addr().String())).Must(t).Decode(t, &r)
	if len(r.Endpoints) != 1 || r.Endpoints[0].Error != "" || !slices.Contains(r.Warnings, "no-starttls") {
		t.Errorf("result = %+v", r)
	}
	tooltest.New(t, &Tool{}, tooltest.Options{}).Get("/api/v1/ldap/check?host=10.0.0.10").ExpectError(t, "TARGET_PRIVATE_DENIED")
	tooltest.New(t, &Tool{}, tooltest.Options{}).Get("/api/v1/ldap/check?host=x.test&mode=ftp").ExpectError(t, "INVALID_INPUT")
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := parseMessage([]byte{0x30, 0x03, 0x02, 0x01}); err == nil {
		t.Error("truncated message parsed")
	}
}
