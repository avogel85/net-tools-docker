package homelab

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// GuestRow is one VM or container on a Proxmox node.
type GuestRow struct {
	Node   string  `json:"node"`
	VMID   int     `json:"vmid"`
	Type   string  `json:"type"` // lxc | qemu
	Name   string  `json:"name"`
	Status string  `json:"status"`
	IP     string  `json:"ip,omitempty"`
	CPUs   float64 `json:"cpus"`
	MemMB  int     `json:"memMb"`
	Tags   string  `json:"tags,omitempty"`
	Uptime int64   `json:"uptime"`
}

// NodeStatus reports whether a node answered.
type NodeStatus struct {
	Node  string `json:"node"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type pveClient struct {
	base, auth string
	hc         *http.Client
}

func newPVE(n PVENode) (*pveClient, error) {
	b, err := os.ReadFile(n.TokenFile)
	if err != nil {
		return nil, fmt.Errorf("token file: %w", err)
	}
	return &pveClient{
		base: strings.TrimSuffix(n.URL, "/") + "/api2/json",
		auth: "PVEAPIToken=" + strings.TrimSpace(string(b)),
		hc: &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // Proxmox nodes use self-signed certs on a trusted LAN
		}},
	}, nil
}

func (c *pveClient) get(ctx context.Context, path string, out any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	req.Header.Set("Authorization", c.auth)
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return err
	}
	return json.Unmarshal(env.Data, out)
}

func runInventory(ctx context.Context, cfg *Config) map[string]any {
	var mu sync.Mutex
	guests := []GuestRow{}
	statuses := make([]NodeStatus, len(cfg.Proxmox.Nodes))
	var wg sync.WaitGroup
	for i, n := range cfg.Proxmox.Nodes {
		wg.Go(func() {
			st := NodeStatus{Node: n.Name}
			defer func() { statuses[i] = st }()
			c, err := newPVE(n)
			if err != nil {
				st.Error = err.Error()
				return
			}
			var res []struct {
				Type    string  `json:"type"`
				ID      string  `json:"id"`
				VMID    int     `json:"vmid"`
				Name    string  `json:"name"`
				Status  string  `json:"status"`
				MaxCPU  float64 `json:"maxcpu"`
				MaxMem  float64 `json:"maxmem"`
				Tags    string  `json:"tags"`
				Uptime  int64   `json:"uptime"`
				NodeRes string  `json:"node"`
			}
			if err := c.get(ctx, "/cluster/resources?type=vm", &res); err != nil {
				st.Error = err.Error()
				return
			}
			st.OK = true
			var gw sync.WaitGroup
			sem := make(chan struct{}, 6)
			for _, r := range res {
				if r.Type != "lxc" && r.Type != "qemu" {
					continue
				}
				g := GuestRow{Node: n.Name, VMID: r.VMID, Type: r.Type, Name: r.Name, Status: r.Status,
					CPUs: r.MaxCPU, MemMB: int(r.MaxMem / (1 << 20)), Tags: r.Tags, Uptime: r.Uptime}
				nodeName := r.NodeRes
				if nodeName == "" {
					nodeName = n.Name
				}
				gw.Add(1)
				sem <- struct{}{}
				go func() {
					defer gw.Done()
					defer func() { <-sem }()
					if g.Type == "lxc" && g.Status == "running" {
						g.IP = lxcIP(ctx, c, nodeName, g.VMID)
					}
					mu.Lock()
					guests = append(guests, g)
					mu.Unlock()
				}()
			}
			gw.Wait()
		})
	}
	var uni *UniFiResult
	wg.Go(func() { uni = runUniFi(ctx, cfg.UniFi) })
	wg.Wait()
	sort.Slice(guests, func(i, j int) bool {
		if guests[i].Node != guests[j].Node {
			return guests[i].Node < guests[j].Node
		}
		return guests[i].VMID < guests[j].VMID
	})
	return map[string]any{"guests": guests, "nodes": statuses, "unifi": uni}
}

// lxcIP returns the first non-loopback IPv4 address of a running container, or "".
func lxcIP(ctx context.Context, c *pveClient, node string, vmid int) string {
	var ifs []struct {
		Name string `json:"name"`
		Inet string `json:"inet"`
	}
	if err := c.get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/interfaces", node, vmid), &ifs); err != nil {
		return ""
	}
	for _, i := range ifs {
		if i.Name != "lo" && i.Inet != "" {
			return strings.SplitN(i.Inet, "/", 2)[0]
		}
	}
	return ""
}
