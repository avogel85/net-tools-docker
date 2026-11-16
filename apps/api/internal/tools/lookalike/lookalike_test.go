package lookalike

import (
	"net/url"
	"slices"
	"testing"

	"github.com/digibaro/net-tools-docker/apps/api/internal/dnsx/dnstest"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func TestVariants(t *testing.T) {
	v := Variants("example", "com")
	by := map[string]string{}
	for _, c := range v {
		by[c.Domain] = c.Kind
	}
	for d, kind := range map[string]string{
		"exmaple.com": "transposition", "exampe.com": "omission", "exaample.com": "repetition", "exampl3.com": "homoglyph",
		"example.net": "tld", "exam-ple.com": "hyphenation", "wwwexample.com": "prefix", "examplw.com": "replacement",
		"example-login.com": "suffix",
	} {
		if by[d] != kind {
			t.Errorf("%s = %q, want %q", d, by[d], kind)
		}
	}
	if _, ok := by["example.com"]; ok {
		t.Error("the domain itself is not a variant")
	}
	for _, c := range v {
		if !validLabel(c.Domain[:len(c.Domain)-len(".com")]) && c.Kind != "tld" {
			t.Errorf("invalid label %q", c.Domain)
		}
	}
	if idn := v[slices.IndexFunc(v, func(c Candidate) bool { return c.Kind == "idn" })]; idn.Unicode == "" {
		t.Errorf("idn without unicode form: %+v", idn)
	}
}

func TestFind(t *testing.T) {
	addr := dnstest.Zone(t,
		"exmaple.com. 300 IN NS ns1.squatter.test.",
		"exmaple.com. 300 IN A 192.0.2.66",
		"exmaple.com. 300 IN MX 10 mail.squatter.test.",
		"example.net. 300 IN NS ns.example.net.",
	)
	env := tooltest.New(t, &Tool{}, tooltest.Options{AllowPrivate: true})
	var r Result
	env.Get("/api/v1/dns/lookalike?domain=www.Example.com&resolver="+url.QueryEscape(addr.String())).Must(t).Decode(t, &r)
	if r.Domain != "example.com" || r.Generated == 0 || len(r.Candidates) == 0 {
		t.Fatalf("result = %+v", r)
	}
	if len(r.Candidates) > MaxCandidates || (r.Generated > MaxCandidates) != r.Truncated {
		t.Errorf("cap: %d candidates of %d, truncated %v", len(r.Candidates), r.Generated, r.Truncated)
	}
	reg := []string{}
	for _, c := range r.Candidates {
		if c.Registered {
			reg = append(reg, c.Domain)
		}
	}
	slices.Sort(reg)
	if !slices.Equal(reg, []string{"example.net", "exmaple.com"}) {
		t.Errorf("registered = %v", reg)
	}
	first := r.Candidates[0]
	if !first.Registered {
		t.Errorf("registered candidates must come first: %+v", first)
	}
	for _, c := range r.Candidates {
		if c.Domain == "exmaple.com" && (!slices.Equal(c.A, []string{"192.0.2.66"}) || !slices.Equal(c.MX, []string{"mail.squatter.test"})) {
			t.Errorf("details = %+v", c)
		}
	}
}

func TestLookalikeValidation(t *testing.T) {
	env := tooltest.New(t, &Tool{}, tooltest.Options{})
	env.Get("/api/v1/dns/lookalike?domain=nodot").ExpectError(t, "INVALID_INPUT")
	env.Get("/api/v1/dns/lookalike?domain=co.uk").ExpectError(t, "INVALID_INPUT")
	env.Get("/api/v1/dns/lookalike?domain=example.com&resolver=10.1.1.1").ExpectError(t, "TARGET_PRIVATE_DENIED")
}
