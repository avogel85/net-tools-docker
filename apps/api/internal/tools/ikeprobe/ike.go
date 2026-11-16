package ikeprobe

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strconv"
	"unicode"
)

// IKEv2 payload types (RFC 7296 §3.2).
const (
	plSA     = 33
	plKE     = 34
	plNonce  = 40
	plNotify = 41
	plVendor = 43
)

// IKEv2 notify types of interest.
const (
	ntNoProposal    = 14
	ntInvalidKE     = 17
	ntCookie        = 16390
	ntFragmentation = 16430
	ntSigHash       = 16431
)

// Transform types and IDs (IANA "IKEv2 Parameters").
const (
	tfENCR  = 1
	tfPRF   = 2
	tfINTEG = 3
	tfDH    = 4
)

var encrNames = map[uint16]string{3: "3DES", 12: "AES-CBC", 13: "AES-CTR", 18: "AES-GCM-8", 19: "AES-GCM-12", 20: "AES-GCM-16", 28: "ChaCha20-Poly1305"}
var prfNames = map[uint16]string{2: "HMAC-SHA1", 5: "HMAC-SHA2-256", 6: "HMAC-SHA2-384", 7: "HMAC-SHA2-512"}
var integNames = map[uint16]string{2: "HMAC-SHA1-96", 12: "HMAC-SHA2-256-128", 13: "HMAC-SHA2-384-192", 14: "HMAC-SHA2-512-256"}

// GroupNames maps Diffie-Hellman group numbers to names.
var GroupNames = map[uint16]string{1: "MODP-768", 2: "MODP-1024", 5: "MODP-1536", 14: "MODP-2048", 15: "MODP-3072", 16: "MODP-4096", 19: "ECP-256", 20: "ECP-384", 21: "ECP-521", 31: "Curve25519", 32: "Curve448"}

// WeakGroups are below current guidance (RFC 8247).
var WeakGroups = map[uint16]bool{1: true, 2: true, 5: true}

// offeredGroups, strongest first; the first is used for the initial key share.
var offeredGroups = []uint16{31, 19, 20, 14, 21, 15, 5, 2}

// keyShare returns a KE payload body for group (a valid public value, so responders accept it).
func keyShare(group uint16) ([]byte, error) {
	var curve ecdh.Curve
	switch group {
	case 31:
		curve = ecdh.X25519()
	case 19:
		curve = ecdh.P256()
	case 20:
		curve = ecdh.P384()
	case 21:
		curve = ecdh.P521()
	case 2, 5, 14, 15:
		size := map[uint16]int{2: 128, 5: 192, 14: 256, 15: 384}[group]
		b := make([]byte, size)
		_, _ = rand.Read(b)
		b[0] &= 0x7f // below the prime
		b[0] |= 0x40
		return b, nil
	default:
		return nil, errors.New("unsupported group")
	}
	k, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err //nolint:wrapcheck // not expected
	}
	pub := k.PublicKey().Bytes()
	if group != 31 {
		pub = pub[1:] // IKEv2 ECP: x || y without the 0x04 prefix (RFC 5903)
	}
	return pub, nil
}

type transform struct {
	typ    byte
	id     uint16
	keyLen uint16
}

func encodeTransform(t transform, last bool) []byte {
	attrs := []byte{}
	if t.keyLen != 0 {
		attrs = binary.BigEndian.AppendUint16([]byte{0x80, 0x0e}, t.keyLen) // Key Length, TV format
	}
	b := make([]byte, 8, 8+len(attrs))
	if !last {
		b[0] = 3
	}
	binary.BigEndian.PutUint16(b[2:], uint16(8+len(attrs))) //nolint:gosec // small
	b[4] = t.typ
	binary.BigEndian.PutUint16(b[6:], t.id)
	return append(b, attrs...)
}

