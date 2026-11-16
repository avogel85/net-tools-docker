package email

import (
	"cmp"
	"context"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/miekg/dns"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// Flattening limits. Flattening is only useful for records over 10 lookups, so it may follow more
// than a receiver would, but never more than MaxFlattenQueries DNS queries per request.
const (
	MaxFlattenQueries  = 100
	maxFlattenMXHosts  = 10
	DefaultFlattenSize = 450 // RFC 7208 §3.4: keep the answer within a 512-byte UDP response
	minFlattenSize     = 100
	maxFlattenSize     = 2048
	maxFlattenPrefixes = 2000
)

// addrResolver adds address lookups to resolver (SPF a and mx mechanisms).
type addrResolver interface {
	resolver
	Addrs(ctx context.Context, name string, qtype uint16) ([]netip.Addr, string, error)
}

// Addrs returns the A or AAAA addresses of name.
func (s sysResolver) Addrs(ctx context.Context, name string, qtype uint16) ([]netip.Addr, string, error) {
	m, err := s.query(ctx, name, qtype)
	if err != nil {
		return nil, "", err
	}
	var out []netip.Addr
	for _, rr := range m.Answer {
		switch v := rr.(type) {
		case *dns.A:
			if a, ok := netip.AddrFromSlice(v.A.To4()); ok {
				out = append(out, a)
			}
		case *dns.AAAA:
			if a, ok := netip.AddrFromSlice(v.AAAA); ok {
				out = append(out, a.Unmap())
			}
		}
	}
	return out, "", nil
}

// SPFFlattenTool is EM-14.
type SPFFlattenTool struct {
	deps tools.Deps
	res  addrResolver
}

func (*SPFFlattenTool) ID() string                                { return "email.spfflatten" }
func (*SPFFlattenTool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *SPFFlattenTool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	if t.res == nil {
		t.res = sysResolver{}
	}
	r.With(d.RateLimit(limits.Light), d.Slot, d.Timeout(45*time.Second)).Get("/email/spf-flatten", t.handle)
}

// FlatRecord is one TXT record of the flattened policy.
type FlatRecord struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Length int    `json:"length"`
}

// FlatSource shows what one term of the original record became.
type FlatSource struct {
	Term     string   `json:"term"`
	Kept     bool     `json:"kept"`
	Prefixes []string `json:"prefixes"`
}

// SPFFlattenResult is the response payload.
type SPFFlattenResult struct {
	Domain          string       `json:"domain"`
	Original        string       `json:"original"`
	OriginalLookups int          `json:"originalLookups"`
	Lookups         int          `json:"lookups"`
	Records         []FlatRecord `json:"records"`
	Sources         []FlatSource `json:"sources"`
	IP4             int          `json:"ip4"`
	IP6             int          `json:"ip6"`
	Queries         int          `json:"queries"`
	Problems        []string     `json:"problems"`
	Warnings        []string     `json:"warnings"`
}

type flattener struct {
	ctx      context.Context
	res      addrResolver
	queries  int
	limitHit bool
	problems []string
	warnings []string
	seen     map[string]bool
	visited  map[string]bool // every record name read, for chain-name collisions
}

func addOnce(list *[]string, code string) {
	if !slices.Contains(*list, code) {
		*list = append(*list, code)
	}
}

// budget reserves one DNS query; false once MaxFlattenQueries is used up.
func (f *flattener) budget() bool {
	if f.queries >= MaxFlattenQueries {
		if !f.limitHit {
			f.limitHit = true
			addOnce(&f.problems, "flatten-query-limit")
		}
		return false
	}
	f.queries++
	return true
}

// expansion is the result of flattening one record (or one include).
type expansion struct {
	prefixes []netip.Prefix
	// opaque is set when the record holds something that cannot be turned into addresses (exists,
	// ptr, macros, a missing record): the term that led here is kept as-is.
	opaque  bool
	lookups int // lookups a receiver spends evaluating this record (for kept terms)
}

