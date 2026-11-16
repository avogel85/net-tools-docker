// Package voip implements CLI-10: a WebSocket echo for a VoIP-like stream of small packets.
// The browser sends numbered packets at a fixed rate and measures round-trip time, jitter and
// late packets; the server only echoes. WebSocket runs over TCP, so lost packets show up as
// delay (retransmissions), not as loss.
package voip

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
)

// Limits (docs/SECURITY.md).
const (
	// MaxDuration bounds one test; the browser runs at most 20 s.
	MaxDuration = 25 * time.Second
	// MaxPackets is 20 s at 100 packets per second plus slack.
	MaxPackets = 2100
	// MaxPacketBytes is the largest accepted message.
	MaxPacketBytes = 512
	// MinPacketBytes is the 4-byte sequence number plus the 8-byte send time.
	MinPacketBytes = 12
)

// Tool is net.voip.
type Tool struct{}

func (*Tool) ID() string                                { return "net.voip" }
func (*Tool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *Tool) Mount(r chi.Router, d tools.Deps) {
	r.With(d.RateLimit(limits.Heavy), d.Slot).Get("/voip/ws", t.handle)
}

func (t *Tool) handle(w http.ResponseWriter, r *http.Request) {
	// The server's ReadTimeout would end the hijacked connection mid-test.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(MaxDuration + 5*time.Second))
	// Default origin check: only pages served from the same host may connect.
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept has written the HTTP error
	}
	defer func() { _ = c.CloseNow() }()
	c.SetReadLimit(MaxPacketBytes)
	ctx, cancel := context.WithTimeout(r.Context(), MaxDuration)
	defer cancel()
	switch err := Echo(ctx, c); {
	case err == nil:
		_ = c.Close(websocket.StatusNormalClosure, "done")
	case errors.Is(err, errTooMany):
		_ = c.Close(websocket.StatusPolicyViolation, "packet limit reached")
	case errors.Is(err, errBadPacket):
		_ = c.Close(websocket.StatusUnsupportedData, "binary packets of 12 to 512 bytes only")
	case errors.Is(err, context.DeadlineExceeded):
		_ = c.Close(websocket.StatusPolicyViolation, "time limit reached")
	}
}

var (
	errTooMany   = errors.New("too many packets")
	errBadPacket = errors.New("bad packet")
)

// Echo sends every binary message straight back until the client closes or a limit is hit.
func Echo(ctx context.Context, c *websocket.Conn) error {
	for n := 0; ; n++ {
		typ, b, err := c.Read(ctx)
		if err != nil {
			if s := websocket.CloseStatus(err); s == websocket.StatusNormalClosure || s == websocket.StatusGoingAway {
				return nil
			}
			if ctx.Err() != nil {
				return fmt.Errorf("voip: %w", ctx.Err())
			}
			return nil // client went away
		}
		if typ != websocket.MessageBinary || len(b) < MinPacketBytes {
			return errBadPacket
		}
		if n >= MaxPackets {
			return errTooMany
		}
		if err := c.Write(ctx, websocket.MessageBinary, b); err != nil {
			return nil //nolint:nilerr // client went away
		}
	}
}