func encodeProposal(num byte, ts []transform, last bool) []byte {
	body := []byte{}
	for i, t := range ts {
		body = append(body, encodeTransform(t, i == len(ts)-1)...)
	}
	b := make([]byte, 8, 8+len(body))
	if !last {
		b[0] = 2
	}
	binary.BigEndian.PutUint16(b[2:], uint16(8+len(body))) //nolint:gosec // small
	b[4], b[5], b[6], b[7] = num, 1, 0, byte(len(ts)&0xff) // protocol IKE, no SPI
	return append(b, body...)
}

func payload(next byte, body []byte) []byte {
	b := make([]byte, 4, 4+len(body))
	b[0] = next
	binary.BigEndian.PutUint16(b[2:], uint16(4+len(body))) //nolint:gosec // small
	return append(b, body...)
}

// SAInit builds an IKEv2 IKE_SA_INIT request with an AEAD and a CBC proposal and a key share for group.
func SAInit(spi []byte, group uint16) ([]byte, error) {
	dh := make([]transform, 0, len(offeredGroups))
	for _, g := range offeredGroups {
		dh = append(dh, transform{tfDH, g, 0})
	}
	p1 := append([]transform{{tfENCR, 20, 256}, {tfENCR, 20, 128}, {tfENCR, 28, 0}, {tfPRF, 5, 0}, {tfPRF, 6, 0}, {tfPRF, 7, 0}}, dh...)
	p2 := append([]transform{{tfENCR, 12, 256}, {tfENCR, 12, 128}, {tfENCR, 3, 0}, {tfPRF, 5, 0}, {tfPRF, 2, 0}, {tfINTEG, 12, 0}, {tfINTEG, 14, 0}, {tfINTEG, 2, 0}}, dh...)
	sa := append(encodeProposal(1, p1, false), encodeProposal(2, p2, true)...)
	ke, err := keyShare(group)
	if err != nil {
		return nil, err
	}
	keBody := append(binary.BigEndian.AppendUint16(nil, group), 0, 0)
	keBody = append(keBody, ke...)
	nonce := make([]byte, 32)
	_, _ = rand.Read(nonce)
	frag := []byte{0, 0, byte(ntFragmentation >> 8), byte(ntFragmentation & 0xff)}
	body := append(payload(plKE, sa), payload(plNonce, keBody)...)
	body = append(body, payload(plNotify, nonce)...)
	body = append(body, payload(0, frag)...)
	h := make([]byte, 28)
	copy(h, spi)
	h[16], h[17], h[18], h[19] = plSA, 0x20, 34, 0x08        // version 2.0, IKE_SA_INIT, initiator
	binary.BigEndian.PutUint32(h[24:], uint32(28+len(body))) //nolint:gosec // small
	return append(h, body...), nil
}

// V2Response is what an IKE_SA_INIT response says.
type V2Response struct {
	Encryption    string
	PRF           string
	Integrity     string
	Group         uint16
	NoProposal    bool
	WantsGroup    uint16
	Cookie        bool
	Fragmentation bool
	SigHash       bool
	VendorIDs     []string
	Notifies      []string
}

// ParseV2 reads an IKEv2 response for the request with SPI spi.
func ParseV2(b []byte, spi []byte) (V2Response, error) {
	var r V2Response
	if len(b) < 28 || string(b[:8]) != string(spi) || b[17]>>4 != 2 {
		return r, errors.New("not an IKEv2 response")
	}
	next := b[16]
	b = b[28:]
	for next != 0 && len(b) >= 4 {
		n := int(binary.BigEndian.Uint16(b[2:]))
		if n < 4 || n > len(b) {
			return r, errors.New("malformed payload")
		}
		body := b[4:n]
		switch next {
		case plSA:
			parseV2SA(body, &r)
		case plKE:
			if len(body) >= 2 {
				r.Group = binary.BigEndian.Uint16(body)
			}
		case plNotify:
			if len(body) >= 4 {
				spiSize := int(body[1])
				t := binary.BigEndian.Uint16(body[2:])
				data := []byte{}
				if 4+spiSize <= len(body) {
					data = body[4+spiSize:]
				}
				switch t {
				case ntNoProposal:
					r.NoProposal = true
				case ntInvalidKE:
					if len(data) >= 2 {
						r.WantsGroup = binary.BigEndian.Uint16(data)
					}
				case ntCookie:
					r.Cookie = true
				case ntFragmentation:
					r.Fragmentation = true
				case ntSigHash:
					r.SigHash = true
				}
				r.Notifies = append(r.Notifies, strconv.Itoa(int(t)))
			}
		case plVendor:
			r.VendorIDs = append(r.VendorIDs, vendorString(body))
		}
		next = b[0]
		b = b[n:]
	}
	return r, nil
}

