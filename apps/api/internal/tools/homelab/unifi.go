package homelab

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// UniFiConfig points at the UniFi Network Integration API (read-only API key).
type UniFiConfig struct {
	URL     string `json:"url"`     // e.g. https://192.168.1.1
	KeyFile string `json:"keyFile"` // file containing the API key
}

// UniFiClient is one client the UDM knows (wired or wireless).
type UniFiClient struct {
	Name        string `json:"name"`
	IP          string `json:"ip"`
	MAC         string `json:"mac"`
	Type        string `json:"type"`
	ConnectedAt string `json:"connectedAt,omitempty"`
}

// UniFiDevice is a UniFi network device (switch, AP, gateway).
type UniFiDevice struct {
	Name  string `json:"name"`
	IP    string `json:"ip"`
	MAC   string `json:"mac"`
	Model string `json:"model"`
	State string `json:"state"`
}

// UniFiResult is the UniFi part of the inventory.
type UniFiResult struct {
	OK      bool          `json:"ok"`
	Error   string        `json:"error,omitempty"`
	Clients []UniFiClient `json:"clients"`
	Devices []UniFiDevice `json:"devices"`
}

func unifiGet(ctx context.Context, hc *http.Client, base, key, path string, out any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/proxy/network/integration"+path, nil)
	req.Header.Set("X-API-KEY", key)
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func runUniFi(ctx context.Context, c *UniFiConfig) *UniFiResult {
	if c == nil || c.URL == "" {
		return nil
	}
	res := &UniFiResult{Clients: []UniFiClient{}, Devices: []UniFiDevice{}}
	kb, err := os.ReadFile(c.KeyFile)
	if err != nil {
		res.Error = "key file: " + err.Error()
		return res
	}
	key := strings.TrimSpace(string(kb))
	base := strings.TrimSuffix(c.URL, "/")
	hc := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // the UDM uses a self-signed certificate on a trusted LAN
	}}
	var sites struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := unifiGet(ctx, hc, base, key, "/v1/sites", &sites); err != nil || len(sites.Data) == 0 {
		if err == nil {
			err = fmt.Errorf("no sites")
		}
		res.Error = err.Error()
		return res
	}
	site := sites.Data[0].ID
	var clients struct {
		Data []struct {
			Type        string `json:"type"`
			Name        string `json:"name"`
			ConnectedAt string `json:"connectedAt"`
			IP          string `json:"ipAddress"`
			MAC         string `json:"macAddress"`
		} `json:"data"`
	}
	if err := unifiGet(ctx, hc, base, key, "/v1/sites/"+site+"/clients?limit=200", &clients); err != nil {
		res.Error = "clients: " + err.Error()
		return res
	}
	for _, c := range clients.Data {
		res.Clients = append(res.Clients, UniFiClient{Name: c.Name, IP: c.IP, MAC: strings.ToLower(c.MAC), Type: c.Type, ConnectedAt: c.ConnectedAt})
	}
	var devices struct {
		Data []struct {
			Name  string `json:"name"`
			IP    string `json:"ipAddress"`
			MAC   string `json:"macAddress"`
			Model string `json:"model"`
			State string `json:"state"`
		} `json:"data"`
	}
	if err := unifiGet(ctx, hc, base, key, "/v1/sites/"+site+"/devices?limit=200", &devices); err == nil {
		for _, d := range devices.Data {
			res.Devices = append(res.Devices, UniFiDevice{Name: d.Name, IP: d.IP, MAC: strings.ToLower(d.MAC), Model: d.Model, State: d.State})
		}
	}
	res.OK = true
	return res
}
