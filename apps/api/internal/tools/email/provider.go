package email

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/miekg/dns"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/dnslookup"
)

// ProviderTool is EM-20: which providers a domain uses for mail, from its public DNS.
type ProviderTool struct {
	deps tools.Deps
	res  resolver
}

func (*ProviderTool) ID() string                                { return "email.providers" }
func (*ProviderTool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *ProviderTool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	if t.res == nil {
		t.res = sysResolver{}
	}
	r.With(d.RateLimit(limits.Light), d.Slot, d.Timeout(30*time.Second)).Get("/email/providers", t.handle)
}

// Detection is one provider found, with the record that shows it.
type Detection struct {
	Provider string `json:"provider"`
	// Role: inbound, gateway, sending, verification or reporting.
	Role     string `json:"role"`
	Evidence string `json:"evidence"`
}

// ProviderResult is the response payload.
type ProviderResult struct {
	Domain     string      `json:"domain"`
	MX         []string    `json:"mx"`
	SPF        string      `json:"spf"`
	DMARC      string      `json:"dmarc"`
	Selectors  []string    `json:"selectors"`
	Detections []Detection `json:"detections"`
}

type fingerprint struct{ match, provider string }

// Host suffixes of MX targets. Gateways filter mail before it reaches the mailbox provider.
var mxInbound = []fingerprint{
	{".mail.protection.outlook.com", "Microsoft 365"}, {".mx.microsoft", "Microsoft 365"},
	{"aspmx.l.google.com", "Google Workspace"}, {".aspmx.l.google.com", "Google Workspace"}, {".googlemail.com", "Google Workspace"}, {"smtp.google.com", "Google Workspace"},
	{".zoho.com", "Zoho Mail"}, {".zoho.eu", "Zoho Mail"}, {".zoho.in", "Zoho Mail"},
	{".messagingengine.com", "Fastmail"}, {".protonmail.ch", "Proton Mail"}, {".icloud.com", "iCloud Mail"},
	{".yahoodns.net", "Yahoo Mail"}, {".yandex.net", "Yandex Mail"}, {".amazonaws.com", "Amazon WorkMail / SES"},
	{".mailgun.org", "Mailgun"}, {".sendgrid.net", "SendGrid"}, {".privateemail.com", "Namecheap Private Email"},
	{".ovh.net", "OVHcloud"}, {".kundenserver.de", "IONOS"}, {".ionos.com", "IONOS"}, {".ionos.de", "IONOS"},
	{".one.com", "one.com"}, {".transip.email", "TransIP"}, {".strato.de", "STRATO"}, {".gandi.net", "Gandi"},
	{".secureserver.net", "GoDaddy"}, {".hostinger.com", "Hostinger"}, {".mailbox.org", "mailbox.org"}, {".tutanota.de", "Tuta"},
}

var mxGateway = []fingerprint{
	{".pphosted.com", "Proofpoint"}, {".ppe-hosted.com", "Proofpoint Essentials"}, {".mimecast.com", "Mimecast"}, {".mimecast.co.za", "Mimecast"},
	{".barracudanetworks.com", "Barracuda"}, {".iphmx.com", "Cisco Secure Email"}, {".messagelabs.com", "Broadcom Email Security.cloud"},
	{".hornetsecurity.com", "Hornetsecurity"}, {".antispamcloud.com", "N-able Mail Assure / SpamExperts"}, {".spamexperts.com", "N-able Mail Assure / SpamExperts"},
	{".hydra.sophos.com", "Sophos Email"}, {".mailcontrol.com", "Forcepoint"}, {".trendmicro.com", "Trend Micro Email Security"}, {".trendmicro.eu", "Trend Micro Email Security"},
	{".mxrecord.io", "Mailprotector"}, {".spamtitan.com", "SpamTitan"}, {".mailguard.com.au", "MailGuard"}, {".fortimail.com", "FortiMail Cloud"},
}

// Domains in SPF include:/redirect= terms.
var spfSending = []fingerprint{
	{"spf.protection.outlook.com", "Microsoft 365"}, {"_spf.google.com", "Google Workspace"}, {"amazonses.com", "Amazon SES"},
	{"sendgrid.net", "SendGrid"}, {"mailgun.org", "Mailgun"}, {"spf.mandrillapp.com", "Mailchimp Transactional"}, {"servers.mcsv.net", "Mailchimp"},
	{"_spf.salesforce.com", "Salesforce"}, {"exacttarget.com", "Salesforce Marketing Cloud"}, {"mail.zendesk.com", "Zendesk"},
	{"spf.mtasv.net", "Postmark"}, {"sparkpostmail.com", "SparkPost"}, {"spf.sendinblue.com", "Brevo"}, {"spf.brevo.com", "Brevo"},
	{"mktomail.com", "Marketo"}, {"hubspotemail.net", "HubSpot"}, {"spf.hubspot.com", "HubSpot"}, {"zoho.com", "Zoho"}, {"zoho.eu", "Zoho"},
	{"protonmail.ch", "Proton Mail"}, {"messagingengine.com", "Fastmail"}, {"email.freshdesk.com", "Freshdesk"}, {"helpscoutemail.com", "Help Scout"},
	{"mail.intercom.io", "Intercom"}, {"_spf.atlassian.net", "Atlassian"}, {"pphosted.com", "Proofpoint"}, {"mimecast.com", "Mimecast"},
	{"spf.mailjet.com", "Mailjet"}, {"_spf.elasticemail.com", "Elastic Email"}, {"spf.smtp2go.com", "SMTP2GO"},
	{"_spf.qualtrics.com", "Qualtrics"}, {"spf.constantcontact.com", "Constant Contact"}, {"_spf.createsend.com", "Campaign Monitor"}, {"secureserver.net", "GoDaddy"},
	{"icloud.com", "iCloud Mail"}, {"_spf.mx.cloudflare.net", "Cloudflare Email Routing"}, {"mxsspf.sendpulse.com", "SendPulse"},
}