func parseV2SA(sa []byte, r *V2Response) {
	if len(sa) < 8 {
		return
	}
	n := int(binary.BigEndian.Uint16(sa[2:]))
	if n > len(sa) {
		return
	}
	spiSize := int(sa[6])
	ts := sa[8+spiSize : n]
	for len(ts) >= 8 {
		tl := int(binary.BigEndian.Uint16(ts[2:]))
		if tl < 8 || tl > len(ts) {
			return
		}
		typ, id := ts[4], binary.BigEndian.Uint16(ts[6:])
		keyLen := ""
		if tl >= 12 && binary.BigEndian.Uint16(ts[8:]) == 0x800e {
			keyLen = "-" + strconv.Itoa(int(binary.BigEndian.Uint16(ts[10:])))
		}
		switch typ {
		case tfENCR:
			r.Encryption = name(encrNames, id) + keyLen
		case tfPRF:
			r.PRF = name(prfNames, id)
		case tfINTEG:
			r.Integrity = name(integNames, id)
		case tfDH:
			r.Group = id
		}
		ts = ts[tl:]
	}
}

func name(m map[uint16]string, id uint16) string {
	if n, ok := m[id]; ok {
		return n
	}
	return "#" + strconv.Itoa(int(id))
}

// knownVIDs are well-known vendor IDs (hex).
var knownVIDs = map[string]string{
	"4a131c81070358455c5728f20e95452f": "RFC 3947 NAT-T",
	"afcad71368a1f1c96b8696fc77570100": "Dead Peer Detection (RFC 3706)",
	"4048b7d56ebce88525e7de7f00d6c2d3": "IKE fragmentation (Cisco)",
	"90cb80913ebb696e086381b5ec427b1f": "draft-ietf-ipsec-nat-t-ike-02",
	"7d9419a65310ca6f2c179d9215529d56": "draft-ietf-ipsec-nat-t-ike-03",
	"cd60464335df21f87cfdb2fc68b6a448": "draft-ietf-ipsec-nat-t-ike-02",
	"12f5f28c457168a9702d9fe274cc0100": "Cisco Unity",
	"1e2b516905991c7d7c96fcbfb587e461": "Microsoft (MS NT5 ISAKMPOAKLEY)",
}

// vendorString shows a vendor ID as text when printable (Cisco sends readable IDs), by name when
// well known, else as hex.
func vendorString(b []byte) string {
	h := hex.EncodeToString(b)
	for k, v := range knownVIDs {
		if len(h) >= len(k) && h[:len(k)] == k {
			return v
		}
	}
	printable := len(b) > 4
	for _, c := range string(b) {
		if c > unicode.MaxASCII || !unicode.IsPrint(c) {
			printable = false
			break
		}
	}
	if printable {
		return string(b)
	}
	if len(h) > 64 {
		h = h[:64] + "…"
	}
	return h
}

// --- IKEv1 Main Mode ---------------------------------------------------------------------------

// v1Transforms are offered in the IKEv1 probe: (encryption, key length, hash, group).
var v1Transforms = [][4]uint16{{7, 256, 4, 14}, {7, 256, 2, 14}, {7, 128, 2, 2}, {5, 0, 2, 2}}

var v1Encr = map[uint16]string{1: "DES", 5: "3DES", 7: "AES"}
var v1Hash = map[uint16]string{1: "MD5", 2: "SHA1", 4: "SHA2-256", 5: "SHA2-384", 6: "SHA2-512"}

