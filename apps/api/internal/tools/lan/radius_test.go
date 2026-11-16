package lan

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // RADIUS
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

// fakeRadius accepts user "alice" with password "secret-pw"; it drops packets with a bad
// Message-Authenticator, as real servers do.
func fakeRadius(t *testing.T, secret string) int {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	go func() {
		buf := make([]byte, 4096)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			req := append([]byte{}, buf[:n]...)
			// Verify Message-Authenticator (last attribute in our requests).
			cp := append([]byte{}, req...)
			clear(cp[n-16:])
			mac := hmac.New(md5.New, []byte(secret))
			mac.Write(cp)
			if !hmac.Equal(mac.Sum(nil), req[n-16:]) {
				continue
			}
			code := byte(radAccessAccept)
			msg := "welcome"
			if req[0] == radAccessRequest {
				var user, pw []byte
				for a := req[20:]; len(a) >= 2 && int(a[1]) <= len(a); a = a[a[1]:] {
					switch a[0] {
					case attrUserName:
						user = a[2:a[1]]
					case attrUserPassword:
						pw = a[2:a[1]]
					}
				}
				want := encryptPassword([]byte("secret-pw"), []byte(secret), req[4:20])
				if string(user) != "alice" || !bytes.Equal(pw, want) {
					code, msg = radAccessReject, "bad credentials"
				}
			}
			resp := append([]byte{code, req[1], 0, 0}, make([]byte, 16)...)
			resp = append(resp, attr(attrReplyMessage, []byte(msg))...)
			binary.BigEndian.PutUint16(resp[2:], uint16(len(resp))) //nolint:gosec // test
			h := md5.New()                                          //nolint:gosec // RADIUS
			h.Write(resp[:4])
			h.Write(req[4:20])
			h.Write(resp[20:])
			h.Write([]byte(secret))
			copy(resp[4:20], h.Sum(nil))
			_, _ = pc.WriteTo(resp, from)
		}
	}()
	return pc.LocalAddr().(*net.UDPAddr).Port
}

func TestRadiusProbe(t *testing.T) {
	RadiusAttempt = 300 * time.Millisecond
	t.Cleanup(func() { RadiusAttempt = 3 * time.Second })
	addr := "127.0.0.1:" + strconv.Itoa(fakeRadius(t, "s3cr3t"))

	if r := RadiusProbe(t.Context(), addr, RadiusRequest{Mode: "status", Secret: "s3cr3t"}); r.Result != "accept" || !r.ResponseAuthOK || r.ReplyMessages[0] != "welcome" {
		t.Errorf("status = %+v", r)
	}
	if r := RadiusProbe(t.Context(), addr, RadiusRequest{Mode: "auth", Secret: "s3cr3t", Username: "alice", Password: "secret-pw"}); r.Result != "accept" || !r.ResponseAuthOK {
		t.Errorf("auth ok = %+v", r)
	}
	if r := RadiusProbe(t.Context(), addr, RadiusRequest{Mode: "auth", Secret: "s3cr3t", Username: "alice", Password: "wrong"}); r.Result != "reject" || r.ReplyMessages[0] != "bad credentials" {
		t.Errorf("auth bad = %+v", r)
	}
	if r := RadiusProbe(t.Context(), addr, RadiusRequest{Mode: "status", Secret: "wrong"}); r.Answered || r.Result != "no-reply" {
		t.Errorf("wrong secret = %+v", r)
	}
	pc, _ := net.ListenPacket("udp", "127.0.0.1:0")
	closed := pc.LocalAddr().String()
	_ = pc.Close()
	if r := RadiusProbe(t.Context(), closed, RadiusRequest{Mode: "status", Secret: "x"}); r.Result != "refused" {
		t.Errorf("closed = %+v", r)
	}
}

func TestEncryptPasswordLong(t *testing.T) {
	if n := len(encryptPassword(make([]byte, 20), []byte("s"), make([]byte, 16))); n != 32 {
		t.Errorf("len = %d", n)
	}
}

func TestRadiusHandler(t *testing.T) {
	port := fakeRadius(t, "s3cr3t")
	env := tooltest.New(t, &RadiusTool{}, tooltest.Options{AllowPrivate: true, Env: map[string]string{"NETTOOLS_LAN": "true"}})
	var r RadiusResult
	env.Post("/api/v1/lan/radius", RadiusRequest{Host: "127.0.0.1", Port: port, Secret: "s3cr3t", Mode: "status"}).Must(t).Decode(t, &r)
	if r.Result != "accept" {
		t.Errorf("result = %+v", r)
	}
	env.Post("/api/v1/lan/radius", RadiusRequest{Host: "127.0.0.1", Mode: "status"}).ExpectError(t, "INVALID_INPUT")
	env.Post("/api/v1/lan/radius", RadiusRequest{Host: "127.0.0.1", Secret: "x", Mode: "auth"}).ExpectError(t, "INVALID_INPUT")
	env.Post("/api/v1/lan/radius", RadiusRequest{Host: "127.0.0.1", Secret: "x", Mode: "coa"}).ExpectError(t, "INVALID_INPUT")
}
