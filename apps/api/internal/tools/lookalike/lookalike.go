// Package lookalike implements DNS-28: typo, homoglyph and TLD variants of a domain and whether
// they are registered (for monitoring look-alike domains of your own brand).
package lookalike

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/miekg/dns"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/digibaro/net-tools-docker/apps/api/internal/dnsx"
	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnslookup"
)

// Limits (docs/SECURITY.md).
const (
	MaxCandidates = 150
	MaxDetails    = 50
	parallel      = 10
)

// TLDs tried for the TLD-swap variants.
var TLDs = []string{"com", "net", "org", "info", "biz", "co", "io", "nl", "be", "de", "eu", "uk", "us", "app", "online", "site", "xyz", "shop", "store", "top"}

var keyboard = map[rune]string{
	'q': "wa", 'w': "qeas", 'e': "wrds", 'r': "etdf", 't': "ryfg", 'y': "tugh", 'u': "yihj", 'i': "uojk", 'o': "ipkl", 'p': "ol",
	'a': "qwsz", 's': "awedxz", 'd': "serfcx", 'f': "drtgvc", 'g': "ftyhbv", 'h': "gyujnb", 'j': "huikmn", 'k': "jiolm", 'l': "kop",
	'z': "asx", 'x': "zsdc", 'c': "xdfv", 'v': "cfgb", 'b': "vghn", 'n': "bhjm", 'm': "njk",
	'1': "2q", '2': "13wq", '3': "24ew", '4': "35re", '5': "46tr", '6': "57yt", '7': "68uy", '8': "79iu", '9': "80oi", '0': "9po",
}

// ASCII look-alikes (both directions where it makes sense).
var asciiGlyphs = [][2]string{{"o", "0"}, {"0", "o"}, {"l", "1"}, {"1", "l"}, {"i", "1"}, {"l", "i"}, {"i", "l"}, {"m", "rn"}, {"rn", "m"}, {"w", "vv"}, {"vv", "w"}, {"d", "cl"}, {"cl", "d"}, {"s", "5"}, {"e", "3"}, {"a", "4"}, {"g", "q"}, {"q", "g"}, {"u", "v"}, {"v", "u"}}

// Unicode homoglyphs (Cyrillic and Greek) for IDN variants.
var idnGlyphs = map[rune]rune{'a': 'а', 'c': 'с', 'e': 'е', 'o': 'о', 'p': 'р', 'x': 'х', 'y': 'у', 'i': 'і', 'j': 'ј', 's': 'ѕ', 'k': 'κ', 'n': 'ո'}

var kindOrder = map[string]int{"tld": 0, "homoglyph": 1, "idn": 2, "transposition": 3, "omission": 4, "repetition": 5, "hyphenation": 6, "prefix": 7, "suffix": 7, "replacement": 8, "bitsquatting": 9}

// Candidate is one variant.
type Candidate struct {
	Domain string `json:"domain"`
	// Unicode is the display form of an IDN variant.
	Unicode    string   `json:"unicode"`
	Kind       string   `json:"kind"`
	Registered bool     `json:"registered"`
	Checked    bool     `json:"checked"`
	A          []string `json:"a"`
	MX         []string `json:"mx"`
	Error      string   `json:"error"`
}

// Result is the response payload.
type Result struct {
	Domain     string      `json:"domain"`
	Resolver   string      `json:"resolver"`
	Generated  int         `json:"generated"`
	Truncated  bool        `json:"truncated"`
	Candidates []Candidate `json:"candidates"`
}

// Tool is the lookalike domain finder.
type Tool struct{ deps tools.Deps }

func (*Tool) ID() string                                { return "dns.lookalike" }
func (*Tool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(60*time.Second)).Get("/dns/lookalike", t.handle)
}

