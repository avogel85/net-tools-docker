package webhook

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func TestBins(t *testing.T) {
	store := NewStore()
	env := tooltest.New(t, &BinTool{Store: store}, tooltest.Options{})
	capture := tooltest.New(t, &CaptureTool{Store: store}, tooltest.Options{})

	var info BinInfo
	env.Post("/api/v1/webhook/bins", map[string]any{}).Must(t).Decode(t, &info)
	if len(info.ID) != 24 || len(info.Token) != 32 || info.Path != "/api/v1/hook/"+info.ID {
		t.Fatalf("info = %+v", info)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, info.Path+"/events/github?x=1", strings.NewReader(`{"action":"opened"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	if r := capture.Do(req); r.Raw.Code != http.StatusOK {
		t.Fatalf("capture status %d", r.Raw.Code)
	}
	binReq := httptest.NewRequestWithContext(t.Context(), http.MethodPut, info.Path, strings.NewReader(string([]byte{0xff, 0xfe, 0x00})))
	capture.Do(binReq)

	read := func(token string) tooltest.Response {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/webhook/bins/"+info.ID, nil)
		r.Header.Set("X-Bin-Token", token)
		return env.Do(r)
	}
	var content BinContent
	read(info.Token).Must(t).Decode(t, &content)
	if len(content.Requests) != 2 {
		t.Fatalf("requests = %+v", content.Requests)
	}
	first := content.Requests[0]
	if first.Method != "POST" || first.Path != "/events/github" || first.Query != "x=1" || first.Body != `{"action":"opened"}` || first.ContentType != "application/json" {
		t.Errorf("first = %+v", first)
	}
	found := false
	for _, h := range first.Headers {
		found = found || (h.Name == "X-Github-Event" && h.Value == "issues")
	}
	if !found || !content.Requests[1].Binary || content.Requests[1].Body != "" {
		t.Errorf("headers %v second %+v", first.Headers, content.Requests[1])
	}
	read("wrong").ExpectError(t, "NOT_FOUND")

	// Unknown bins refuse captures.
	if r := capture.Do(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/hook/nope", nil)); r.Raw.Code != http.StatusNotFound {
		t.Errorf("unknown bin status %d", r.Raw.Code)
	}
}

func TestLimits(t *testing.T) {
	store := NewStore()
	now := time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	env := tooltest.New(t, &BinTool{Store: store}, tooltest.Options{})
	for range MaxBinsPerUser {
		env.Post("/api/v1/webhook/bins", map[string]any{}).Must(t)
	}
	env.Post("/api/v1/webhook/bins", map[string]any{}).ExpectError(t, "BUSY")

	// Requests beyond the cap are counted, not stored; bins expire.
	var id string
	for k := range store.bins {
		id = k
	}
	for range MaxRequests + 5 {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader("x"))
		store.Capture(id, r)
	}
	if b := store.bins[id]; len(b.requests) != MaxRequests || b.dropped != 5 {
		t.Errorf("stored %d dropped %d", len(b.requests), b.dropped)
	}
	now = now.Add(BinTTL + time.Second)
	if store.Capture(id, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)) {
		t.Error("expired bin accepted a request")
	}
	env.Post("/api/v1/webhook/bins", map[string]any{}).Must(t) // the expired ones no longer count
}

func TestBodyCap(t *testing.T) {
	store := NewStore()
	b, _ := store.Create(netip.Addr{})
	store.Capture(b.ID, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(strings.Repeat("a", MaxBody+100))))
	if c := store.bins[b.ID].requests[0]; !c.Truncated || len(c.Body) != MaxBody {
		t.Errorf("truncated %v len %d", c.Truncated, len(c.Body))
	}
}
