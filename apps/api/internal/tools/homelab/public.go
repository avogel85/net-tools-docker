package homelab

import (
	"context"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

// PublicRow is one public hostname: DNS, certificate and HTTP status.
type PublicRow struct {
	Host      string   `json:"host"`
	IPs       []string `json:"ips"`
	Status    int      `json:"status"`
	FinalURL  string   `json:"finalUrl,omitempty"`
	Redirects int      `json:"redirects"`
	HTTPMs    float64  `json:"httpMs"`
	DaysLeft  *int     `json:"daysLeft"`
	Verified  bool     `json:"verified"`
	State     string   `json:"state"` // ok | degraded | down
	Error     string   `json:"error,omitempty"`
}

func checkPublic(ctx context.Context, host string) PublicRow {
	row := PublicRow{Host: host, IPs: []string{}}
	h, _ := hostPort(host, "443")
	if ips, err := net.DefaultResolver.LookupHost(ctx, h); err == nil {
		row.IPs = ips
	} else {
		row.State, row.Error = "down", "DNS: "+err.Error()
		return row
	}
	cert := checkCert(ctx, host)
	row.DaysLeft, row.Verified = cert.DaysLeft, cert.Verified
	client := &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+"/", nil)
	req.Header.Set("User-Agent", "NetTools-homelab/1")
	start := time.Now()
	resp, err := client.Do(req)
	row.HTTPMs = float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		row.State, row.Error = "down", err.Error()
		return row
	}
	defer resp.Body.Close()
	row.Status, row.FinalURL = resp.StatusCode, resp.Request.URL.String()
	switch {
	case resp.StatusCode >= 500:
		row.State = "down"
	case !row.Verified || (row.DaysLeft != nil && *row.DaysLeft < 14):
		row.State = "degraded"
	default:
		row.State = "ok"
	}
	return row
}

func runPublic(ctx context.Context, hosts []string) map[string]any {
	rows := make([]PublicRow, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Go(func() { rows[i] = checkPublic(ctx, h) })
	}
	wg.Wait()
	rank := map[string]int{"down": 0, "degraded": 1, "ok": 2}
	sort.SliceStable(rows, func(i, j int) bool { return rank[rows[i].State] < rank[rows[j].State] })
	return map[string]any{"rows": rows}
}