func validLabel(l string) bool {
	if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' || (len(l) > 3 && l[2:4] == "--" && !strings.HasPrefix(l, "xn")) {
		return false
	}
	for _, c := range l {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// Variants generates look-alikes of label (the registrable label, without the suffix).
func Variants(label, suffix string) []Candidate {
	seen := map[string]bool{label + "." + suffix: true}
	var out []Candidate
	add := func(kind, l, suf string) {
		if !validLabel(l) {
			return
		}
		d := l + "." + suf
		if !seen[d] {
			seen[d] = true
			out = append(out, Candidate{Domain: d, Kind: kind})
		}
	}
	r := []rune(label)
	for _, g := range asciiGlyphs {
		for i := 0; ; {
			j := strings.Index(label[i:], g[0])
			if j < 0 {
				break
			}
			add("homoglyph", label[:i+j]+g[1]+label[i+j+len(g[0]):], suffix)
			i += j + 1
		}
	}
	for i := 0; i+1 < len(r); i++ {
		s := slices.Clone(r)
		s[i], s[i+1] = s[i+1], s[i]
		add("transposition", string(s), suffix)
	}
	for i := range r {
		add("omission", string(r[:i])+string(r[i+1:]), suffix)
	}
	for i := range r {
		add("repetition", string(r[:i+1])+string(r[i:]), suffix)
	}
	for i, c := range r {
		for _, k := range keyboard[c] {
			add("replacement", string(r[:i])+string(k)+string(r[i+1:]), suffix)
		}
	}
	for _, tld := range TLDs {
		if tld != suffix {
			add("tld", label, tld)
		}
	}
	for i := 1; i < len(r); i++ {
		add("hyphenation", string(r[:i])+"-"+string(r[i:]), suffix)
	}
	add("prefix", "www"+label, suffix)
	add("prefix", "my"+label, suffix)
	add("suffix", label+"-login", suffix)
	add("suffix", label+"-secure", suffix)
	add("suffix", label+"-support", suffix)
	for i, c := range r {
		for bit := range 8 {
			f := c ^ (1 << bit) // labels are ASCII (validLabel), so c < 128
			if (f >= 'a' && f <= 'z') || (f >= '0' && f <= '9') || f == '-' {
				add("bitsquatting", string(r[:i])+string(f)+string(r[i+1:]), suffix)
			}
		}
	}
	for i, c := range r {
		if h, ok := idnGlyphs[c]; ok {
			u := string(r[:i]) + string(h) + string(r[i+1:])
			a, err := idna.ToASCII(u)
			if err == nil && !seen[a+"."+suffix] {
				seen[a+"."+suffix] = true
				out = append(out, Candidate{Domain: a + "." + suffix, Unicode: u + "." + suffix, Kind: "idn"})
			}
		}
	}
	for i := range out {
		out[i].A, out[i].MX = []string{}, []string{}
	}
	return out
}

func check(ctx context.Context, srv dnsx.Server, c *Candidate, details bool) {
	m, _, err := dnsx.Query(ctx, srv, c.Domain, dns.TypeNS)
	if err != nil {
		c.Error = "dns-error"
		return
	}
	c.Checked = true
	// NOERROR (even without NS in the answer, e.g. a referral answered by a stub) means the name exists.
	c.Registered = m.Rcode == dns.RcodeSuccess
	if !c.Registered || !details {
		return
	}
	if addrs, err := dnsx.Addresses(ctx, srv, c.Domain); err == nil {
		for _, a := range addrs {
			c.A = append(c.A, a.String())
		}
	}
	if mx, _, err := dnsx.Query(ctx, srv, c.Domain, dns.TypeMX); err == nil {
		for _, rr := range mx.Answer {
			if v, ok := rr.(*dns.MX); ok {
				c.MX = append(c.MX, strings.TrimSuffix(strings.ToLower(v.Mx), "."))
			}
		}
	}
}

// Find generates and checks the variants of domain.
func Find(ctx context.Context, srv dnsx.Server, domain string) (Result, error) {
	res := Result{Domain: domain, Resolver: srv.Label, Candidates: []Candidate{}}
	etld1, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return res, httpapi.InvalidInput("domain", "enter a registrable domain such as example.com")
	}
	suffix, _ := publicsuffix.PublicSuffix(etld1)
	label := strings.TrimSuffix(etld1, "."+suffix)
	if strings.Contains(label, ".") || strings.HasPrefix(label, "xn--") {
		return res, httpapi.InvalidInput("domain", "enter an ASCII registrable domain such as example.com")
	}
	res.Domain = etld1
	cands := Variants(label, suffix)
	res.Generated = len(cands)
	// Kinds most often registered by squatters first, so the cap drops the least likely ones.
	slices.SortStableFunc(cands, func(a, b Candidate) int { return kindOrder[a.Kind] - kindOrder[b.Kind] })
	if len(cands) > MaxCandidates {
		cands, res.Truncated = cands[:MaxCandidates], true
	}
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	for i := range cands {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			check(ctx, srv, &cands[i], false)
		})
	}
	wg.Wait()
	n := 0
	for i := range cands {
		if cands[i].Registered && n < MaxDetails {
			n++
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				check(ctx, srv, &cands[i], true)
			})
		}
	}
	wg.Wait()
	slices.SortStableFunc(cands, func(a, b Candidate) int {
		if a.Registered != b.Registered {
			if a.Registered {
				return -1
			}
			return 1
		}
		return 0
	})
	res.Candidates = cands
	return res, nil
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	raw := strings.TrimSpace(q.Get("domain"))
	domain, err := target.NormalizeHostname(raw)
	if err != nil || !strings.Contains(domain, ".") || len(raw) > 253 {
		httpapi.Error(w, r, httpapi.InvalidInput("domain", "enter a domain such as example.com"))
		return
	}
	resolver := q.Get("resolver")
	if len(resolver) > 300 {
		httpapi.Error(w, r, httpapi.InvalidInput("resolver", "too long"))
		return
	}
	srv, err := dnsx.ResolveServer(r.Context(), t.deps.Target, resolver, dnsx.UDP, 0)
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	start := time.Now()
	res, err := Find(r.Context(), srv, domain)
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start)), Target: &httpapi.TargetMeta{Input: raw, Used: srv.Addr.String()}})
}
