package email

import (
	"context"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/miekg/dns"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/target"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnslookup"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// Validator limits: addresses per request and distinct domains looked up.
const (
	MaxValidateAddresses = 50
	MaxValidateDomains   = 25
)

// ValidateTool is EM-19: address syntax and whether the domain can receive mail. No mail is sent
// and no SMTP connection is made.
type ValidateTool struct {
	deps tools.Deps
	res  resolver
}

func (*ValidateTool) ID() string                                { return "email.validate" }
func (*ValidateTool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *ValidateTool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	if t.res == nil {
		t.res = sysResolver{}
	}
	r.With(d.RateLimit(limits.Light), d.Slot, d.Timeout(30*time.Second)).Get("/email/validate", t.handle)
}

// AddressCheck is the verdict for one address.
type AddressCheck struct {
	Input  string `json:"input"`
	Local  string `json:"local"`
	Domain string `json:"domain"`
	// ASCII is the domain in A-label form (IDN).
	ASCII string `json:"ascii"`
	// Verdict: valid, risky (deliverable but disposable/role/typo) or invalid.
	Verdict    string   `json:"verdict"`
	Problems   []string `json:"problems"`
	Warnings   []string `json:"warnings"`
	MX         []string `json:"mx"`
	ImplicitMX bool     `json:"implicitMx"`
	Disposable bool     `json:"disposable"`
	Role       bool     `json:"role"`
	Free       bool     `json:"free"`
	SMTPUTF8   bool     `json:"smtputf8"`
	Suggestion string   `json:"suggestion"`
}

// ValidateResult is the response payload.
type ValidateResult struct {
	Results []AddressCheck `json:"results"`
}

var (
	// RFC 5322 dot-atom local part (atext with single dots).
	dotAtom = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\\.[A-Za-z0-9!#$%&'*+/=?^_`{|}~-]+)*$")
	quoted  = regexp.MustCompile(`^"(?:[\x20\x21\x23-\x5b\x5d-\x7e]|\\[\x20-\x7e])*"$`)
)

var roleLocals = map[string]bool{
	"admin": true, "administrator": true, "info": true, "support": true, "sales": true, "postmaster": true, "abuse": true, "noreply": true, "no-reply": true,
	"donotreply": true, "webmaster": true, "hostmaster": true, "contact": true, "office": true, "billing": true, "help": true, "marketing": true, "security": true,
	"root": true, "hr": true, "jobs": true, "careers": true, "privacy": true, "legal": true, "team": true, "hello": true, "mail": true, "newsletter": true,
	"accounts": true, "finance": true, "invoice": true, "invoices": true, "service": true, "enquiries": true, "inquiries": true, "media": true, "press": true,
}

var freeProviders = []string{
	"gmail.com", "googlemail.com", "outlook.com", "hotmail.com", "live.com", "msn.com", "yahoo.com", "ymail.com", "icloud.com", "me.com", "mac.com", "aol.com",
	"gmx.com", "gmx.net", "gmx.de", "web.de", "proton.me", "protonmail.com", "pm.me", "zoho.com", "yandex.com", "yandex.ru", "mail.ru", "tutanota.com", "tuta.io",
	"fastmail.com", "hey.com", "outlook.nl", "hotmail.nl", "live.nl", "ziggo.nl", "kpnmail.nl", "planet.nl", "hetnet.nl", "home.nl", "xs4all.nl", "telenet.be",
	"hotmail.co.uk", "yahoo.co.uk", "btinternet.com", "orange.fr", "free.fr", "laposte.net", "libero.it", "t-online.de", "freenet.de", "seznam.cz", "qq.com", "163.com",
}

var disposable = map[string]bool{}

func init() {
	for _, d := range strings.Fields(`mailinator.com guerrillamail.com guerrillamail.net guerrillamail.org sharklasers.com grr.la guerrillamailblock.com 10minutemail.com
	10minutemail.net temp-mail.org tempmail.com tempmail.net tempmailo.com tempmail.plus yopmail.com yopmail.net yopmail.fr trashmail.com trashmail.de
	trashmail.net getnada.com nada.email dispostable.com maildrop.cc mohmal.com throwawaymail.com emailondeck.com mintemail.com fakeinbox.com mailnesia.com
	spambox.us spamgourmet.com mailcatch.com mytemp.email tempr.email discard.email moakt.com burnermail.io inboxkitten.com getairmail.com mailpoof.com
	emailfake.com fakemail.net tempinbox.com temporaryemail.net tmpmail.org tmpmail.net mail.tm mail.gw 33mail.com anonaddy.me spam4.me trbvm.com
	mailsac.com harakirimail.com jetable.org mailtemp.info tempail.com luxusmail.org emltmp.com fexpost.com fexbox.org tmail.ws 1secmail.com 1secmail.org
	1secmail.net byom.de crazymailing.com e4ward.com mailexpire.com mailforspam.com mt2015.com spambog.com wegwerfmail.de wegwerfmail.net einrot.com
	meltmail.com mailinator.net mailinator2.com notmailinator.com binkmail.com bobmail.info chammy.info devnullmail.com letthemeatspam.com safetymail.info
	suremail.info thisisnotmyrealemail.com tradermail.info veryrealemail.com zippymail.info armyspy.com cuvox.de dayrep.com einrot.de fleckens.hu gustr.com
	jourrapide.com rhyta.com superrito.com teleworm.us`) {
		disposable[d] = true
	}
}

// levenshtein distance, capped for short strings.
func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// suggest returns a well-known provider domain that domain is probably a typo of.
func suggest(domain string) string {
	if slices.Contains(freeProviders, domain) {
		return ""
	}
	best, bestD := "", 3
	for _, p := range freeProviders {
		if d := levenshtein(domain, p); d < bestD && d > 0 {
			best, bestD = p, d
		}
	}
	if bestD <= 1 || (bestD == 2 && len(domain) >= 8) {
		return best
	}
	return ""
}

// parseAddress splits and checks the syntax of one address.
func parseAddress(in string) AddressCheck {
	c := AddressCheck{Input: in, Problems: []string{}, Warnings: []string{}, MX: []string{}}
	s := strings.TrimSpace(in)
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		s = s[1 : len(s)-1]
	}
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		c.Problems = append(c.Problems, "syntax")
		return c
	}
	c.Local, c.Domain = s[:at], strings.ToLower(s[at+1:])
	if len(c.Local) > 64 {
		c.Problems = append(c.Problems, "local-too-long")
	}
	switch {
	case dotAtom.MatchString(c.Local), quoted.MatchString(c.Local):
	case utf8.ValidString(c.Local) && !strings.ContainsAny(c.Local, " \"(),:;<>@[\\]") && !strings.Contains(c.Local, "..") && !strings.HasPrefix(c.Local, ".") && !strings.HasSuffix(c.Local, "."):
		c.SMTPUTF8 = true // RFC 6531 internationalised local part
	default:
		c.Problems = append(c.Problems, "local-syntax")
	}
	if strings.HasPrefix(c.Domain, "[") {
		c.Problems = append(c.Problems, "address-literal")
		return c
	}
	ascii, err := target.NormalizeHostname(c.Domain)
	if err != nil || !strings.Contains(ascii, ".") {
		c.Problems = append(c.Problems, "domain-syntax")
		return c
	}
	c.ASCII = ascii
	if len(c.Local)+1+len(ascii) > 254 {
		c.Problems = append(c.Problems, "too-long")
	}
	if c.SMTPUTF8 {
		c.Warnings = append(c.Warnings, "smtputf8")
	}
	return c
}

