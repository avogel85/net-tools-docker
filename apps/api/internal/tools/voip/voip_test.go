package voip_test

import (
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/voip"
)

func serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		c.SetReadLimit(voip.MaxPacketBytes)
		if err := voip.Echo(r.Context(), c); err != nil {
			_ = c.Close(websocket.StatusPolicyViolation, err.Error())
		}
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil) //nolint:bodyclose // websocket
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

func TestEcho(t *testing.T) {
	c := dial(t, serve(t))
	ctx := t.Context()
	for i := range uint32(5) {
		pkt := make([]byte, 172)
		binary.LittleEndian.PutUint32(pkt, i)
		if err := c.Write(ctx, websocket.MessageBinary, pkt); err != nil {
			t.Fatal(err)
		}
		typ, got, err := c.Read(ctx)
		if err != nil || typ != websocket.MessageBinary || binary.LittleEndian.Uint32(got) != i || len(got) != 172 {
			t.Fatalf("echo %d: %v %v %d", i, err, typ, len(got))
		}
	}
	if err := c.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
}

func TestRejects(t *testing.T) {
	tests := []struct {
		name string
		typ  websocket.MessageType
		size int
	}{
		{"text frame", websocket.MessageText, 20},
		{"too short", websocket.MessageBinary, 4},
		{"too large", websocket.MessageBinary, voip.MaxPacketBytes + 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := dial(t, serve(t))
			ctx := t.Context()
			_ = c.Write(ctx, tt.typ, make([]byte, tt.size))
			if _, _, err := c.Read(ctx); err == nil || websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				t.Fatalf("expected the server to close with an error, got %v", err)
			}
		})
	}
}

func TestPacketLimit(t *testing.T) {
	c := dial(t, serve(t))
	ctx := t.Context()
	pkt := make([]byte, voip.MinPacketBytes)
	for i := range voip.MaxPackets + 1 {
		if err := c.Write(ctx, websocket.MessageBinary, pkt); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		if _, _, err := c.Read(ctx); err != nil {
			if i != voip.MaxPackets || websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
				t.Fatalf("packet %d: %v", i, err)
			}
			return
		}
	}
	t.Fatal("limit not enforced")
}