// MainMode builds the first IKEv1 Main Mode message (PSK proposals plus DPD/NAT-T vendor IDs).
func MainMode(cookie []byte) []byte {
	var transforms []byte
	for i, t := range v1Transforms {
		be := binary.BigEndian.AppendUint16
		attrs := be([]byte{0x80, 0x01}, t[0])
		if t[1] != 0 {
			attrs = be(append(attrs, 0x80, 0x0e), t[1])
		}
		attrs = be(append(attrs, 0x80, 0x02), t[2])
		attrs = append(attrs, 0x80, 0x03, 0x00, 0x01) // PSK
		attrs = be(append(attrs, 0x80, 0x04), t[3])
		attrs = append(attrs, 0x80, 0x0b, 0x00, 0x01, 0x80, 0x0c, 0x70, 0x80) // life 28800 s
		tb := make([]byte, 8, 8+len(attrs))
		if i < len(v1Transforms)-1 {
			tb[0] = 3
		}
		binary.BigEndian.PutUint16(tb[2:], uint16(8+len(attrs))) //nolint:gosec // small
		tb[4], tb[5] = byte(i+1), 1                              // transform number, KEY_IKE
		transforms = append(transforms, append(tb, attrs...)...)
	}
	prop := make([]byte, 8, 8+len(transforms))
	binary.BigEndian.PutUint16(prop[2:], uint16(8+len(transforms))) //nolint:gosec // small
	prop[4], prop[5], prop[6], prop[7] = 1, 1, 0, byte(len(v1Transforms)&0xff)
	prop = append(prop, transforms...)
	saBody := append([]byte{0, 0, 0, 1, 0, 0, 0, 1}, prop...) // DOI IPsec, situation identity-only
	vid1, _ := hex.DecodeString("afcad71368a1f1c96b8696fc77570100")
	vid2, _ := hex.DecodeString("4a131c81070358455c5728f20e95452f")
	body := append(payload(13, saBody), payload(13, vid1)...)
	body = append(body, payload(0, vid2)...)
	h := make([]byte, 28)
	copy(h, cookie)
	h[16], h[17], h[18] = 1, 0x10, 2                         // first payload SA, version 1.0, Identity Protection
	binary.BigEndian.PutUint32(h[24:], uint32(28+len(body))) //nolint:gosec // small
	return append(h, body...)
}

// V1Response is what an IKEv1 Main Mode response says.
type V1Response struct {
	Encryption string
	Hash       string
	Group      uint16
	NoProposal bool
	VendorIDs  []string
}

// ParseV1 reads an IKEv1 response for cookie.
func ParseV1(b []byte, cookie []byte) (V1Response, error) {
	var r V1Response
	if len(b) < 28 || string(b[:8]) != string(cookie) || b[17]>>4 != 1 {
		return r, errors.New("not an IKEv1 response")
	}
	next := b[16]
	b = b[28:]
	for next != 0 && len(b) >= 4 {
		n := int(binary.BigEndian.Uint16(b[2:]))
		if n < 4 || n > len(b) {
			return r, errors.New("malformed payload")
		}
		body := b[4:n]
		switch next {
		case 1: // SA: DOI, situation, proposal, transform
			if len(body) >= 8+8+8 {
				attrs := body[8+8+8:]
				for len(attrs) >= 4 {
					t := binary.BigEndian.Uint16(attrs) & 0x7fff
					v := binary.BigEndian.Uint16(attrs[2:])
					switch t {
					case 1:
						r.Encryption = name(v1Encr, v)
					case 2:
						r.Hash = name(v1Hash, v)
					case 4:
						r.Group = v
					case 14:
						r.Encryption += "-" + strconv.Itoa(int(v))
					}
					attrs = attrs[4:]
				}
			}
		case 11: // Notification
			if len(body) >= 8 && binary.BigEndian.Uint16(body[6:]) == ntNoProposal {
				r.NoProposal = true
			}
		case 13:
			r.VendorIDs = append(r.VendorIDs, vendorString(body))
		}
		next = b[0]
		b = b[n:]
	}
	return r, nil
}