type domainInfo struct {
	mx       []string
	implicit bool
	nullMX   bool
	missing  bool
	err      bool
}

func lookupDomain(ctx context.Context, res resolver, domain string) domainInfo {
	mxs, rcode, err := res.MX(ctx, domain)
	if err != nil {
		return domainInfo{err: true}
	}
	if rcode == "NXDOMAIN" {
		return domainInfo{missing: true}
	}
	var info domainInfo
	for _, mx := range mxs {
		host := strings.TrimSuffix(strings.ToLower(mx.Mx), ".")
		if host == "" && mx.Preference == 0 && len(mxs) == 1 {
			info.nullMX = true
		}
		info.mx = append(info.mx, host)
	}
	if len(info.mx) == 0 {
		// RFC 5321 §5.1: no MX → the domain's own address is the mail host.
		na, _, _ := res.Exists(ctx, domain, dns.TypeA)
		n4, _, _ := res.Exists(ctx, domain, dns.TypeAAAA)
		if na+n4 > 0 {
			info.implicit = true
		} else {
			info.missing = true
		}
	}
	return info
}

// ValidateAddresses checks each address; each distinct domain is looked up once.
func ValidateAddresses(ctx context.Context, res resolver, inputs []string) []AddressCheck {
	out := make([]AddressCheck, len(inputs))
	domains := []string{}
	for i, in := range inputs {
		out[i] = parseAddress(in)
		if out[i].ASCII != "" && !slices.Contains(domains, out[i].ASCII) && len(domains) < MaxValidateDomains {
			domains = append(domains, out[i].ASCII)
		}
	}
	infos := make(map[string]domainInfo, len(domains))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, d := range domains {
		wg.Go(func() {
			info := lookupDomain(ctx, res, d)
			mu.Lock()
			infos[d] = info
			mu.Unlock()
		})
	}
	wg.Wait()
	for i := range out {
		c := &out[i]
		if c.ASCII != "" {
			info, ok := infos[c.ASCII]
			switch {
			case !ok:
				c.Warnings = append(c.Warnings, "not-checked")
			case info.err:
				c.Warnings = append(c.Warnings, "dns-error")
			case info.missing:
				c.Problems = append(c.Problems, "no-mail-domain")
			case info.nullMX:
				c.Problems = append(c.Problems, "null-mx")
			default:
				c.ImplicitMX = info.implicit
				if info.mx != nil {
					c.MX = info.mx
				}
				if info.implicit {
					c.Warnings = append(c.Warnings, "implicit-mx")
				}
			}
			c.Disposable = disposable[c.ASCII]
			c.Free = slices.Contains(freeProviders, c.ASCII)
			if c.Disposable {
				c.Warnings = append(c.Warnings, "disposable")
			}
			if c.Suggestion = suggest(c.ASCII); c.Suggestion != "" {
				c.Warnings = append(c.Warnings, "typo")
			}
		}
		if roleLocals[strings.ToLower(strings.SplitN(c.Local, "+", 2)[0])] {
			c.Role = true
			c.Warnings = append(c.Warnings, "role")
		}
		switch {
		case len(c.Problems) > 0:
			c.Verdict = "invalid"
		case c.Disposable || c.Suggestion != "" || c.Role || slices.Contains(c.Warnings, "dns-error") || slices.Contains(c.Warnings, "not-checked"):
			c.Verdict = "risky"
		default:
			c.Verdict = "valid"
		}
	}
	return out
}

func (t *ValidateTool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	raw := q.String("addresses", "", true, 16384)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	var inputs []string
	for _, s := range strings.FieldsFunc(raw, func(c rune) bool { return c == '\n' || c == ',' || c == ';' || c == '\r' }) {
		if s = strings.TrimSpace(s); s != "" {
			inputs = append(inputs, s)
		}
	}
	if len(inputs) == 0 {
		httpapi.Error(w, r, httpapi.InvalidInput("addresses", "enter at least one e-mail address"))
		return
	}
	if len(inputs) > MaxValidateAddresses {
		httpapi.Error(w, r, httpapi.InvalidInput("addresses", "at most 50 addresses per check"))
		return
	}
	start := time.Now()
	res := ValidateResult{Results: ValidateAddresses(r.Context(), t.res, inputs)}
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start))})
}
