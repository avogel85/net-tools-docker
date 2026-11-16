package email

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/miekg/dns"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func validateZone() fakeDNS {
	return fakeDNS{
		mx: map[string][]*dns.MX{
			"example.com":           {{Preference: 10, Mx: "mx.example.com."}},
			"gmail.com":             {{Preference: 5, Mx: "gmail-smtp-in.l.google.com."}},
			"nomail.test":           {{Preference: 0, Mx: "."}},
			"mailinator.com":        {{Preference: 10, Mx: "mail.mailinator.com."}},
			"xn--bcher-kva.example": {{Preference: 10, Mx: "mx.example."}},
		},
		a: map[string]int{"implicit.test": 1},
	}
}

func TestValidateAddresses(t *testing.T) {
	in := []string{
		"alice@example.com", "<bob.smith+tag@Example.COM>", "info@example.com", "carol@gmial.com", "x@mailinator.com",
		"d@nomail.test", "e@implicit.test", "f@missing.test", "not-an-address", "a..b@example.com", `"john doe"@example.com`,
		"jörg@bücher.example", strings.Repeat("a", 65) + "@example.com", "g@[192.0.2.1]",
	}
	r := ValidateAddresses(context.Background(), validateZone(), in)
	want := []struct{ verdict, code string }{
		{"valid", ""}, {"valid", ""}, {"risky", "role"}, {"invalid", "typo"}, {"risky", "disposable"},
		{"invalid", "null-mx"}, {"valid", "implicit-mx"}, {"invalid", "no-mail-domain"}, {"invalid", "syntax"}, {"invalid", "local-syntax"}, {"valid", ""},
		{"valid", "smtputf8"}, {"invalid", "local-too-long"}, {"invalid", "address-literal"},
	}
	for i, w := range want {
		c := r[i]
		if c.Verdict != w.verdict || (w.code != "" && !slices.Contains(c.Problems, w.code) && !slices.Contains(c.Warnings, w.code)) {
			t.Errorf("%q: verdict %s problems %v warnings %v, want %s %s", in[i], c.Verdict, c.Problems, c.Warnings, w.verdict, w.code)
		}
	}
	if r[3].Suggestion != "gmail.com" || r[1].Local != "bob.smith+tag" || r[1].Domain != "example.com" {
		t.Errorf("suggestion %q local %q domain %q", r[3].Suggestion, r[1].Local, r[1].Domain)
	}
	if r[11].ASCII != "xn--bcher-kva.example" || !r[11].SMTPUTF8 {
		t.Errorf("idn = %+v", r[11])
	}
	if r[6].MX == nil {
		t.Error("implicit MX must be an empty list, not null")
	}
	if !r[4].Disposable || !r[2].Role || r[0].Free || !slices.Equal(r[0].MX, []string{"mx.example.com"}) {
		t.Errorf("flags: %+v %+v %+v", r[4], r[2], r[0])
	}
}

func TestSuggest(t *testing.T) {
	for in, want := range map[string]string{"gmial.com": "gmail.com", "hotmial.com": "hotmail.com", "gmail.com": "", "example.com": "", "outlok.com": "outlook.com"} {
		if got := suggest(in); got != want {
			t.Errorf("suggest(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateHandler(t *testing.T) {
	env := tooltest.New(t, &ValidateTool{res: validateZone()}, tooltest.Options{})
	var r ValidateResult
	env.Get("/api/v1/email/validate?addresses=alice%40example.com%0Abad").Must(t).Decode(t, &r)
	if len(r.Results) != 2 || r.Results[0].Verdict != "valid" || r.Results[1].Verdict != "invalid" {
		t.Errorf("results = %+v", r.Results)
	}
	env.Get("/api/v1/email/validate?addresses=").ExpectError(t, "INVALID_INPUT")
	env.Get("/api/v1/email/validate?addresses="+strings.Repeat("a%40b.com,", 51)).ExpectError(t, "INVALID_INPUT")
}
