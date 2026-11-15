package dnsx

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync"

	"github.com/miekg/dns"

	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
)

// HostLookup resolves dial-target names through DNS_RESOLVER (A/AAAA over UDP, TCP on truncation).
// When it reports NXDOMAIN or fails, fallback (the container resolver) is asked instead, so LAN
// names such as nas.lan or single-label hosts keep working. With no DNS_RESOLVER it returns
// fallback unchanged.
func HostLookup(fallback target.LookupFunc) target.LookupFunc {
	if !upstream.IsValid() {
		return fallback
	}
	srv := Server{Label: "upstream", Addr: upstream, Transport: UDP}
	return func(ctx context.Context, network, host string) ([]netip.Addr, error) {
		addrs, nx, err := lookupVia(ctx, srv, network, host)
		if err == nil && !nx {
			if len(addrs) == 0 {
				return nil, errors.New("no addresses")
			}
			return addrs, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err() //nolint:wrapcheck // caller cancellation passes through
		}
		return fallback(ctx, network, host)
	}
}

// lookupVia queries the A and/or AAAA records of host. nx is true when every answer was NXDOMAIN.
func lookupVia(ctx context.Context, s Server, network, host string) (addrs []netip.Addr, nx bool, err error) {
	types := map[string][]uint16{"ip4": {dns.TypeA}, "ip6": {dns.TypeAAAA}}[network]
	if types == nil {
		types = []uint16{dns.TypeA, dns.TypeAAAA}
	}
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs []error
		nxN  int
	)
	for _, qt := range types {
		wg.Go(func() {
			m, _, e := Query(ctx, s, host, qt)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case e != nil:
				errs = append(errs, e)
			case m.Rcode == dns.RcodeNameError:
				nxN++
			case m.Rcode != dns.RcodeSuccess:
				errs = append(errs, errors.New(RcodeName(m.Rcode)))
			default:
				addrs = append(addrs, answerAddrs(m, qt)...)
			}
		})
	}
	wg.Wait()
	if len(addrs) > 0 {
		return orderAddrs(addrs, HasIPv6Route()), false, nil
	}
	if nxN == len(types) {
		return nil, true, nil
	}
	return nil, false, errors.Join(errs...)
}

// orderAddrs puts the family the host can reach first, keeping the resolver's order within each
// family, as Go's resolver does (RFC 6724): callers dial the first address, and an IPv6 address in
// a container without IPv6 would fail.
func orderAddrs(addrs []netip.Addr, v6 bool) []netip.Addr {
	out := make([]netip.Addr, 0, len(addrs))
	for _, first := range []bool{v6, !v6} {
		for _, a := range addrs {
			if a.Is6() == first {
				out = append(out, a)
			}
		}
	}
	return out
}

