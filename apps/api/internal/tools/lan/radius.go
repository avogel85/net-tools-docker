package lan

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // RADIUS (RFC 2865) is defined on MD5
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
)

// RADIUS codes and attributes.
const (
	radAccessRequest   = 1
	radAccessAccept    = 2
	radAccessReject    = 3
	radAccessChallenge = 11
	radStatusServer    = 12
	attrUserName       = 1
	attrUserPassword   = 2
	attrReplyMessage   = 18
	attrNASIdentifier  = 32
	attrMessageAuth    = 80
)

// RadiusAttempt bounds one request; it is sent at most twice. Var for tests.
var RadiusAttempt = 3 * time.Second

// RadiusTool is LAN-10: RADIUS Status-Server and Access-Request (PAP) test.
type RadiusTool struct{ deps tools.Deps }

func (*RadiusTool) ID() string                                { return "lan.radius" }
func (*RadiusTool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.RequireLAN(d) }

func (t *RadiusTool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	// POST: the shared secret and the test password must not appear in URLs or logs.
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(15*time.Second)).Post("/lan/radius", t.handle)
}

// RadiusRequest is the POST body.
type RadiusRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Secret   string `json:"secret"`
	Mode     string `json:"mode"` // status or auth
	Username string `json:"username"`
	Password string `json:"password"`
}

// RadiusResult is the response payload.
type RadiusResult struct {
	Address  string `json:"address"`
	Mode     string `json:"mode"`
	Answered bool   `json:"answered"`
	// Result: accept, reject, challenge, other, no-reply, refused.
	Result        string   `json:"result"`
	ReplyMessages []string `json:"replyMessages"`
	// ResponseAuthOK: the reply was signed with the same shared secret.
	ResponseAuthOK bool    `json:"responseAuthOk"`
	MessageAuth    string  `json:"messageAuth"` // ok, bad or absent
	RTTMs          float64 `json:"rttMs"`
}

func attr(t byte, v []byte) []byte {
	return append([]byte{t, byte((2 + len(v)) & 0xff)}, v...) // values are validated to fit
}

// encryptPassword hides the password as in RFC 2865 §5.2.
func encryptPassword(pw, secret, auth []byte) []byte {
	padded := make([]byte, (len(pw)+15)/16*16)
	if len(padded) == 0 {
		padded = make([]byte, 16)
	}
	copy(padded, pw)
	out := make([]byte, len(padded))
	prev := auth
	for i := 0; i < len(padded); i += 16 {
		h := md5.Sum(append(append([]byte{}, secret...), prev...)) //nolint:gosec // RFC 2865
		for j := range 16 {
			out[i+j] = padded[i+j] ^ h[j]
		}
		prev = out[i : i+16]
	}
	return out
}

// checkReply verifies the Response Authenticator and Message-Authenticator of a reply.
func checkReply(resp, reqAuth, secret []byte, res *RadiusResult) {
	if len(resp) < 20 {
		return
	}
	n := int(binary.BigEndian.Uint16(resp[2:]))
	if n < 20 || n > len(resp) {
		return
	}
	resp = resp[:n]
	h := md5.New() //nolint:gosec // RFC 2865 response authenticator
	h.Write(resp[:4])
	h.Write(reqAuth)
	h.Write(resp[20:])
	h.Write(secret)
	res.ResponseAuthOK = bytes.Equal(h.Sum(nil), resp[4:20])
	res.MessageAuth = "absent"
	for a := resp[20:]; len(a) >= 2; {
		l := int(a[1])
		if l < 2 || l > len(a) {
			break
		}
		switch a[0] {
		case attrReplyMessage:
			res.ReplyMessages = append(res.ReplyMessages, string(a[2:l]))
		case attrMessageAuth:
			if l == 18 {
				off := n - len(a) + 2
				cp := append([]byte{}, resp...)
				copy(cp[4:20], reqAuth)
				clear(cp[off : off+16])
				mac := hmac.New(md5.New, secret)
				mac.Write(cp)
				res.MessageAuth = map[bool]string{true: "ok", false: "bad"}[hmac.Equal(mac.Sum(nil), a[2:18])]
			}
		}
		a = a[l:]
	}
}

