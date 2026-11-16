package web

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/gif"  // register decoders for DecodeConfig
	_ "image/jpeg" // register decoders for DecodeConfig
	_ "image/png"  // register decoders for DecodeConfig
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/html"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/httpcore"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// Social preview limits.
const (
	MaxSocialImage = 5 << 20
	MaxInlineImage = 1 << 20
)

// SocialTool is WEB-22: Open Graph / Twitter card preview and checks.
type SocialTool struct{ deps tools.Deps }

func (*SocialTool) ID() string                                { return "web.social" }
func (*SocialTool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *SocialTool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Light), d.Slot, d.Timeout(40*time.Second)).Get("/http/social", t.handle)
}

// SocialImage describes the preview image.
type SocialImage struct {
	URL         string `json:"url"`
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	Bytes       int    `json:"bytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	// DataURI is set for images up to 1 MB, so the browser can show them without contacting the site.
	DataURI string `json:"dataUri"`
	Error   string `json:"error"`
}

// SocialIcon is one link rel=icon / apple-touch-icon.
type SocialIcon struct {
	Rel   string `json:"rel"`
	Href  string `json:"href"`
	Sizes string `json:"sizes"`
}

// SocialResult is the response payload.
type SocialResult struct {
	URL         string            `json:"url"`
	Status      int               `json:"status"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Canonical   string            `json:"canonical"`
	ThemeColor  string            `json:"themeColor"`
	Manifest    string            `json:"manifest"`
	OG          map[string]string `json:"og"`
	Twitter     map[string]string `json:"twitter"`
	Icons       []SocialIcon      `json:"icons"`
	Image       *SocialImage      `json:"image,omitempty"`
	Problems    []string          `json:"problems"`
	Warnings    []string          `json:"warnings"`
}

