package email

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/go-chi/chi/v5"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnslookup"
)

// DKIM verifier limits.
const (
	MaxDKIMMessage    = 900 << 10
	MaxDKIMSignatures = 10
)

// DKIMVerifyTool is EM-18: verify the DKIM signatures of a pasted message. POST, because a
// message is personal data and must stay out of URLs and access logs; nothing is stored.
type DKIMVerifyTool struct {
	deps tools.Deps
	res  resolver
}

func (*DKIMVerifyTool) ID() string                                { return "email.dkimverify" }
func (*DKIMVerifyTool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *DKIMVerifyTool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	if t.res == nil {
		t.res = sysResolver{}
	}
	r.With(d.RateLimit(limits.Light), d.Slot, d.Timeout(30*time.Second)).Post("/email/dkim-verify", t.handle)
}

// SignatureCheck is the result for one DKIM-Signature header.
type SignatureCheck struct {
	Domain      string   `json:"domain"`
	Selector    string   `json:"selector"`
	Algorithm   string   `json:"algorithm"`
	Canon       string   `json:"canon"`
	Identifier  string   `json:"identifier"`
	Headers     []string `json:"headers"`
	BodyLength  bool     `json:"bodyLength"`
	Signed      string   `json:"signed"`
	Expires     string   `json:"expires"`
	Result      string   `json:"result"`
	Error       string   `json:"error"`
	KeyRecord   string   `json:"keyRecord"`
	FromAligned bool     `json:"fromAligned"`
	Warnings    []string `json:"warnings"`
}

// ARCSet is one ARC instance (results only; ARC seals are not verified cryptographically).
type ARCSet struct {
	Instance string `json:"instance"`
	CV       string `json:"cv"`
	Domain   string `json:"domain"`
	Results  string `json:"results"`
}

// DKIMVerifyResult is the response payload.
type DKIMVerifyResult struct {
	From       string           `json:"from"`
	Subject    string           `json:"subject"`
	Signatures []SignatureCheck `json:"signatures"`
	// AuthResults are the Authentication-Results headers the receiving servers added.
	AuthResults []string `json:"authResults"`
	ARC         []ARCSet `json:"arc"`
	Problems    []string `json:"problems"`
}

func tagValue(v string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(v, ";") {
		k, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(k))] = strings.Join(strings.Fields(val), "")
	}
	return out
}

// fromDomain returns the domain of the first address in a From header.
func fromDomain(from string) string {
	at := strings.LastIndex(from, "@")
	if at < 0 {
		return ""
	}
	d := from[at+1:]
	if i := strings.IndexAny(d, "> \t\r\n,;"); i >= 0 {
		d = d[:i]
	}
	return strings.ToLower(strings.TrimSuffix(d, "."))
}

// aligned: relaxed DMARC alignment (same organizational domain, approximated by the last two labels
// or one being a subdomain of the other).
func aligned(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a) {
		return true
	}
	tail := func(s string) string {
		l := strings.Split(s, ".")
		if len(l) < 2 {
			return s
		}
		return strings.Join(l[len(l)-2:], ".")
	}
	return tail(a) == tail(b)
}