// RadiusProbe sends one request to addr and interprets the reply.
func RadiusProbe(ctx context.Context, addr string, req RadiusRequest) RadiusResult {
	res := RadiusResult{Address: addr, Mode: req.Mode, Result: "no-reply", ReplyMessages: []string{}, MessageAuth: "absent"}
	secret := []byte(req.Secret)
	id := make([]byte, 1)
	_, _ = rand.Read(id)
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	var pkt []byte
	if req.Mode == "auth" {
		attrs := attr(attrUserName, []byte(req.Username))
		attrs = append(attrs, attr(attrNASIdentifier, []byte("nettools"))...)
		attrs = append(attrs, attr(attrUserPassword, encryptPassword([]byte(req.Password), secret, auth))...)
		pkt = signRequest(radAccessRequest, id[0], secret, auth, attrs)
	} else {
		pkt = signRequest(radStatusServer, id[0], secret, auth, attr(attrNASIdentifier, []byte("nettools")))
	}
	var d net.Dialer
	c, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		res.Result = "error"
		return res
	}
	defer c.Close()
	buf := make([]byte, 4096)
	for range 2 {
		start := time.Now()
		_ = c.SetDeadline(time.Now().Add(RadiusAttempt))
		if _, err := c.Write(pkt); err != nil {
			break
		}
		n, err := c.Read(buf)
		if err != nil {
			if errors.Is(err, syscall.ECONNREFUSED) {
				res.Result = "refused"
				return res
			}
			continue
		}
		if n < 20 || buf[1] != id[0] {
			continue
		}
		res.RTTMs = float64(time.Since(start).Microseconds()) / 1000
		res.Answered = true
		switch buf[0] {
		case radAccessAccept:
			res.Result = "accept"
		case radAccessReject:
			res.Result = "reject"
		case radAccessChallenge:
			res.Result = "challenge"
		default:
			res.Result = "other"
		}
		checkReply(buf[:n], auth, secret, &res)
		return res
	}
	return res
}

// signRequest builds a request with authenticator auth and signs it with Message-Authenticator.
func signRequest(code, id byte, secret, auth, attrs []byte) []byte {
	pkt := append([]byte{code, id, 0, 0}, auth...)
	pkt = append(pkt, attrs...)
	pkt = append(pkt, attr(attrMessageAuth, make([]byte, 16))...)
	binary.BigEndian.PutUint16(pkt[2:], uint16(len(pkt))) //nolint:gosec // < 4096
	mac := hmac.New(md5.New, secret)
	mac.Write(pkt)
	copy(pkt[len(pkt)-16:], mac.Sum(nil))
	return pkt
}

func (t *RadiusTool) handle(w http.ResponseWriter, r *http.Request) {
	var req RadiusRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		httpapi.Error(w, r, httpapi.InvalidInput("body", "invalid JSON"))
		return
	}
	switch {
	case req.Mode != "status" && req.Mode != "auth":
		httpapi.Error(w, r, httpapi.InvalidInput("mode", "mode is status or auth"))
		return
	case req.Secret == "" || len(req.Secret) > 128:
		httpapi.Error(w, r, httpapi.InvalidInput("secret", "enter the shared secret (at most 128 characters)"))
		return
	case req.Mode == "auth" && (req.Username == "" || len(req.Username) > 253 || len(req.Password) > 128):
		httpapi.Error(w, r, httpapi.InvalidInput("username", "enter a test user name (and a password of at most 128 characters)"))
		return
	case req.Port < 0 || req.Port > 65535:
		httpapi.Error(w, r, httpapi.InvalidInput("port", "1-65535"))
		return
	}
	if req.Port == 0 {
		req.Port = 1812
	}
	tgt, err := t.deps.Target.Resolve(r.Context(), req.Host, target.Opts{Field: "host"})
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	res := RadiusProbe(r.Context(), tgt.AddrPort(req.Port), req)
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", Target: &httpapi.TargetMeta{Input: req.Host, Used: tgt.IP.String() + ":" + strconv.Itoa(req.Port)}})
}