// ParseSocial reads the head tags of doc; relative URLs are resolved against base.
func ParseSocial(doc *html.Node, base *url.URL) SocialResult {
	res := SocialResult{OG: map[string]string{}, Twitter: map[string]string{}, Icons: []SocialIcon{}, Problems: []string{}, Warnings: []string{}}
	abs := func(s string) string {
		if u, err := base.Parse(strings.TrimSpace(s)); err == nil && s != "" {
			return u.String()
		}
		return s
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if res.Title == "" && n.FirstChild != nil {
					res.Title = strings.Join(strings.Fields(n.FirstChild.Data), " ")
				}
			case "meta":
				key := strings.ToLower(attrOf(n, "property"))
				if key == "" {
					key = strings.ToLower(attrOf(n, "name"))
				}
				val := strings.TrimSpace(attrOf(n, "content"))
				switch {
				case strings.HasPrefix(key, "og:"):
					if _, dup := res.OG[key[3:]]; !dup {
						res.OG[key[3:]] = val
					}
				case strings.HasPrefix(key, "twitter:"):
					if _, dup := res.Twitter[key[8:]]; !dup {
						res.Twitter[key[8:]] = val
					}
				case key == "description" && res.Description == "":
					res.Description = val
				case key == "theme-color" && res.ThemeColor == "":
					res.ThemeColor = val
				}
			case "link":
				rel := strings.ToLower(attrOf(n, "rel"))
				href := attrOf(n, "href")
				switch {
				case strings.Contains(rel, "canonical") && res.Canonical == "":
					res.Canonical = abs(href)
				case strings.Contains(rel, "manifest") && res.Manifest == "":
					res.Manifest = abs(href)
				case strings.Contains(rel, "icon") && len(res.Icons) < 20:
					res.Icons = append(res.Icons, SocialIcon{Rel: rel, Href: abs(href), Sizes: attrOf(n, "sizes")})
				}
			case "body":
				return // head data only
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return res
}

func addCode(list *[]string, code string) {
	if !slices.Contains(*list, code) {
		*list = append(*list, code)
	}
}

// review adds problems and warnings for what social networks need.
func review(res *SocialResult) {
	if res.OG["title"] == "" {
		addCode(&res.Warnings, "missing-og-title")
		if res.Title == "" {
			addCode(&res.Problems, "missing-title")
		}
	}
	if res.OG["description"] == "" && res.Description == "" {
		addCode(&res.Warnings, "missing-description")
	} else if res.OG["description"] == "" {
		addCode(&res.Warnings, "missing-og-description")
	}
	if res.OG["image"] == "" && res.Twitter["image"] == "" {
		addCode(&res.Problems, "missing-og-image")
	}
	if img := res.OG["image"]; img != "" {
		if u, err := url.Parse(img); err != nil || !u.IsAbs() {
			addCode(&res.Warnings, "image-relative")
		} else if u.Scheme != "https" {
			addCode(&res.Warnings, "image-http")
		}
	}
	if res.Twitter["card"] == "" {
		addCode(&res.Warnings, "missing-twitter-card")
	}
	if title := firstNonEmpty(res.OG["title"], res.Title); utf8.RuneCountInString(title) > 70 {
		addCode(&res.Warnings, "title-long")
	}
	if d := firstNonEmpty(res.OG["description"], res.Description); utf8.RuneCountInString(d) > 200 {
		addCode(&res.Warnings, "description-long")
	}
	if len(res.Icons) == 0 {
		addCode(&res.Warnings, "missing-favicon")
	}
	if res.Image != nil {
		switch {
		case res.Image.Error != "" || res.Image.Status >= 400:
			addCode(&res.Problems, "image-unreachable")
		case !strings.HasPrefix(res.Image.ContentType, "image/"):
			addCode(&res.Problems, "image-not-image")
		case res.Image.Bytes > MaxSocialImage-1:
			addCode(&res.Warnings, "image-large")
		case res.Image.Width > 0 && (res.Image.Width < 200 || res.Image.Height < 200):
			addCode(&res.Warnings, "image-small")
		}
	}
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

func (t *SocialTool) image(r *http.Request, raw string) *SocialImage {
	img := &SocialImage{URL: raw}
	hops, err := (&httpcore.Client{Resolver: t.deps.Target}).Do(r.Context(), httpcore.Request{
		URL: raw, Method: http.MethodGet, FollowRedirects: true, MaxRedirects: 5, MaxBody: MaxSocialImage, KeepRaw: true,
		Timeout: 15 * time.Second, UserAgent: t.deps.UserAgent,
	})
	if err != nil || len(hops) == 0 {
		img.Error = "fetch-failed"
		return img
	}
	last := hops[len(hops)-1]
	if last.Error != "" {
		img.Error = "fetch-failed"
		return img
	}
	img.Status, img.ContentType, img.Bytes = last.Status, strings.ToLower(strings.TrimSpace(strings.SplitN(last.ContentType, ";", 2)[0])), last.BodyBytes
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(last.Raw)); err == nil {
		img.Width, img.Height = cfg.Width, cfg.Height
	}
	inline := map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}
	if last.Status < 300 && inline[img.ContentType] && len(last.Raw) <= MaxInlineImage && !last.BodyTruncated {
		img.DataURI = "data:" + img.ContentType + ";base64," + base64.StdEncoding.EncodeToString(last.Raw)
	}
	return img
}

func (t *SocialTool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	raw := q.String("url", "", true, 2048)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	hops, err := fetch(r.Context(), t.deps, httpcore.Request{URL: raw, FollowRedirects: true, PreviewBytes: int(t.deps.Config.Limits.HTTPMaxBodyBytes)})
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	last := hops[len(hops)-1]
	base, _ := url.Parse(last.URL)
	doc, err := html.Parse(strings.NewReader(last.BodyPreview))
	if err != nil || base == nil {
		httpapi.Error(w, r, httpapi.InvalidInput("url", "the page is not HTML"))
		return
	}
	res := ParseSocial(doc, base)
	res.URL, res.Status = last.URL, last.Status
	if src := firstNonEmpty(res.OG["image"], res.Twitter["image"]); src != "" {
		if u, err := base.Parse(src); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			res.Image = t.image(r, u.String())
		}
	}
	review(&res)
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", Target: &httpapi.TargetMeta{Input: raw}})
}
