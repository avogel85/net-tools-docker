package dnsx

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/miekg/dns"
)

// fakeDNS serves answers from handler on 127.0.0.1 (UDP) and returns its address.
func fakeDNS(t *testing.T, handler dns.HandlerFunc) netip.AddrPort {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &dns.Server{PacketConn: pc, Handler: handler}
	go func() { _ = srv.ActivateAndServe() }()
	t.Cleanup(func() { _ = srv.Shutdown() })
	return netip.MustParseAddrPort(pc.LocalAddr().String())
}

// zone answers A/AAAA/TXT from a map; unknown names are NXDOMAIN.
func zone(records map[string][]string, ede uint16) dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		q := r.Question[0]
		rrs, ok := records[q.Name]
		if !ok {
			m.Rcode = dns.RcodeNameError
		}
		for _, s := range rrs {
			rr, err := dns.NewRR(q.Name + " 60 IN " + s)
			if err == nil && rr.Header().Rrtype == q.Qtype {
				m.Answer = append(m.Answer, rr)
			}
		}
		if ede != 0 {
			m.SetEdns0(1232, false)
			opt := m.IsEdns0()
			opt.Option = append(opt.Option, &dns.EDNS0_EDE{InfoCode: ede, ExtraText: "gravity"})
		}
		_ = w.WriteMsg(m)
	}
}

func withUpstream(t *testing.T, up netip.AddrPort) {
	t.Helper()
	old := upstream
	Configure(up)
	t.Cleanup(func() { Configure(old) })
}

func TestHostLookup(t *testing.T) {
	up := fakeDNS(t, zone(map[string][]string{"example.com.": {"A 93.184.215.14", "AAAA 2606:2800:21f:cb07:6820:80da:af6b:8b2c"}}, 0))
	fallback := func(_ context.Context, _, host string) ([]netip.Addr, error) {
		if host == "nas.lan" {
			return []netip.Addr{netip.MustParseAddr("192.168.1.10")}, nil
		}
		return nil, errors.New("fallback: unknown")
	}

	withUpstream(t, netip.AddrPort{})
	if got, _ := HostLookup(fallback)(context.Background(), "ip", "nas.lan"); len(got) != 1 {
		t.Fatalf("without DNS_RESOLVER the fallback is used directly, got %v", got)
	}

	withUpstream(t, up)
	look := HostLookup(fallback)
	tests := []struct {
		network, host string
		want          int
		wantErr       bool
	}{
		{"ip", "example.com", 2, false},
		{"ip4", "example.com", 1, false},
		{"ip6", "example.com", 1, false},
		{"ip", "nas.lan", 1, false},     // NXDOMAIN upstream → container resolver
		{"ip", "missing.test", 0, true}, // NXDOMAIN on both
	}
	for _, tt := range tests {
		got, err := look(context.Background(), tt.network, tt.host)
		if (err != nil) != tt.wantErr || len(got) != tt.want {
			t.Errorf("%s %s = %v, %v", tt.network, tt.host, got, err)
		}
	}
}

func TestOrderAddrs(t *testing.T) {
	v4a, v6, v4b := netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("192.0.2.2")
	in := []netip.Addr{v6, v4a, v4b}
	if got := orderAddrs(in, false); got[0] != v4a || got[1] != v4b || got[2] != v6 {
		t.Errorf("no IPv6 route: %v", got)
	}
	if got := orderAddrs(in, true); got[0] != v6 || got[1] != v4a {
		t.Errorf("IPv6 route: %v", got)
	}
}

func TestSystemAndLocalServer(t *testing.T) {
	sys := netip.MustParseAddrPort("127.0.0.11:53")
	SystemOverride = &sys
	t.Cleanup(func() { SystemOverride = nil })

	withUpstream(t, netip.AddrPort{})
	s, err := SystemServer(UDP)
	if err != nil || s.Addr != sys || !s.Local || s.Label != "System (127.0.0.11:53)" {
		t.Fatalf("no upstream: %+v, %v", s, err)
	}

	withUpstream(t, netip.MustParseAddrPort("9.9.9.9:53"))
	s, _ = SystemServer(UDP)
	if s.Addr.String() != "9.9.9.9:53" || s.Local || s.Label != "Server resolver (9.9.9.9)" {
		t.Errorf("upstream: %+v", s)
	}
	l, err := ResolveServer(context.Background(), nil, "local", UDP, 0)
	if err != nil || l.Addr != sys || !l.Local || l.Label != "Local resolver (127.0.0.11:53)" {
		t.Errorf("local: %+v, %v", l, err)
	}
	if _, err := ResolveServer(context.Background(), nil, "local", DoT, 0); err == nil {
		t.Error("local over DoT must be refused")
	}
}

func TestCheckFiltered(t *testing.T) {
	known := map[string][]string{"ads.example.": {"A 192.0.2.1", "TXT \"v=spf1 -all\""}, "gone.example.": nil}
	ref := fakeDNS(t, zone(known, 0))
	old := Reference
	Reference = func() Server { return Server{Label: "Reference", Addr: ref, Transport: UDP} }
	t.Cleanup(func() { Reference = old })

	nullSrv := fakeDNS(t, zone(map[string][]string{"ads.example.": {"A 0.0.0.0"}, "ok.example.": {"A 192.0.2.9"}}, 0))
	edeSrv := fakeDNS(t, zone(map[string][]string{}, dns.ExtendedErrorCodeBlocked))
	nxSrv := fakeDNS(t, zone(map[string][]string{"ads.example.": {}}, 0))

	tests := []struct {
		name    string
		srv     Server
		qname   string
		qtype   uint16
		reason  string // "" = not filtered
		refData int
	}{
		{"null address", Server{Addr: nullSrv, Transport: UDP}, "ads.example", dns.TypeA, "null-address", 1},
		{"normal answer", Server{Addr: nullSrv, Transport: UDP}, "ok.example", dns.TypeA, "", 0},
		{"ede blocked", Server{Addr: edeSrv, Transport: UDP}, "ads.example", dns.TypeA, "ede", 1},
		{"ede even if reference also empty", Server{Addr: edeSrv, Transport: UDP}, "gone.example", dns.TypeA, "ede", 0},
		{"local nodata, reference has TXT", Server{Addr: nxSrv, Transport: UDP, Local: true}, "ads.example", dns.TypeTXT, "missing", 1},
		{"local nxdomain, reference agrees", Server{Addr: nxSrv, Transport: UDP, Local: true}, "gone.example", dns.TypeA, "", 0},
		{"public nodata is not compared", Server{Addr: nxSrv, Transport: UDP}, "ads.example", dns.TypeTXT, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _, err := Query(context.Background(), tt.srv, tt.qname, tt.qtype)
			if err != nil {
				t.Fatal(err)
			}
			f := CheckFiltered(context.Background(), tt.srv, tt.qname, tt.qtype, m)
			if tt.reason == "" {
				if f != nil {
					t.Fatalf("unexpected %+v", f)
				}
				return
			}
			if f == nil || f.Reason != tt.reason || len(f.ReferenceData) != tt.refData {
				t.Fatalf("got %+v, want reason %q with %d reference records", f, tt.reason, tt.refData)
			}
			if tt.reason == "ede" && (f.EDE != 15 || f.EDEText != "gravity") {
				t.Errorf("EDE = %d %q", f.EDE, f.EDEText)
			}
		})
	}
}
