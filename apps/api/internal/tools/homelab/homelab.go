// Package homelab implements the "Homelab" tool category: checks that only make sense on this
// network and are driven by a local JSON config (HOMELAB_CONFIG, default /etc/nettools/homelab.json).
// Nothing here takes a target from the request, so there is no SSRF surface.
package homelab

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
)

// Kind selects which homelab tool a Tool value implements.
type Kind string

const (
	Inventory  Kind = "inventory"
	Reach      Kind = "reach"
	Certs      Kind = "certs"
	DNSCompare Kind = "dns-compare"
	Public     Kind = "public-check"
)

// ReachTarget is one host with the ports that must be reachable from this server.
type ReachTarget struct {
	Name  string `json:"name"`
	Host  string `json:"host"`
	Ports []int  `json:"ports"`
	Group string `json:"group,omitempty"`
}

// Resolver is one DNS server to compare.
type Resolver struct {
	Name string `json:"name"`
	Addr string `json:"addr"` // ip or ip:port
}

// PVENode is one standalone Proxmox node queried with a read-only API token.
type PVENode struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	TokenFile string `json:"tokenFile"` // contains "user@realm!tokenid=secret"
}

// Config is the whole homelab config file.
type Config struct {
	Reach struct {
		Targets []ReachTarget `json:"targets"`
	} `json:"reach"`
	Certs struct {
		Hosts []string `json:"hosts"`
	} `json:"certs"`
	DNS struct {
		Resolvers   []Resolver `json:"resolvers"`
		DefaultName string     `json:"defaultName"`
	} `json:"dns"`
	Public struct {
		Hosts []string `json:"hosts"`
	} `json:"public"`
	Proxmox struct {
		Nodes []PVENode `json:"nodes"`
	} `json:"proxmox"`
	UniFi *UniFiConfig `json:"unifi,omitempty"`
}

func configPath() string {
	if p := os.Getenv("HOMELAB_CONFIG"); p != "" {
		return p
	}
	return "/etc/nettools/homelab.json"
}

// load reads the config on every call (it is tiny) so edits apply without a restart.
func load() (*Config, error) {
	if !strings.EqualFold(os.Getenv("NETTOOLS_LOCAL"), "true") {
		return nil, errors.New("NETTOOLS_LOCAL is not true")
	}
	b, err := os.ReadFile(configPath())
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Tool is one homelab tool.
type Tool struct {
	Kind Kind
	deps tools.Deps
}

func (t *Tool) ID() string { return "homelab." + string(t.Kind) }

func (t *Tool) Enabled(tools.Deps) (bool, tools.Reason) {
	if _, err := load(); err != nil {
		return false, tools.ReasonDisabledByAdmin
	}
	return true, ""
}

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	timeout := 25 * time.Second
	r.With(d.RateLimit(limits.Light), d.Slot, d.Timeout(timeout)).Get("/homelab/"+string(t.Kind), t.handle)
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	cfg, err := load()
	if err != nil {
		httpapi.Error(w, r, httpapi.InvalidInput("homelab", "homelab config not available: "+err.Error()))
		return
	}
	var data any
	switch t.Kind {
	case Inventory:
		data = runInventory(r.Context(), cfg)
	case Reach:
		data = runReach(r.Context(), cfg)
	case Certs:
		data = runCerts(r.Context(), cfg.Certs.Hosts)
	case DNSCompare:
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			name = cfg.DNS.DefaultName
		}
		typ := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type")))
		if typ == "" {
			typ = "A"
		}
		res, err := runDNSCompare(r.Context(), cfg, name, typ)
		if err != nil {
			httpapi.Error(w, r, httpapi.InvalidInput("name", err.Error()))
			return
		}
		data = res
	case Public:
		data = runPublic(r.Context(), cfg.Public.Hosts)
	}
	httpapi.JSON(w, r, data, &httpapi.Meta{Tool: t.ID(), Vantage: "server"})
}
