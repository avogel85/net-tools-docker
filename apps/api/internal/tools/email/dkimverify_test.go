package email

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"slices"
	"strings"
	"testing"

	"github.com/emersion/go-msgauth/dkim"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

const plainMsg = "From: Alice <alice@example.com>\r\nTo: bob@example.net\r\nSubject: Hello\r\nDate: Mon, 16 Nov 2026 10:00:00 +0000\r\nMessage-ID: <1@example.com>\r\n\r\nHi Bob,\r\nthis is a test.\r\n"

func sign(t *testing.T, msg, domain, selector string) (string, fakeDNS) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	var out bytes.Buffer
	if err := dkim.Sign(&out, strings.NewReader(msg), &dkim.SignOptions{Domain: domain, Selector: selector, Signer: key, HeaderKeys: []string{"From", "To", "Subject", "Date"}}); err != nil {
		t.Fatal(err)
	}
	return out.String(), fakeDNS{txt: map[string][]string{selector + "._domainkey." + domain: {"v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(pub)}}}
}

func TestVerifyDKIM(t *testing.T) {
	ctx := context.Background()
	signed, zone := sign(t, plainMsg, "example.com", "s1")

	r := VerifyDKIM(ctx, zone, signed)
	if len(r.Signatures) != 1 || r.Signatures[0].Result != "pass" || !r.Signatures[0].FromAligned || len(r.Problems) != 0 {
		t.Fatalf("pass = %+v", r)
	}
	s := r.Signatures[0]
	if s.Selector != "s1" || s.Algorithm != "rsa-sha256" || !strings.HasPrefix(s.KeyRecord, "v=DKIM1") || r.Subject != "Hello" {
		t.Errorf("details = %+v", s)
	}

	// Pasted with bare LF line endings: still verifies.
	if r = VerifyDKIM(ctx, zone, strings.ReplaceAll(signed, "\r\n", "\n")); r.Signatures[0].Result != "pass" {
		t.Errorf("LF = %+v", r.Signatures[0])
	}
	// Body changed after signing.
	if r = VerifyDKIM(ctx, zone, strings.Replace(signed, "this is a test", "this is a TEST", 1)); r.Signatures[0].Result != "fail" || !slices.Contains(r.Signatures[0].Warnings, "body-changed") || !slices.Contains(r.Problems, "no-aligned-pass") {
		t.Errorf("tampered = %+v", r)
	}
	// Key not published.
	if r = VerifyDKIM(ctx, fakeDNS{}, signed); r.Signatures[0].Result == "pass" {
		t.Errorf("no key = %+v", r.Signatures[0])
	}
	// Signed by another domain: valid but not aligned with From.
	other, zone2 := sign(t, plainMsg, "mailer.test", "k1")
	if r = VerifyDKIM(ctx, zone2, other); r.Signatures[0].Result != "pass" || r.Signatures[0].FromAligned || !slices.Contains(r.Problems, "no-aligned-pass") {
		t.Errorf("unaligned = %+v", r)
	}
	if r = VerifyDKIM(ctx, zone, plainMsg); !slices.Contains(r.Problems, "no-signature") {
		t.Errorf("unsigned = %+v", r.Problems)
	}
}

func TestVerifyDKIMEd25519AndARC(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	var out bytes.Buffer
	arc := "ARC-Seal: i=1; a=rsa-sha256; cv=none; d=relay.test; s=arc; b=xyz\r\nARC-Authentication-Results: i=1; relay.test; dkim=pass header.d=example.com\r\n"
	if err := dkim.Sign(&out, strings.NewReader(arc+plainMsg), &dkim.SignOptions{Domain: "example.com", Selector: "ed", Signer: priv}); err != nil {
		t.Fatal(err)
	}
	zone := fakeDNS{txt: map[string][]string{"ed._domainkey.example.com": {"v=DKIM1; k=ed25519; p=" + base64.StdEncoding.EncodeToString(pub)}}}
	r := VerifyDKIM(context.Background(), zone, out.String())
	if r.Signatures[0].Result != "pass" || r.Signatures[0].Algorithm != "ed25519-sha256" {
		t.Errorf("ed25519 = %+v", r.Signatures[0])
	}
	if len(r.ARC) != 1 || r.ARC[0].CV != "none" || r.ARC[0].Domain != "relay.test" || !strings.Contains(r.ARC[0].Results, "dkim=pass") {
		t.Errorf("arc = %+v", r.ARC)
	}
}

func TestDKIMVerifyHandler(t *testing.T) {
	signed, zone := sign(t, plainMsg, "example.com", "s1")
	env := tooltest.New(t, &DKIMVerifyTool{res: zone}, tooltest.Options{})
	var r DKIMVerifyResult
	env.Post("/api/v1/email/dkim-verify", map[string]string{"message": signed}).Must(t).Decode(t, &r)
	if len(r.Signatures) != 1 || r.Signatures[0].Result != "pass" {
		t.Errorf("handler = %+v", r)
	}
	env.Post("/api/v1/email/dkim-verify", map[string]string{"message": " "}).ExpectError(t, "INVALID_INPUT")
	env.Post("/api/v1/email/dkim-verify", map[string]string{"message": strings.Repeat("x", MaxDKIMMessage+1)}).ExpectError(t, "INVALID_INPUT")
}

func TestFromDomain(t *testing.T) {
	for in, want := range map[string]string{"Alice <alice@Example.COM>": "example.com", "bob@example.net": "example.net", "no address": ""} {
		if got := fromDomain(in); got != want {
			t.Errorf("fromDomain(%q) = %q", in, got)
		}
	}
	if !aligned("mail.example.com", "example.com") || aligned("example.net", "example.com") {
		t.Error("aligned")
	}
}
