package dnstest

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

// Zone serves the given zone-file records (e.g. "www.example.com. 300 IN A 192.0.2.1"): answers
// with the records matching name and type, follows one CNAME, NODATA for known names with other
// types and NXDOMAIN otherwise.
func Zone(t testing.TB, records ...string) netip.AddrPort {
	t.Helper()
	byName := map[string][]dns.RR{}
	for _, s := range records {
		rr, err := dns.NewRR(s)
		if err != nil {
			t.Fatalf("bad record %q: %v", s, err)
		}
		name := strings.ToLower(rr.Header().Name)
		byName[name] = append(byName[name], rr)
	}
	return Start(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		q := r.Question[0]
		name := strings.ToLower(q.Name)
		rrs, ok := byName[name]
		if !ok {
			m.Rcode = dns.RcodeNameError
			_ = w.WriteMsg(m)
			return
		}
		for _, rr := range rrs {
			switch {
			case rr.Header().Rrtype == q.Qtype || q.Qtype == dns.TypeANY:
				m.Answer = append(m.Answer, rr)
			case rr.Header().Rrtype == dns.TypeCNAME:
				m.Answer = append(m.Answer, rr)
				for _, t := range byName[strings.ToLower(rr.(*dns.CNAME).Target)] {
					if t.Header().Rrtype == q.Qtype {
						m.Answer = append(m.Answer, t)
					}
				}
			}
		}
		_ = w.WriteMsg(m)
	})
}
