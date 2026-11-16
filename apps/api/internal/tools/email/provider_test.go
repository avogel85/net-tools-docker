package email

import (
	"context"
	"testing"

	"github.com/miekg/dns"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func providerZone() fakeDNS {
	return fakeDNS{
		txt: map[string][]string{
			"example.com": {
				"v=spf1 include:spf.protection.outlook.com include:sendgrid.net include:_spf.salesforce.com ip4:192.0.2.1 -all",
				"MS=ms12345678",
				"google-site-verification=abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz",
				"atlassian-domain-verification=xyz",
			},
			"_dmarc.example.com": {"v=DMARC1; p=reject; rua=mailto:abc@rua.dmarcian.com,mailto:own@example.com"},
		},
		mx: map[string][]*dns.MX{"example.com": {
			{Preference: 20, Mx: "mx2.example.com."},
			{Preference: 10, Mx: "mx-a.eu.pphosted.com."},
			{Preference: 30, Mx: "example-com.mail.protection.outlook.com."},
		}},
		a: map[string]int{"selector1._domainkey.example.com": 1, "s1._domainkey.example.com": 1},
	}
}

func TestDetectProviders(t *testing.T) {
	r := DetectProviders(context.Background(), providerZone(), "example.com")
	if r.MX[0] != "mx-a.eu.pphosted.com" || len(r.MX) != 3 {
		t.Errorf("mx = %v", r.MX)
	}
	want := map[string]string{
		"Proofpoint|gateway": "MX mx-a.eu.pphosted.com", "Microsoft 365|inbound": "MX example-com.mail.protection.outlook.com",
		"Microsoft 365|sending": "SPF include:spf.protection.outlook.com", "SendGrid|sending": "SPF include:sendgrid.net",
		"Salesforce|sending": "SPF include:_spf.salesforce.com", "Microsoft 365|verification": "TXT MS=ms12345678",
		"Atlassian|verification": "TXT atlassian-domain-verification=xyz", "dmarcian|reporting": "DMARC rua mailto:abc@rua.dmarcian.com",
	}
	got := map[string]string{}
	for _, d := range r.Detections {
		got[d.Provider+"|"+d.Role] = d.Evidence
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if got["Google|verification"] == "" || len(got["Google|verification"]) > 70 {
		t.Errorf("long TXT not shortened: %q", got["Google|verification"])
	}
	if len(r.Selectors) != 2 || r.SPF == "" || r.DMARC == "" {
		t.Errorf("selectors %v spf %q dmarc %q", r.Selectors, r.SPF, r.DMARC)
	}
}

func TestHostMatch(t *testing.T) {
	for _, c := range []struct {
		host, suffix string
		want         bool
	}{
		{"aspmx.l.google.com.", "aspmx.l.google.com", true},
		{"alt1.aspmx.l.google.com", ".aspmx.l.google.com", true},
		{"evilpphosted.com", ".pphosted.com", false},
		{"sendgrid.net", "sendgrid.net", true},
		{"u123.wl.sendgrid.net", "sendgrid.net", true},
		{"notsendgrid.net", "sendgrid.net", false},
	} {
		if got := hostMatch(c.host, c.suffix); got != c.want {
			t.Errorf("hostMatch(%q, %q) = %v", c.host, c.suffix, got)
		}
	}
}

func TestProviderHandler(t *testing.T) {
	env := tooltest.New(t, &ProviderTool{res: providerZone()}, tooltest.Options{})
	var r ProviderResult
	env.Get("/api/v1/email/providers?domain=example.com").Must(t).Decode(t, &r)
	if len(r.Detections) < 8 {
		t.Errorf("detections = %+v", r.Detections)
	}
	env.Get("/api/v1/email/providers?domain=nodot").ExpectError(t, "INVALID_INPUT")
}