func (f *flattener) fetch(domain string) (string, string) {
	if !f.budget() {
		return "", "query-limit"
	}
	w := &spfWalker{ctx: f.ctx, res: f.res}
	return w.fetch(domain)
}

// record flattens the pass result of domain's SPF record as seen from an include: only "+"
// mechanisms contribute, all is ignored.
func (f *flattener) record(domain string, depth int) expansion {
	if depth > maxDepth || f.seen[domain] {
		addOnce(&f.problems, "include-loop")
		return expansion{opaque: true}
	}
	f.seen[domain] = true
	f.visited[domain] = true
	defer delete(f.seen, domain)
	txt, ferr := f.fetch(domain)
	if ferr != "" {
		if ferr == "no-record" || ferr == "nxdomain" {
			addOnce(&f.problems, "include-missing")
		}
		return expansion{opaque: true}
	}
	var out expansion
	terms := strings.Fields(txt)[1:]
	hasAll := slices.ContainsFunc(terms, func(raw string) bool { return parseTerm(raw).Kind == "all" })
	for _, raw := range terms {
		term := parseTerm(raw)
		if term.Kind == "redirect" && !hasAll {
			out.lookups++
			sub := f.record(strings.ToLower(term.Value), depth+1)
			out.merge(sub)
			continue
		}
		if !isMechanism(term.Kind) || term.Kind == "all" {
			continue
		}
		if term.Qualifier != "+" {
			// A non-pass mechanism inside an include only stops later mechanisms from matching;
			// dropping it can widen the policy.
			addOnce(&f.warnings, "include-non-pass")
			if usesLookup(term.Kind) {
				out.lookups++
			}
			continue
		}
		out.merge(f.term(domain, term, depth))
	}
	return out
}

func (e *expansion) merge(o expansion) {
	e.prefixes = append(e.prefixes, o.prefixes...)
	e.opaque = e.opaque || o.opaque
	e.lookups += o.lookups
}

func isMechanism(kind string) bool {
	switch kind {
	case "all", "include", "a", "mx", "ptr", "ip4", "ip6", "exists":
		return true
	}
	return false
}

func usesLookup(kind string) bool {
	switch kind {
	case "include", "a", "mx", "ptr", "exists", "redirect":
		return true
	}
	return false
}