// HasIPv6Route reports whether the host has a route to the global IPv6 internet: a UDP "connect"
// picks a source address without sending a packet. Var for tests.
var HasIPv6Route = func() bool {
	c, err := net.Dial("udp6", "[2001:4860:4860::8888]:53")
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func answerAddrs(m *dns.Msg, qtype uint16) []netip.Addr {
	var out []netip.Addr
	for _, rr := range m.Answer {
		switch v := rr.(type) {
		case *dns.A:
			if qtype == dns.TypeA {
				if a, ok := netip.AddrFromSlice(v.A.To4()); ok {
					out = append(out, a)
				}
			}
		case *dns.AAAA:
			if qtype == dns.TypeAAAA {
				if a, ok := netip.AddrFromSlice(v.AAAA); ok {
					out = append(out, a.Unmap())
				}
			}
		}
	}
	return out
}

// Filtered explains why an answer looks blocked by a DNS filter (Pi-hole, AdGuard Home, a
// resolver with RPZ) rather than being the real data of the name.
type Filtered struct {
	// Reason is "ede" (the resolver said so), "null-address" (0.0.0.0 / ::) or "missing" (NXDOMAIN
	// or no records, while the reference resolver has them).
	Reason string `json:"reason"`
	// EDE is the Extended DNS Error code (RFC 8914: 15 Blocked, 16 Censored, 17 Filtered), 0 if none.
	EDE     int    `json:"ede"`
	EDEText string `json:"edeText"`
	// Reference is the resolver the answer was compared with; empty when no comparison was made.
	Reference      string   `json:"reference"`
	ReferenceRcode string   `json:"referenceRcode"`
	ReferenceData  []string `json:"referenceData"`
}

// filterEDE are the RFC 8914 codes a filtering resolver uses for a policy block.
var filterEDE = map[uint16]bool{dns.ExtendedErrorCodeBlocked: true, dns.ExtendedErrorCodeCensored: true, dns.ExtendedErrorCodeFiltered: true}

// Reference is the unfiltered resolver used to confirm a suspected block: DNS_RESOLVER when set,
// else Cloudflare. Var for tests.
var Reference = func() Server {
	if upstream.IsValid() {
		return Server{Label: "Server resolver (" + upstream.Addr().String() + ")", Addr: upstream, Transport: UDP}
	}
	return Server{Label: "Cloudflare (1.1.1.1)", Addr: netip.MustParseAddrPort("1.1.1.1:53"), Transport: UDP}
}

// CheckFiltered inspects an answer from used and, when it looks like a filter block, asks the
// reference resolver once. It returns nil for normal answers. Only the local (container) resolver
// gets the "missing" comparison: an empty answer from a public resolver is taken at face value.
func CheckFiltered(ctx context.Context, used Server, name string, qtype uint16, m *dns.Msg) *Filtered {
	f := &Filtered{ReferenceData: []string{}}
	if code, text, ok := blockEDE(m); ok {
		f.Reason, f.EDE, f.EDEText = "ede", int(code), text
	}
	switch {
	case f.Reason != "":
	case nullOnly(m):
		f.Reason = "null-address"
	case used.Local && m.Rcode == dns.RcodeNameError, used.Local && m.Rcode == dns.RcodeSuccess && len(answerData(m, qtype)) == 0:
		f.Reason = "missing"
	default:
		return nil
	}
	ref := Reference()
	if ref.Addr == used.Addr {
		if f.Reason == "ede" {
			return f
		}
		return nil
	}
	rm, _, err := Query(ctx, ref, name, qtype)
	if err != nil {
		// Without a comparison only the resolver's own statement counts.
		if f.Reason == "ede" {
			return f
		}
		return nil
	}
	f.Reference, f.ReferenceRcode = ref.Label, RcodeName(rm.Rcode)
	if data := answerData(rm, qtype); len(data) > 0 {
		f.ReferenceData = data
	}
	realAnswer := rm.Rcode == dns.RcodeSuccess && len(f.ReferenceData) > 0 && !nullOnly(rm)
	if f.Reason == "ede" || realAnswer {
		return f
	}
	return nil
}

func blockEDE(m *dns.Msg) (uint16, string, bool) {
	opt := m.IsEdns0()
	if opt == nil {
		return 0, "", false
	}
	for _, o := range opt.Option {
		if e, ok := o.(*dns.EDNS0_EDE); ok && filterEDE[e.InfoCode] {
			return e.InfoCode, strings.TrimSpace(e.ExtraText), true
		}
	}
	return 0, "", false
}

// nullOnly reports an answer whose addresses are all 0.0.0.0 or :: (Pi-hole's default NULL mode).
func nullOnly(m *dns.Msg) bool {
	n := 0
	for _, rr := range m.Answer {
		switch v := rr.(type) {
		case *dns.A:
			if !v.A.IsUnspecified() {
				return false
			}
			n++
		case *dns.AAAA:
			if !v.AAAA.IsUnspecified() {
				return false
			}
			n++
		}
	}
	return n > 0
}

// answerData returns the data of the answer records of qtype (at most 10).
func answerData(m *dns.Msg, qtype uint16) []string {
	var out []string
	for _, r := range Records(m.Answer) {
		if t, _ := ParseType(r.Type); (t == qtype || qtype == dns.TypeANY) && len(out) < 10 {
			out = append(out, r.Data)
		}
	}
	return out
}
