package homelab

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// CertRow is one host's certificate state.
type CertRow struct {
	Host      string `json:"host"`
	NotAfter  string `json:"notAfter,omitempty"`
	DaysLeft  *int   `json:"daysLeft"`
	Issuer    string `json:"issuer,omitempty"`
	Subject   string `json:"subject,omitempty"`
	Verified  bool   `json:"verified"`
	VerifyErr string `json:"verifyError,omitempty"`
	Error     string `json:"error,omitempty"`
}

func hostPort(h string, def string) (string, string) {
	h = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(h), "https://"), "http://")
	h = strings.TrimSuffix(h, "/")
	if host, port, err := net.SplitHostPort(h); err == nil {
		return host, port
	}
	return h, def
}

func checkCert(ctx context.Context, raw string) CertRow {
	host, port := hostPort(raw, "443")
	row := CertRow{Host: raw}
	d := tls.Dialer{NetDialer: &net.Dialer{Timeout: 5 * time.Second}, Config: &tls.Config{ServerName: host, InsecureSkipVerify: true}} //nolint:gosec // read the cert, verify below
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		row.Error = err.Error()
		return row
	}
	defer c.Close()
	certs := c.(*tls.Conn).ConnectionState().PeerCertificates
	if len(certs) == 0 {
		row.Error = "no certificate presented"
		return row
	}
	leaf := certs[0]
	days := int(time.Until(leaf.NotAfter).Hours() / 24)
	row.DaysLeft, row.NotAfter = &days, leaf.NotAfter.UTC().Format(time.RFC3339)
	row.Issuer, row.Subject = leaf.Issuer.CommonName, leaf.Subject.CommonName
	inter := x509.NewCertPool()
	for _, ic := range certs[1:] {
		inter.AddCert(ic)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: host, Intermediates: inter}); err != nil {
		row.VerifyErr = err.Error()
	} else {
		row.Verified = true
	}
	return row
}

func runCerts(ctx context.Context, hosts []string) map[string]any {
	rows := make([]CertRow, len(hosts))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for i, h := range hosts {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			rows[i] = checkCert(ctx, h)
		}()
	}
	wg.Wait()
	sortCerts(rows)
	return map[string]any{"rows": rows}
}

// sortCerts puts errors first, then the fewest days left.
func sortCerts(rows []CertRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if (a.DaysLeft == nil) != (b.DaysLeft == nil) {
			return a.DaysLeft == nil
		}
		if a.DaysLeft == nil {
			return a.Host < b.Host
		}
		return *a.DaysLeft < *b.DaysLeft
	})
}