// VerifyDKIM checks every DKIM signature of the raw message.
func VerifyDKIM(ctx context.Context, res resolver, raw string) DKIMVerifyResult {
	out := DKIMVerifyResult{Signatures: []SignatureCheck{}, AuthResults: []string{}, ARC: []ARCSet{}, Problems: []string{}}
	// Messages pasted from a browser often have bare LF line endings; DKIM needs CRLF.
	msg := strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\n", "\r\n")
	hdr, err := textproto.NewReader(bufio.NewReader(strings.NewReader(msg))).ReadMIMEHeader()
	if err != nil && len(hdr) == 0 {
		out.Problems = append(out.Problems, "not-a-message")
		return out
	}
	out.From, out.Subject = hdr.Get("From"), hdr.Get("Subject")
	out.AuthResults = append(out.AuthResults, hdr.Values("Authentication-Results")...)
	seals := map[string]*ARCSet{}
	for _, s := range hdr.Values("Arc-Seal") {
		tv := tagValue(s)
		seals[tv["i"]] = &ARCSet{Instance: tv["i"], CV: tv["cv"], Domain: tv["d"]}
	}
	for _, a := range hdr.Values("Arc-Authentication-Results") {
		i, rest, _ := strings.Cut(a, ";")
		inst := strings.TrimPrefix(strings.TrimSpace(i), "i=")
		if s, ok := seals[inst]; ok {
			s.Results = strings.TrimSpace(rest)
		}
	}
	for i := 1; i <= 50; i++ {
		if s, ok := seals[strconv.Itoa(i)]; ok {
			out.ARC = append(out.ARC, *s)
		}
	}

	sigs := hdr.Values("Dkim-Signature")
	if len(sigs) == 0 {
		out.Problems = append(out.Problems, "no-signature")
		return out
	}
	// Key records, cached so each selector is fetched once and shown in the result.
	var mu sync.Mutex
	keys := map[string]string{}
	lookup := func(name string) ([]string, error) {
		txts, rcode, err := res.TXT(ctx, name)
		if err != nil {
			return nil, err //nolint:wrapcheck // go-msgauth reports it as a temporary error
		}
		if rcode == "NXDOMAIN" || len(txts) == 0 {
			return nil, errors.New("no key record at " + name)
		}
		mu.Lock()
		keys[strings.ToLower(name)] = strings.Join(txts, " ")
		mu.Unlock()
		return txts, nil
	}
	vs, verr := dkim.VerifyWithOptions(strings.NewReader(msg), &dkim.VerifyOptions{LookupTXT: lookup, MaxVerifications: MaxDKIMSignatures})
	if errors.Is(verr, dkim.ErrTooManySignatures) {
		out.Problems = append(out.Problems, "too-many-signatures")
	} else if verr != nil && len(vs) == 0 {
		out.Problems = append(out.Problems, "not-a-message")
		return out
	}
	from := fromDomain(out.From)
	for i, v := range vs {
		if v == nil {
			continue
		}
		c := SignatureCheck{Domain: v.Domain, Identifier: v.Identifier, Headers: v.HeaderKeys, Warnings: []string{}}
		if c.Headers == nil {
			c.Headers = []string{}
		}
		if i < len(sigs) {
			tv := tagValue(sigs[i])
			c.Selector, c.Algorithm, c.Canon = tv["s"], tv["a"], tv["c"]
			if c.Canon == "" {
				c.Canon = "simple/simple"
			}
			c.BodyLength = tv["l"] != ""
			if c.Domain == "" {
				c.Domain = tv["d"]
			}
		}
		if !v.Time.IsZero() {
			c.Signed = v.Time.UTC().Format(time.RFC3339)
		}
		if !v.Expiration.IsZero() {
			c.Expires = v.Expiration.UTC().Format(time.RFC3339)
		}
		switch {
		case v.Err == nil:
			c.Result = "pass"
		case dkim.IsTempFail(v.Err):
			c.Result, c.Error = "temperror", v.Err.Error()
		case dkim.IsPermFail(v.Err):
			c.Result, c.Error = "permerror", v.Err.Error()
		default:
			c.Result, c.Error = "fail", v.Err.Error()
		}
		if strings.Contains(c.Error, "body hash did not verify") {
			c.Result = "fail"
			c.Warnings = append(c.Warnings, "body-changed")
		}
		c.KeyRecord = keys[strings.ToLower(c.Selector+"._domainkey."+c.Domain)]
		c.FromAligned = aligned(strings.ToLower(c.Domain), from)
		if c.BodyLength {
			c.Warnings = append(c.Warnings, "body-length")
		}
		if c.Algorithm == "rsa-sha1" {
			c.Warnings = append(c.Warnings, "sha1")
		}
		hasFrom := false
		for _, h := range c.Headers {
			hasFrom = hasFrom || strings.EqualFold(h, "from")
		}
		if !hasFrom {
			c.Warnings = append(c.Warnings, "from-not-signed")
		}
		out.Signatures = append(out.Signatures, c)
	}
	if !anyAlignedPass(out.Signatures) {
		out.Problems = append(out.Problems, "no-aligned-pass")
	}
	return out
}

func anyAlignedPass(s []SignatureCheck) bool {
	for _, c := range s {
		if c.Result == "pass" && c.FromAligned {
			return true
		}
	}
	return false
}

func (t *DKIMVerifyTool) handle(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxDKIMMessage+4096)).Decode(&in); err != nil || strings.TrimSpace(in.Message) == "" || len(in.Message) > MaxDKIMMessage {
		httpapi.Error(w, r, httpapi.InvalidInput("message", `expected JSON {"message": "<raw message>"} of at most 900 KB`))
		return
	}
	start := time.Now()
	res := VerifyDKIM(r.Context(), t.res, in.Message)
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start))})
}
