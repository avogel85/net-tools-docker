package homelab

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// ReachRow is one host:port probe.
type ReachRow struct {
	Name   string  `json:"name"`
	Group  string  `json:"group"`
	Host   string  `json:"host"`
	Port   int     `json:"port"`
	State  string  `json:"state"` // open | closed | filtered
	RTTMs  float64 `json:"rttMs"`
	Detail string  `json:"detail,omitempty"`
}

func runReach(ctx context.Context, cfg *Config) map[string]any {
	var rows []ReachRow
	for _, t := range cfg.Reach.Targets {
		for _, p := range t.Ports {
			rows = append(rows, ReachRow{Name: t.Name, Group: t.Group, Host: t.Host, Port: p})
		}
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 32)
	for i := range rows {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			probe(ctx, &rows[i])
		}()
	}
	wg.Wait()
	open := 0
	for _, r := range rows {
		if r.State == "open" {
			open++
		}
	}
	return map[string]any{"rows": rows, "open": open, "total": len(rows)}
}

func probe(ctx context.Context, r *ReachRow) {
	d := net.Dialer{Timeout: 3 * time.Second}
	start := time.Now()
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(r.Host, strconv.Itoa(r.Port)))
	switch {
	case err == nil:
		r.State, r.RTTMs = "open", float64(time.Since(start).Microseconds())/1000
		_ = c.Close()
	case errors.Is(err, syscall.ECONNREFUSED):
		r.State, r.Detail = "closed", "connection refused (host is up)"
	default:
		r.State, r.Detail = "filtered", "no response (timeout, firewall or host down)"
	}
}