// term flattens one pass mechanism of the record of domain.
func (f *flattener) term(domain string, term SPFTerm, depth int) expansion {
	if strings.Contains(term.Value, "%") {
		addOnce(&f.warnings, "macro-kept")
		return expansion{opaque: true, lookups: b2i(usesLookup(term.Kind))}
	}
	switch term.Kind {
	case "ip4", "ip6":
		p, ok := parseIPTerm(term)
		if !ok {
			addOnce(&f.problems, "syntax")
			return expansion{}
		}
		return expansion{prefixes: []netip.Prefix{p}}
	case "include":
		if term.Value == "" {
			addOnce(&f.problems, "syntax")
			return expansion{}
		}
		sub := f.record(strings.ToLower(term.Value), depth+1)
		sub.lookups++
		return sub
	case "a", "mx":
		return f.hostTerm(domain, term)
	case "ptr":
		addOnce(&f.warnings, "ptr-deprecated")
		return expansion{opaque: true, lookups: 1}
	case "exists":
		addOnce(&f.warnings, "exists-kept")
		return expansion{opaque: true, lookups: 1}
	}
	return expansion{}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// hostTerm resolves a[:host][/v4][//v6] or mx[:host][/v4][//v6] to prefixes.
func (f *flattener) hostTerm(domain string, term SPFTerm) expansion {
	out := expansion{lookups: 1}
	host, cidr, _ := strings.Cut(term.Value, "/")
	if host == "" {
		host = domain
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	bits4, bits6, ok := dualCIDR(cidr, term.Value)
	if !ok {
		addOnce(&f.problems, "syntax")
		return expansion{}
	}
	hosts := []string{host}
	if term.Kind == "mx" {
		if !f.budget() {
			out.opaque = true
			return out
		}
		mxs, _, err := f.res.MX(f.ctx, host)
		if err != nil {
			addOnce(&f.problems, "dns-error")
			out.opaque = true
			return out
		}
		hosts = hosts[:0]
		for _, mx := range mxs {
			if len(hosts) == maxFlattenMXHosts {
				addOnce(&f.problems, "too-many-mx")
				break
			}
			hosts = append(hosts, strings.ToLower(strings.TrimSuffix(mx.Mx, ".")))
		}
	}
	for _, h := range hosts {
		for _, qt := range []uint16{dns.TypeA, dns.TypeAAAA} {
			if !f.budget() {
				out.opaque = true
				return out
			}
			addrs, _, err := f.res.Addrs(f.ctx, h, qt)
			if err != nil {
				addOnce(&f.problems, "dns-error")
				out.opaque = true
				return out
			}
			for _, a := range addrs {
				bits := bits6
				if a.Is4() {
					bits = bits4
				}
				if p, err := a.Prefix(bits); err == nil {
					out.prefixes = append(out.prefixes, p)
				}
			}
		}
	}
	return out
}

// dualCIDR parses the "/24//64" suffix of a and mx (cidr is the part after the first "/").
func dualCIDR(cidr, value string) (int, int, bool) {
	bits4, bits6 := 32, 128
	if !strings.Contains(value, "/") {
		return bits4, bits6, true
	}
	v4, v6, dual := strings.Cut(cidr, "//")
	if strings.HasPrefix(cidr, "/") { // "a//64": only an IPv6 length
		v4, v6, dual = "", cidr[1:], true
	}
	var err error
	if v4 != "" {
		if bits4, err = strconv.Atoi(v4); err != nil || bits4 < 0 || bits4 > 32 {
			return 0, 0, false
		}
	}
	if dual {
		if bits6, err = strconv.Atoi(v6); err != nil || bits6 < 0 || bits6 > 128 {
			return 0, 0, false
		}
	}
	return bits4, bits6, true
}

func parseIPTerm(term SPFTerm) (netip.Prefix, bool) {
	v := term.Value
	if !strings.Contains(v, "/") {
		a, err := netip.ParseAddr(v)
		if err != nil {
			return netip.Prefix{}, false
		}
		v += "/" + strconv.Itoa(a.BitLen())
	}
	p, err := netip.ParsePrefix(v)
	if err != nil || p.Addr().Zone() != "" || p.Addr().Is4() != (term.Kind == "ip4") {
		return netip.Prefix{}, false
	}
	return p.Masked(), true
}

// AggregatePrefixes removes duplicates and prefixes covered by others and joins adjacent halves
// into their parent, so the list is as short as possible for the same addresses.
func AggregatePrefixes(in []netip.Prefix) []netip.Prefix {
	ps := make([]netip.Prefix, 0, len(in))
	for _, p := range in {
		p = netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-b2i(p.Addr().Is4In6())*96).Masked()
		if p.IsValid() {
			ps = append(ps, p)
		}
	}
	for {
		slices.SortFunc(ps, func(a, b netip.Prefix) int {
			if c := cmp.Compare(b2i(a.Addr().Is6()), b2i(b.Addr().Is6())); c != 0 {
				return c
			}
			if c := a.Addr().Compare(b.Addr()); c != 0 {
				return c
			}
			return cmp.Compare(a.Bits(), b.Bits())
		})
		out := ps[:0:0]
		for _, p := range ps {
			if n := len(out); n > 0 && out[n-1].Overlaps(p) { // sorted: out[n-1] contains p
				continue
			}
			out = append(out, p)
		}
		merged := false
		for i := 0; i+1 < len(out); i++ {
			a, b := out[i], out[i+1]
			if a.Bits() != b.Bits() || a.Bits() == 0 || a.Addr().Is4() != b.Addr().Is4() {
				continue
			}
			parent, _ := a.Addr().Prefix(a.Bits() - 1)
			if parent.Addr() == a.Addr() && parent.Contains(b.Addr()) {
				out[i] = parent
				out = slices.Delete(out, i+1, i+2)
				merged = true
			}
		}
		ps = out
		if !merged {
			return ps
		}
	}
}

func prefixTerm(p netip.Prefix) string {
	kind := "ip4:"
	if p.Addr().Is6() {
		kind = "ip6:"
	}
	if p.IsSingleIP() {
		return kind + p.Addr().String()
	}
	return kind + p.String()
}

// FlattenSPF builds a flattened policy for domain: lookups are replaced by the addresses they
// resolve to now, terms that cannot be resolved (exists, ptr, macros) are kept, and when the
// result exceeds size bytes the addresses move to chained records <label>N.<domain>.
func FlattenSPF(ctx context.Context, res addrResolver, domain, label string, size int) SPFFlattenResult {
	f := &flattener{ctx: ctx, res: res, seen: map[string]bool{domain: true}, visited: map[string]bool{domain: true}}
	out := SPFFlattenResult{Domain: domain, Records: []FlatRecord{}, Sources: []FlatSource{}, Problems: []string{}, Warnings: []string{}}
	check := CheckSPF(ctx, res, domain)
	out.OriginalLookups = check.Lookups
	if check.Record != nil {
		out.Original = check.Record.Record
	}
	if out.Original == "" {
		code := "no-record"
		if check.Record != nil && check.Record.Error != "" {
			code = check.Record.Error
		}
		out.Problems = append(out.Problems, code)
		return out
	}
	f.queries = 1 // the record itself

	var (
		pass    []netip.Prefix
		front   []string // non-pass mechanisms, kept in front of the addresses
		kept    []string // pass terms kept as-is
		tail    []string // all, exp=
		lookups int
	)
	terms := strings.Fields(out.Original)[1:]
	cur := domain // the record being read; a redirect switches it
	for i := 0; i < len(terms); i++ {
		raw := terms[i]
		term := parseTerm(raw)
		src := FlatSource{Term: raw, Prefixes: []string{}}
		switch {
		case term.Kind == "all":
			tail = append(tail, raw)
			continue
		case term.Kind == "exp":
			tail = append(tail, raw)
			continue
		case term.Kind == "redirect":
			if slices.ContainsFunc(terms, func(r string) bool { return parseTerm(r).Kind == "all" }) {
				continue // ignored when all is present (RFC 7208 §6.1)
			}
			// The redirected record replaces this one: continue with its terms (RFC 7208 §6.1).
			target := strings.ToLower(term.Value)
			ferr := "include-loop"
			txt := ""
			if !f.seen[target] {
				txt, ferr = f.fetch(target)
			}
			if ferr != "" {
				src.Kept = true
				kept = append(kept, raw)
				lookups++
				switch ferr {
				case "no-record", "nxdomain":
					addOnce(&f.problems, "include-missing")
				case "include-loop":
					addOnce(&f.problems, "include-loop")
				}
				out.Sources = append(out.Sources, src)
				continue
			}
			f.seen[target] = true
			f.visited[target] = true
			cur = target
			out.Sources = append(out.Sources, src)
			terms = append(terms[:i+1], strings.Fields(txt)[1:]...)
			continue
		case !isMechanism(term.Kind):
			continue // unknown modifiers are dropped
		case term.Qualifier != "+":
			if usesLookup(term.Kind) {
				lookups++
			}
			front = append(front, raw)
			addOnce(&f.warnings, "order-changed")
			src.Kept = true
			out.Sources = append(out.Sources, src)
			continue
		}
		e := f.term(cur, term, 0)
		if e.opaque {
			src.Kept = true
			kept = append(kept, raw)
			lookups += e.lookups
		} else {
			agg := AggregatePrefixes(e.prefixes)
			for _, p := range agg {
				src.Prefixes = append(src.Prefixes, prefixTerm(p))
			}
			pass = append(pass, e.prefixes...)
		}
		out.Sources = append(out.Sources, src)
	}
	if !slices.ContainsFunc(tail, func(r string) bool { return parseTerm(r).Kind == "all" }) {
		addOnce(&f.warnings, "no-all")
	}

	agg := AggregatePrefixes(pass)
	if len(agg) > maxFlattenPrefixes {
		addOnce(&f.problems, "too-many-prefixes")
		agg = agg[:maxFlattenPrefixes]
	}
	ipTerms := make([]string, len(agg))
	for i, p := range agg {
		ipTerms[i] = prefixTerm(p)
		if p.Addr().Is4() {
			out.IP4++
		} else {
			out.IP6++
		}
	}

	join := func(parts ...[]string) string {
		s := []string{"v=spf1"}
		for _, p := range parts {
			s = append(s, p...)
		}
		return strings.Join(s, " ")
	}
	if single := join(front, ipTerms, kept, tail); len(single) <= size {
		out.Records = append(out.Records, FlatRecord{Name: domain, Value: single})
	} else {
		var chunks []string
		cur := "v=spf1"
		for _, t := range ipTerms {
			if len(cur)+1+len(t) > size && cur != "v=spf1" {
				chunks = append(chunks, cur)
				cur = "v=spf1"
			}
			cur += " " + t
		}
		if cur != "v=spf1" {
			chunks = append(chunks, cur)
		}
		includes := make([]string, len(chunks))
		for i := range chunks {
			includes[i] = "include:" + label + strconv.Itoa(i+1) + "." + domain
		}
		out.Records = append(out.Records, FlatRecord{Name: domain, Value: join(front, includes, kept, tail)})
		for i, c := range chunks {
			out.Records = append(out.Records, FlatRecord{Name: label + strconv.Itoa(i+1) + "." + domain, Value: c})
		}
		lookups += len(chunks)
		f.checkChainNames(out.Records[1:])
	}
	for i := range out.Records {
		out.Records[i].Length = len(out.Records[i].Value)
		if out.Records[i].Length > size {
			addOnce(&f.problems, "record-too-long")
		}
	}
	out.Lookups = lookups
	if lookups > MaxLookups {
		addOnce(&f.problems, "too-many-lookups")
	}
	out.Queries = f.queries
	out.Problems = append(out.Problems, f.problems...)
	out.Warnings = append(out.Warnings, f.warnings...)
	return out
}

// checkChainNames flags chain records that would overwrite a record the policy itself uses (a
// problem) or a TXT record that already exists (a warning: expected when flattening again).
func (f *flattener) checkChainNames(records []FlatRecord) {
	for _, r := range records {
		if f.visited[r.Name] {
			addOnce(&f.problems, "chain-name-in-use")
			continue
		}
		if !f.budget() {
			return
		}
		if txts, _, err := f.res.TXT(f.ctx, r.Name); err == nil && len(txts) > 0 {
			addOnce(&f.warnings, "chain-name-exists")
		}
	}
}

func (t *SPFFlattenTool) handle(w http.ResponseWriter, r *http.Request) {
	domain, ok := domainParam(w, r, "domain")
	if !ok {
		return
	}
	q := params.New(r.URL.Query())
	label := strings.ToLower(q.String("label", "_spf", false, 30))
	size := q.Int("size", DefaultFlattenSize, minFlattenSize, maxFlattenSize)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	if !validLabel(label) {
		httpapi.Error(w, r, httpapi.InvalidInput("label", "use letters, digits, - and _ (for example _spf)"))
		return
	}
	res := FlattenSPF(r.Context(), t.res, domain, label, size)
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", Target: &httpapi.TargetMeta{Input: domain}})
}

func validLabel(s string) bool {
	if s == "" || len(s) > 30 || s[0] == '-' {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}
