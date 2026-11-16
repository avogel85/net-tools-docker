package ikeprobe

import (
	"encoding/binary"
	"encoding/hex"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func header(spi []byte, first, version, exchange, flags byte, body []byte) []byte {
	h := make([]byte, 28)
	copy(h, spi)
	copy(h[8:], []byte{1, 2, 3, 4, 5, 6, 7, 8})
	h[16], h[17], h[18], h[19] = first, version, exchange, flags
	binary.BigEndian.PutUint32(h[24:], uint32(28+len(body))) //nolint:gosec // test
	return append(h, body...)
}

func notify(t uint16, data []byte) []byte {
	return append([]byte{0, 0, byte(t >> 8), byte(t)}, data...)
}

// responder answers like a strongSwan/Cisco gateway; mode: "ok", "regroup" or "noprop".
func responder(t *testing.T, mode string, v1 bool) string {
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
			req := buf[:n]
			spi := slices.Clone(req[:8])
			var out []byte
			switch {
			case req[17]>>4 == 2:
				group := binary.BigEndian.Uint16(req[28+int(binary.BigEndian.Uint16(req[30:]))+4:])
				switch {
				case mode == "noprop":
					out = header(spi, plNotify, 0x20, 34, 0x20, payload(0, notify(ntNoProposal, nil)))
				case mode == "regroup" && group != 14:
					out = header(spi, plNotify, 0x20, 34, 0x20, payload(0, notify(ntInvalidKE, []byte{0, 14})))
				default:
					sa := encodeProposal(1, []transform{{tfENCR, 20, 256}, {tfPRF, 5, 0}, {tfDH, group, 0}}, true)
					body := payload(plKE, sa)
					body = append(body, payload(plNonce, append([]byte{byte(group >> 8), byte(group), 0, 0}, make([]byte, 32)...))...)
					body = append(body, payload(plNotify, make([]byte, 32))...)
					body = append(body, payload(plVendor, notify(ntFragmentation, nil))...)
					body = append(body, payload(0, []byte("CISCO-GRE-MODE"))...)
					out = header(spi, plSA, 0x20, 34, 0x20, body)
				}
			case req[17]>>4 == 1 && v1:
				attrs := []byte{0x80, 0x01, 0x00, 0x07, 0x80, 0x0e, 0x01, 0x00, 0x80, 0x02, 0x00, 0x02, 0x80, 0x03, 0x00, 0x01, 0x80, 0x04, 0x00, 0x02}
				tr := append([]byte{0, 0, 0, byte(8 + len(attrs)), 1, 1, 0, 0}, attrs...)
				prop := append([]byte{0, 0, 0, byte(8 + len(tr)), 1, 1, 0, 1}, tr...)
				dpd, _ := hex.DecodeString("afcad71368a1f1c96b8696fc77570100")
				body := append(payload(13, append([]byte{0, 0, 0, 1, 0, 0, 0, 1}, prop...)), payload(0, dpd)...)
				out = header(spi, 1, 0x10, 2, 0, body)
			}
			if out != nil {
				_, _ = pc.WriteTo(out, from)
			}
		}
	}()
	return pc.LocalAddr().String()
}

func TestProbe(t *testing.T) {
	PerAttempt = 300 * time.Millisecond
	t.Cleanup(func() { PerAttempt = 3 * time.Second })
	ctx := t.Context()
	r := Probe(ctx, responder(t, "ok", true), false)
	if !r.IKEv2.Accepted || r.IKEv2.Encryption != "AES-GCM-16-256" || r.IKEv2.PRF != "HMAC-SHA2-256" || r.IKEv2.Group != "Curve25519" || !r.IKEv2.Fragmentation {
		t.Errorf("v2 = %+v", r.IKEv2)
	}
	if !slices.Contains(r.IKEv2.VendorIDs, "CISCO-GRE-MODE") {
		t.Errorf("vids = %v", r.IKEv2.VendorIDs)
	}
	if !r.IKEv1.Accepted || r.IKEv1.Encryption != "AES-256" || r.IKEv1.Hash != "SHA1" || r.IKEv1.Group != "MODP-1024" || !slices.Contains(r.IKEv1.VendorIDs, "Dead Peer Detection (RFC 3706)") {
		t.Errorf("v1 = %+v", r.IKEv1)
	}
	for _, f := range []string{"ikev1-enabled", "ikev1-weak", "weak-group"} {
		if !slices.Contains(r.Findings, f) {
			t.Errorf("findings %v miss %s", r.Findings, f)
		}
	}

	r = Probe(ctx, responder(t, "regroup", false), false)
	if !r.IKEv2.Accepted || r.IKEv2.Group != "MODP-2048" || r.IKEv1.Answered {
		t.Errorf("regroup = %+v", r)
	}
	r = Probe(ctx, responder(t, "noprop", false), false)
	if r.IKEv2.Accepted || !r.IKEv2.NoProposal {
		t.Errorf("noprop = %+v", r.IKEv2)
	}
}

func TestHandler(t *testing.T) {
	env := tooltest.New(t, &Tool{}, tooltest.Options{AllowPrivate: true})
	// Port is fixed (500/4500), so only validation and policy are exercised through HTTP.
	env.Get("/api/v1/ike/probe?host=127.0.0.1&port=22").ExpectError(t, "INVALID_INPUT")
	tooltest.New(t, &Tool{}, tooltest.Options{}).Get("/api/v1/ike/probe?host=10.0.0.1").ExpectError(t, "TARGET_PRIVATE_DENIED")
}

func TestKeyShareSizes(t *testing.T) {
	for g, n := range map[uint16]int{31: 32, 19: 64, 20: 96, 14: 256, 2: 128} {
		b, err := keyShare(g)
		if err != nil || len(b) != n {
			t.Errorf("group %d: %d bytes, %v", g, len(b), err)
		}
	}
}
