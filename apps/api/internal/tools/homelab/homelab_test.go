package homelab

import (
	"context"
	"slices"
	"testing"
)

func TestSortCerts(t *testing.T) {
	d := func(n int) *int { return &n }
	rows := []CertRow{{Host: "a", DaysLeft: d(90)}, {Host: "b"}, {Host: "c", DaysLeft: d(5)}}
	sortCerts(rows)
	got := []string{rows[0].Host, rows[1].Host, rows[2].Host}
	if !slices.Equal(got, []string{"b", "c", "a"}) {
		t.Fatalf("order = %v", got)
	}
}

func TestHostPort(t *testing.T) {
	for in, want := range map[string][2]string{"a.example": {"a.example", "443"}, "https://a.example/": {"a.example", "443"}, "a.example:8443": {"a.example", "8443"}} {
		h, p := hostPort(in, "443")
		if h != want[0] || p != want[1] {
			t.Errorf("%s -> %s:%s", in, h, p)
		}
	}
}

func TestReachClosedPort(t *testing.T) {
	cfg := &Config{}
	cfg.Reach.Targets = []ReachTarget{{Name: "lo", Host: "127.0.0.1", Ports: []int{1}}}
	res := runReach(context.Background(), cfg)
	rows := res["rows"].([]ReachRow)
	if len(rows) != 1 || rows[0].State != "closed" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestDNSCompareValidation(t *testing.T) {
	cfg := &Config{}
	if _, err := runDNSCompare(context.Background(), cfg, "bad name!", "A"); err == nil {
		t.Fatal("expected error for invalid name")
	}
	if _, err := runDNSCompare(context.Background(), cfg, "example.com", "NOPE"); err == nil {
		t.Fatal("expected error for unknown type")
	}
}
