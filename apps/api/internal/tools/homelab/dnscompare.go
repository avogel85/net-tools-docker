package homelab

import (
	"context"
	"errors"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// DNSRow is one resolver's answer for the compared name.
type DNSRow struct {
	Resolver string   `json:"resolver"`
	Addr     string   `json:"addr"`
	Rcode    string   `json:"rcode"`
	Answers  []string `json:"answers"`
	MinTTL   uint32   `json:"minTtl"`
	AD       bool     `json:"ad"`
	RTTMs    float64  `json:"rttMs"`
	Error    string   `json:"error,omitempty"`
	Differs  bool     `json:"differs"` // answer set differs from the first (reference) resolver
}

func runDNSCompare(ctx context.Context, cfg *Config, name, typ string) (map[string]any, error) {
	qt, ok := dns.StringToType[typ]
	if !ok {
		return nil, errors.New("unknown record type " + typ)
	}
	name = strings.TrimSpace(name)
	if _, valid := dns.IsDomainName(name); name == "" || !valid {
		return nil, errors.New("not a valid domain name")
	}
	if len(cfg.DNS.Resolvers) == 0 {
		return nil, errors.New("no resolvers configured")
	}
	rows := make([]DNSRow, len(cfg.DNS.Resolvers))
	var wg sync.WaitGroup
	for i, rs := range cfg.DNS.Resolvers {
		wg.Go(func() { rows[i] = queryOne(ctx, rs, dns.Fqdn(name), qt) })
	}
	wg.Wait()
	if len(rows) > 0 && rows[0].Error == "" {
		for i := 1; i < len(rows); i++ {
			rows[i].Differs = rows[i].Error == "" && (rows[i].Rcode != rows[0].Rcode || !slices.Equal(rows[i].Answers, rows[0].Answers))
		}
	}
	diff := 0
	for _, r := range rows {
		if r.Differs {
			diff++
		}
	}
	return map[string]any{"name": name, "type": typ, "rows": rows, "differing": diff}, nil
}

func queryOne(ctx context.Context, rs Resolver, fqdn string, qt uint16) DNSRow {
	row := DNSRow{Resolver: rs.Name, Addr: rs.Addr, Answers: []string{}}
	addr := rs.Addr
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "53")
	}
	m := new(dns.Msg)
	m.SetQuestion(fqdn, qt)
	m.RecursionDesired = true
	m.SetEdns0(1232, true) // DO bit, so AD is reported by validating resolvers
	c := &dns.Client{Timeout: 4 * time.Second}
	in, rtt, err := c.ExchangeContext(ctx, m, addr)
	if err == nil && in.Truncated {
		c.Net = "tcp"
		in, rtt, err = c.ExchangeContext(ctx, m, addr)
	}
	if err != nil {
		row.Error = err.Error()
		return row
	}
	row.Rcode, row.AD, row.RTTMs = dns.RcodeToString[in.Rcode], in.AuthenticatedData, float64(rtt.Microseconds())/1000
	for _, rr := range in.Answer {
		if rr.Header().Rrtype == dns.TypeRRSIG {
			continue
		}
		row.Answers = append(row.Answers, strings.TrimPrefix(rr.String(), rr.Header().String()))
		if row.MinTTL == 0 || rr.Header().Ttl < row.MinTTL {
			row.MinTTL = rr.Header().Ttl
		}
	}
	slices.Sort(row.Answers)
	return row
}