// Prefixes of TXT verification tokens at the domain apex.
var txtVerification = []fingerprint{
	{"ms=", "Microsoft 365"}, {"google-site-verification=", "Google"}, {"facebook-domain-verification=", "Meta"}, {"atlassian-domain-verification=", "Atlassian"},
	{"apple-domain-verification=", "Apple"}, {"docusign=", "DocuSign"}, {"adobe-idp-site-verification=", "Adobe"}, {"adobe-sign-verification=", "Adobe Sign"},
	{"zoom_verify_", "Zoom"}, {"stripe-verification=", "Stripe"}, {"globalsign-domain-verification=", "GlobalSign"}, {"slack-domain-verification=", "Slack"},
	{"dropbox-domain-verification=", "Dropbox"}, {"onetrust-domain-verification=", "OneTrust"}, {"miro-verification=", "Miro"}, {"canva-site-verification=", "Canva"},
	{"1password-site-verification=", "1Password"}, {"jamf-site-verification=", "Jamf"}, {"cisco-ci-domain-verification=", "Cisco"}, {"webexdomainverification", "Webex"},
	{"have-i-been-pwned-verification=", "Have I Been Pwned"}, {"openai-domain-verification=", "OpenAI"}, {"anthropic-domain-verification", "Anthropic"},
	{"yandex-verification:", "Yandex"}, {"mailru-verification:", "Mail.ru"}, {"hubspot-developer-verification=", "HubSpot"}, {"knowbe4-site-verification=", "KnowBe4"},
	{"logmein-verification-code=", "LogMeIn"}, {"citrix-verification-code=", "Citrix"}, {"amazonses:", "Amazon SES"}, {"mongodb-site-verification=", "MongoDB"},
	{"teamviewer-sso-verification=", "TeamViewer"}, {"wrike-verification=", "Wrike"}, {"airtable-verification=", "Airtable"}, {"notion-domain-verification=", "Notion"},
	{"figma-domain-verification=", "Figma"}, {"twilio-domain-verification=", "Twilio"}, {"brevo-code:", "Brevo"}, {"sendinblue-code:", "Brevo"},
	{"zoho-verification=", "Zoho"}, {"protonmail-verification=", "Proton Mail"}, {"klaviyo-site-verification=", "Klaviyo"}, {"pardot", "Salesforce Pardot"},
	{"workplace-domain-verification=", "Meta Workplace"}, {"bugcrowd-verification=", "Bugcrowd"}, {"h1-domain-verification=", "HackerOne"},
}

// DKIM selectors that point at a sending service (checked with one TXT query each).
var dkimSelectors = []fingerprint{
	{"selector1", "Microsoft 365"}, {"selector2", "Microsoft 365"}, {"google", "Google Workspace"}, {"k1", "Mailchimp"}, {"mandrill", "Mailchimp Transactional"},
	{"s1", "SendGrid"}, {"s2", "SendGrid"}, {"pm", "Postmark"}, {"mxvault", "Mailgun / generic"}, {"zendesk1", "Zendesk"}, {"hs1", "HubSpot"}, {"protonmail", "Proton Mail"},
	{"zmail", "Zoho Mail"}, {"fm1", "Fastmail"}, {"sig1", "iCloud Mail"}, {"cf2024-1", "Cloudflare"},
}

// DMARC aggregate-report destinations (rua domains).
var dmarcReporting = []fingerprint{
	{"dmarcian.com", "dmarcian"}, {"vali.email", "Valimail"}, {"agari.com", "Agari"}, {"ondmarc.com", "Red Sift OnDMARC"}, {"easydmarc", "EasyDMARC"},
	{"dmarc.postmarkapp.com", "Postmark DMARC"}, {"uriports.com", "URIports"}, {"mailhardener.com", "Mailhardener"}, {"dmarcadvisor.com", "DMARC Advisor"},
	{"dmarc-reports.cloudflare.net", "Cloudflare DMARC Management"}, {"rua.powerdmarc.com", "PowerDMARC"}, {"proofpoint.com", "Proofpoint"},
	{"emaildefense.proofpoint.com", "Proofpoint"}, {"sendmarc.com", "Sendmarc"}, {"dmarcly.com", "DMARCLY"}, {"glockapps.com", "GlockApps"},
	{"mimecast.com", "Mimecast DMARC Analyzer"}, {"dmarcanalyzer.com", "Mimecast DMARC Analyzer"}, {"fraudmarc.com", "Fraudmarc"}, {"dmarc.everest.email", "Everest"},
}

