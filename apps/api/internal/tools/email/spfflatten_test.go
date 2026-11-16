package email

import (
	"context"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/miekg/dns"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

type fakeAddrDNS struct {
	fakeDNS
	addrs   map[string][]string
	queries *int
}

func (f fakeAddrDNS) TXT(ctx context.Context, name string) ([]string, string, error) {
	if f.queries != nil {
		*f.queries++
	}
	return f.fakeDNS.TXT(ctx, name)
}

func (f fakeAddrDNS) Addrs(_ context.Context, name string, qtype uint16) ([]netip.Addr, string, error) {
	if f.queries != nil {
		*f.queries++
	}
	var out []netip.Addr
	for _, s := range f.addrs[name] {
		a := netip.MustParseAddr(s)
		if a.Is4() == (qtype == dns.TypeA) {
			out = append(out, a)
		}
	}
	return out, "NOERROR", nil
}

func flattenZone() fakeAddrDNS {
	return fakeAddrDNS{
		fakeDNS: fakeDNS{
			txt: map[string][]string{
				"example.com":        {"v=spf1 include:_spf.provider.test mx a:web.example.com/30 ip4:192.0.2.10 -all"},
				"_spf.provider.test": {"v=spf1 include:_a.provider.test include:_b.provider.test ~all"},
				"_a.provider.test":   {"v=spf1 ip4:198.51.100.0/25 ip4:198.51.100.128/25 ip4:192.0.2.0/24 -ip4:203.0.113.1 ~all"},
				"_b.provider.test":   {"v=spf1 ip6:2001:db8::/32 ?all"},
				"opaque.test":        {"v=spf1 include:_x.provider.test ip4:192.0.2.1 exists:%{i}.rbl.test -all"},
				"_x.provider.test":   {"v=spf1 ptr ip4:203.0.113.0/24 -all"},
				"redir.test":         {"v=spf1 redirect=_spf.redir.test"},
				"_spf.redir.test":    {"v=spf1 a ip4:192.0.2.0/24 ~all"},
				"big.test":           {"v=spf1 include:_big.provider.test -all"},
				"_big.provider.test": {bigRecord()},
				"loop.test":          {"v=spf1 include:loop.test -all"},
				"neg.test":           {"v=spf1 -ip4:192.0.2.66 ip4:192.0.2.0/24 ~all"},
				"clash.test":         {"v=spf1 include:_spf1.clash.test include:_big.provider.test -all"},
				"_spf1.clash.test":   {"v=spf1 ip4:203.0.113.0/24"},
				"_spf2.big.test":     {"v=spf1 ip4:192.0.2.1"},
			},
			mx: map[string][]*dns.MX{"example.com": {{Mx: "mx1.example.com."}, {Mx: "mx2.example.com."}}},
		},
		addrs: map[string][]string{
			"mx1.example.com": {"192.0.2.25", "2001:db8:1::25"},
			"mx2.example.com": {"192.0.2.26"},
			"web.example.com": {"192.0.2.40"},
			"_spf.redir.test": {"203.0.113.9"},
		},
	}
}

func bigRecord() string {
	parts := []string{"v=spf1"}
	for i := range 60 {
		parts = append(parts, "ip4:10."+itoa(i*2)+".0.0/16") // gaps: nothing aggregates
	}
	return strings.Join(parts, " ") + " ~all"
}

func itoa(i int) string { return netip.AddrFrom4([4]byte{0, 0, 0, byte(i)}).String()[6:] }

func TestFlattenSPF(t *testing.T) {
	ctx := context.Background()
	r := FlattenSPF(ctx, flattenZone(), "example.com", "_spf", DefaultFlattenSize)
	if len(r.Records) != 1 {
		t.Fatalf("records = %+v", r.Records)
	}
	want := "v=spf1 ip4:192.0.2.0/24 ip4:198.51.100.0/24 ip6:2001:db8::/32 -all"
	if r.Records[0].Value != want || r.Records[0].Name != "example.com" || r.Records[0].Length != len(want) {
		t.Errorf("record = %+v, want %q", r.Records[0], want)
	}
	if r.Lookups != 0 || r.OriginalLookups != 5 || r.IP4 != 2 || r.IP6 != 1 {
		t.Errorf("lookups %d (orig %d), ip4 %d ip6 %d", r.Lookups, r.OriginalLookups, r.IP4, r.IP6)
	}
	if !slices.Contains(r.Warnings, "include-non-pass") {
		t.Errorf("warnings = %v", r.Warnings)
	}
	if len(r.Sources) != 4 || r.Sources[1].Term != "mx" || !slices.Equal(r.Sources[1].Prefixes, []string{"ip4:192.0.2.25", "ip4:192.0.2.26", "ip6:2001:db8:1::25"}) {
		t.Errorf("sources = %+v", r.Sources)
	}
	if !slices.Equal(r.Sources[2].Prefixes, []string{"ip4:192.0.2.40/30"}) {
		t.Errorf("a/30 = %v", r.Sources[2].Prefixes)
	}
}

func TestFlattenSPFKeepsOpaqueTerms(t *testing.T) {
	r := FlattenSPF(context.Background(), flattenZone(), "opaque.test", "_spf", DefaultFlattenSize)
	want := "v=spf1 ip4:192.0.2.1 include:_x.provider.test exists:%{i}.rbl.test -all"
	if len(r.Records) != 1 || r.Records[0].Value != want {
		t.Fatalf("records = %+v, want %q", r.Records, want)
	}
	if r.Lookups != 3 || !r.Sources[0].Kept || r.Sources[1].Kept || !r.Sources[2].Kept {
		t.Errorf("lookups = %d, sources = %+v", r.Lookups, r.Sources)
	}
	for _, w := range []string{"ptr-deprecated", "macro-kept"} {
		if !slices.Contains(r.Warnings, w) {
			t.Errorf("warnings %v miss %s", r.Warnings, w)
		}
	}
}

func TestFlattenSPFRedirect(t *testing.T) {
	r := FlattenSPF(context.Background(), flattenZone(), "redir.test", "_spf", DefaultFlattenSize)
	// a without a host refers to the redirect target; its ~all becomes the policy's all.
	want := "v=spf1 ip4:192.0.2.0/24 ip4:203.0.113.9 ~all"
	if len(r.Records) != 1 || r.Records[0].Value != want {
		t.Fatalf("records = %+v, want %q", r.Records, want)
	}
}

func TestFlattenSPFChains(t *testing.T) {
	r := FlattenSPF(context.Background(), flattenZone(), "big.test", "_spf", 200)
	if len(r.Records) < 3 {
		t.Fatalf("records = %+v", r.Records)
	}
	main := r.Records[0].Value
	if !strings.HasPrefix(main, "v=spf1 include:_spf1.big.test include:_spf2.big.test") || !strings.HasSuffix(main, " -all") {
		t.Errorf("main = %q", main)
	}
	var ips int
	for _, rec := range r.Records[1:] {
		if rec.Length > 200 || !strings.HasPrefix(rec.Value, "v=spf1 ip4:") || strings.Contains(rec.Value, "all") {
			t.Errorf("chunk %+v", rec)
		}
		ips += strings.Count(rec.Value, "ip4:")
	}
	if ips != 60 || r.IP4 != 60 {
		t.Errorf("ip4 terms %d (%d)", ips, r.IP4)
	}
	if r.Lookups != len(r.Records)-1 {
		t.Errorf("lookups = %d for %d records", r.Lookups, len(r.Records))
	}
}

func TestFlattenSPFProblems(t *testing.T) {
	ctx := context.Background()
	if r := FlattenSPF(ctx, flattenZone(), "none.test", "_spf", DefaultFlattenSize); !slices.Contains(r.Problems, "nxdomain") || len(r.Records) != 0 {
		t.Errorf("none = %+v", r)
	}
	if r := FlattenSPF(ctx, flattenZone(), "loop.test", "_spf", DefaultFlattenSize); !slices.Contains(r.Problems, "include-loop") {
		t.Errorf("loop = %+v", r.Problems)
	}
	r := FlattenSPF(ctx, flattenZone(), "neg.test", "_spf", DefaultFlattenSize)
	if r.Records[0].Value != "v=spf1 -ip4:192.0.2.66 ip4:192.0.2.0/24 ~all" || !slices.Contains(r.Warnings, "order-changed") {
		t.Errorf("neg = %+v %v", r.Records, r.Warnings)
	}
}

func TestFlattenSPFChainNames(t *testing.T) {
	ctx := context.Background()
	if r := FlattenSPF(ctx, flattenZone(), "clash.test", "_spf", 200); !slices.Contains(r.Problems, "chain-name-in-use") {
		t.Errorf("clash = %v", r.Problems)
	}
	r := FlattenSPF(ctx, flattenZone(), "big.test", "_spf", 200)
	if !slices.Contains(r.Warnings, "chain-name-exists") || len(r.Problems) != 0 {
		t.Errorf("big = %v %v", r.Problems, r.Warnings)
	}
	if r := FlattenSPF(ctx, flattenZone(), "big.test", "_flat", 200); slices.Contains(r.Warnings, "chain-name-exists") {
		t.Errorf("_flat = %v", r.Warnings)
	}
}

func TestFlattenSPFQueryBudget(t *testing.T) {
	z := flattenZone()
	n := 0
	z.queries = &n
	// 300 includes, each a record of its own: the flattener stops at the budget.
	parts := []string{"v=spf1"}
	for i := range 300 {
		name := "i" + itoa(i) + ".fan.test"
		parts = append(parts, "include:"+name)
		z.txt[name] = []string{"v=spf1 ip4:192.0.2." + itoa(i%250) + " -all"}
	}
	z.txt["fan.test"] = []string{strings.Join(parts, " ") + " -all"}
	n = 0
	r := FlattenSPF(context.Background(), z, "fan.test", "_spf", DefaultFlattenSize)
	if r.Queries > MaxFlattenQueries || !slices.Contains(r.Problems, "flatten-query-limit") {
		t.Errorf("queries = %d, problems = %v", r.Queries, r.Problems)
	}
	// CheckSPF stops at 10 lookups (11 TXT queries); the flattener at MaxFlattenQueries.
	if n > MaxFlattenQueries+12 {
		t.Errorf("%d DNS queries sent", n)
	}
}

func TestAggregatePrefixes(t *testing.T) {
	in := []string{"192.0.2.0/25", "192.0.2.128/25", "192.0.2.7/32", "10.0.0.0/8", "10.1.0.0/16", "2001:db8::/33", "2001:db8:8000::/33", "198.51.100.1/32", "198.51.100.2/32"}
	ps := make([]netip.Prefix, len(in))
	for i, s := range in {
		ps[i] = netip.MustParsePrefix(s)
	}
	var got []string
	for _, p := range AggregatePrefixes(ps) {
		got = append(got, p.String())
	}
	want := []string{"10.0.0.0/8", "192.0.2.0/24", "198.51.100.1/32", "198.51.100.2/32", "2001:db8::/32"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDualCIDR(t *testing.T) {
	for _, c := range []struct {
		value  string
		b4, b6 int
		ok     bool
	}{
		{"host.test", 32, 128, true},
		{"host.test/24", 24, 128, true},
		{"host.test/24//64", 24, 64, true},
		{"//64", 32, 64, true},
		{"/33", 0, 0, false},
		{"host.test//129", 0, 0, false},
	} {
		_, cidr, _ := strings.Cut(c.value, "/")
		b4, b6, ok := dualCIDR(cidr, c.value)
		if ok != c.ok || (ok && (b4 != c.b4 || b6 != c.b6)) {
			t.Errorf("%s = %d %d %v", c.value, b4, b6, ok)
		}
	}
}

func TestSPFFlattenHandler(t *testing.T) {
	env := tooltest.New(t, &SPFFlattenTool{res: flattenZone()}, tooltest.Options{})
	var r SPFFlattenResult
	env.Get("/api/v1/email/spf-flatten?domain=big.test&label=_flat&size=200").Must(t).Decode(t, &r)
	if len(r.Records) < 3 || r.Records[1].Name != "_flat1.big.test" {
		t.Errorf("records = %+v", r.Records)
	}
	env.Get("/api/v1/email/spf-flatten?domain=nodot").ExpectError(t, "INVALID_INPUT")
	env.Get("/api/v1/email/spf-flatten?domain=example.com&label=bad.label").ExpectError(t, "INVALID_INPUT")
	env.Get("/api/v1/email/spf-flatten?domain=example.com&size=50").ExpectError(t, "INVALID_INPUT")
}