func hostMatch(host, suffix string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if strings.HasPrefix(suffix, ".") {
		return strings.HasSuffix(host, suffix)
	}
	return host == suffix || strings.HasSuffix(host, "."+suffix)
}

// DetectProviders reads the domain's MX, TXT, DMARC and a few DKIM selectors.
func DetectProviders(ctx context.Context, res resolver, domain string) ProviderResult {
	out := ProviderResult{Domain: domain, MX: []string{}, Selectors: []string{}, Detections: []Detection{}}
	seen := map[string]bool{}
	add := func(d Detection) {
		if k := d.Provider + "|" + d.Role; !seen[k] {
			seen[k] = true
			out.Detections = append(out.Detections, d)
		}
	}
	var (
		wg   sync.WaitGroup
		mxs  []*dns.MX
		txts []string
		dm   []string
		sel  = make([]bool, len(dkimSelectors))
	)
	wg.Go(func() { mxs, _, _ = res.MX(ctx, domain) })
	wg.Go(func() { txts, _, _ = res.TXT(ctx, domain) })
	wg.Go(func() { dm, _, _ = res.TXT(ctx, "_dmarc."+domain) })
	for i, s := range dkimSelectors {
		wg.Go(func() {
			if n, _, err := res.Exists(ctx, s.match+"._domainkey."+domain, dns.TypeTXT); err == nil && n > 0 {
				sel[i] = true
			}
		})
	}
	wg.Wait()

	slices.SortFunc(mxs, func(a, b *dns.MX) int { return int(a.Preference) - int(b.Preference) })
	for _, mx := range mxs {
		host := strings.TrimSuffix(strings.ToLower(mx.Mx), ".")
		out.MX = append(out.MX, host)
		for _, f := range mxGateway {
			if hostMatch(host, f.match) {
				add(Detection{f.provider, "gateway", "MX " + host})
			}
		}
		for _, f := range mxInbound {
			if hostMatch(host, f.match) {
				add(Detection{f.provider, "inbound", "MX " + host})
			}
		}
	}
	for _, txt := range txts {
		l := strings.ToLower(strings.TrimSpace(txt))
		if l == "v=spf1" || strings.HasPrefix(l, "v=spf1 ") {
			out.SPF = txt
			for _, term := range strings.Fields(l)[1:] {
				term = strings.TrimLeft(term, "+-~?")
				_, v, ok := strings.Cut(term, ":")
				if !ok {
					_, v, ok = strings.Cut(term, "=") // redirect=
				}
				if !ok || (!strings.HasPrefix(term, "include") && !strings.HasPrefix(term, "redirect")) {
					continue
				}
				for _, f := range spfSending {
					if hostMatch(v, f.match) {
						add(Detection{f.provider, "sending", "SPF " + term})
					}
				}
			}
			continue
		}
		for _, f := range txtVerification {
			if strings.HasPrefix(l, f.match) {
				ev := txt
				if len(ev) > 60 {
					ev = ev[:60] + "…"
				}
				add(Detection{f.provider, "verification", "TXT " + ev})
			}
		}
	}
	for i, ok := range sel {
		if ok {
			s := dkimSelectors[i]
			out.Selectors = append(out.Selectors, s.match)
			add(Detection{s.provider, "sending", "DKIM " + s.match + "._domainkey"})
		}
	}
	for _, rec := range dm {
		if !strings.HasPrefix(strings.ToLower(rec), "v=dmarc1") {
			continue
		}
		out.DMARC = rec
		for _, uri := range strings.Split(tagList(rec)["rua"], ",") {
			_, host, ok := strings.Cut(strings.TrimSpace(uri), "@")
			if !ok {
				continue
			}
			host = strings.SplitN(host, "!", 2)[0]
			for _, f := range dmarcReporting {
				if hostMatch(host, f.match) || strings.Contains(host, f.match) {
					add(Detection{f.provider, "reporting", "DMARC rua " + strings.TrimSpace(uri)})
				}
			}
		}
	}
	return out
}

func (t *ProviderTool) handle(w http.ResponseWriter, r *http.Request) {
	domain, ok := domainParam(w, r, "domain")
	if !ok {
		return
	}
	start := time.Now()
	res := DetectProviders(r.Context(), t.res, domain)
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", DurationMs: dnslookup.Ms(time.Since(start)), Target: &httpapi.TargetMeta{Input: domain}})
}
