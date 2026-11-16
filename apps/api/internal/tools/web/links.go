package web

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/html"

	"github.com/digibaro/net-tools-docker/apps/api/internal/httpapi"
	"github.com/digibaro/net-tools-docker/apps/api/internal/limits"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/httpcore"
	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/params"
)

// Crawl limits (docs/SECURITY.md).
const (
	MaxCrawlPages    = 30
	MaxCrawlDepth    = 2
	MaxLinkChecks    = 200
	MaxPerExternal   = 10
	linkParallel     = 4
	linkCheckTimeout = 10 * time.Second
)

// LinksTool is WEB-23: a small same-host crawl that reports broken links and assets.
type LinksTool struct{ deps tools.Deps }

func (*LinksTool) ID() string                                { return "web.links" }
func (*LinksTool) Enabled(d tools.Deps) (bool, tools.Reason) { return tools.Always(d) }

func (t *LinksTool) Mount(r chi.Router, d tools.Deps) {
	t.deps = d
	r.With(d.RateLimit(limits.Heavy), d.Slot, d.Timeout(90*time.Second)).Get("/http/links", t.handle)
}

// LinkCheck is the result for one URL.
type LinkCheck struct {
	URL string `json:"url"`
	// Kind: page (same-host HTML link), asset (img/script/css/…) or external.
	Kind      string `json:"kind"`
	Status    int    `json:"status"`
	Redirects int    `json:"redirects"`
	Final     string `json:"final"`
	Error     string `json:"error"`
	// Broken: status >= 400, an error, or a redirect loop.
	Broken bool `json:"broken"`
	// From is the first page that links here; Refs counts the pages that do.
	From string `json:"from"`
	Refs int    `json:"refs"`
}

// LinksResult is the response payload.
type LinksResult struct {
	Start     string      `json:"start"`
	Pages     []string    `json:"pages"`
	Checked   int         `json:"checked"`
	Broken    int         `json:"broken"`
	Truncated bool        `json:"truncated"`
	Links     []LinkCheck `json:"links"`
}

type found struct {
	url, kind string
}

// extractLinks returns the links and assets of a page (fragments removed, http(s) only).
func extractLinks(doc *html.Node, base *url.URL) []found {
	var out []found
	seen := map[string]bool{}
	add := func(raw, kind string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(raw, "#") {
			return
		}
		u, err := base.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return
		}
		u.Fragment = ""
		s := u.String()
		if !seen[s] {
			seen[s] = true
			out = append(out, found{s, kind})
		}
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "a", "area":
				add(attrOf(n, "href"), "page")
			case "iframe":
				add(attrOf(n, "src"), "page")
			case "img", "script", "source", "audio", "video", "track", "embed":
				add(attrOf(n, "src"), "asset")
			case "link":
				rel := strings.ToLower(attrOf(n, "rel"))
				if strings.Contains(rel, "stylesheet") || strings.Contains(rel, "icon") || strings.Contains(rel, "preload") || strings.Contains(rel, "manifest") {
					add(attrOf(n, "href"), "asset")
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

func sameSite(a, b *url.URL) bool {
	return strings.EqualFold(strings.TrimPrefix(a.Hostname(), "www."), strings.TrimPrefix(b.Hostname(), "www."))
}

// check requests one URL: HEAD first, GET when HEAD is not allowed.
func (t *LinksTool) check(ctx context.Context, raw string) LinkCheck {
	c := LinkCheck{URL: raw}
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		hops, err := (&httpcore.Client{Resolver: t.deps.Target}).Do(ctx, httpcore.Request{
			URL: raw, Method: method, FollowRedirects: true, MaxRedirects: 5, MaxBody: 64 << 10, Timeout: linkCheckTimeout, UserAgent: t.deps.UserAgent,
		})
		if err != nil {
			c.Error = errText(err)
			c.Broken = true
			return c
		}
		last := hops[len(hops)-1]
		c.Redirects, c.Final, c.Status, c.Error = len(hops)-1, last.URL, last.Status, last.Error
		if method == http.MethodHead && (last.Status == http.StatusMethodNotAllowed || last.Status == http.StatusNotImplemented || last.Status == http.StatusForbidden) {
			continue
		}
		break
	}
	if c.Final == c.URL {
		c.Final = ""
	}
	c.Broken = c.Error != "" || c.Status >= 400
	return c
}

func errText(err error) string {
	if e, ok := err.(interface{ APIMessage() string }); ok { //nolint:errorlint // message only
		return e.APIMessage()
	}
	return "request failed"
}

// Crawl walks the site from start and checks every link found.
func (t *LinksTool) Crawl(ctx context.Context, start string, external bool) (LinksResult, error) {
	res := LinksResult{Start: start, Pages: []string{}, Links: []LinkCheck{}}
	first, err := fetch(ctx, t.deps, httpcore.Request{URL: start, FollowRedirects: true, PreviewBytes: int(t.deps.Config.Limits.HTTPMaxBodyBytes)})
	if err != nil {
		return res, err
	}
	startURL, _ := url.Parse(first[len(first)-1].URL)
	type page struct {
		url   string
		depth int
		body  string
		ctype string
	}
	queue := []page{{url: startURL.String(), depth: 0, body: first[len(first)-1].BodyPreview, ctype: first[len(first)-1].ContentType}}
	visited := map[string]bool{startURL.String(): true}
	links := map[string]*LinkCheck{}
	order := []string{}
	for len(queue) > 0 && len(res.Pages) < MaxCrawlPages && ctx.Err() == nil {
		p := queue[0]
		queue = queue[1:]
		if p.body == "" && p.depth > 0 {
			hops, err := fetch(ctx, t.deps, httpcore.Request{URL: p.url, FollowRedirects: true, PreviewBytes: int(t.deps.Config.Limits.HTTPMaxBodyBytes)})
			if err != nil || len(hops) == 0 {
				continue
			}
			last := hops[len(hops)-1]
			p.body, p.ctype = last.BodyPreview, last.ContentType
		}
		if !strings.Contains(strings.ToLower(p.ctype), "html") {
			continue
		}
		res.Pages = append(res.Pages, p.url)
		base, _ := url.Parse(p.url)
		doc, err := html.Parse(strings.NewReader(p.body))
		if err != nil {
			continue
		}
		for _, f := range extractLinks(doc, base) {
			u, _ := url.Parse(f.url)
			kind := f.kind
			if !sameSite(u, startURL) {
				if !external {
					continue
				}
				kind = "external"
			}
			if l, ok := links[f.url]; ok {
				l.Refs++
				continue
			}
			links[f.url] = &LinkCheck{URL: f.url, Kind: kind, From: p.url, Refs: 1}
			order = append(order, f.url)
			if kind == "page" && p.depth < MaxCrawlDepth && !visited[f.url] {
				visited[f.url] = true
				queue = append(queue, page{url: f.url, depth: p.depth + 1})
			}
		}
	}
	if len(queue) > 0 {
		res.Truncated = true
	}

	// Check links, capped overall and per external host.
	perHost := map[string]int{}
	var todo []string
	for _, u := range order {
		l := links[u]
		if l.Kind == "external" {
			h, _ := url.Parse(u)
			if perHost[h.Hostname()] >= MaxPerExternal {
				continue
			}
			perHost[h.Hostname()]++
		}
		if len(todo) >= MaxLinkChecks {
			res.Truncated = true
			break
		}
		todo = append(todo, u)
	}
	sem := make(chan struct{}, linkParallel)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, u := range todo {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			c := t.check(ctx, u)
			mu.Lock()
			defer mu.Unlock()
			l := links[u]
			l.Status, l.Redirects, l.Final, l.Error, l.Broken = c.Status, c.Redirects, c.Final, c.Error, c.Broken
			res.Checked++
			if c.Broken {
				res.Broken++
			}
		})
	}
	wg.Wait()
	for _, u := range todo {
		res.Links = append(res.Links, *links[u])
	}
	slices.SortStableFunc(res.Links, func(a, b LinkCheck) int {
		if a.Broken != b.Broken {
			if a.Broken {
				return -1
			}
			return 1
		}
		return 0
	})
	return res, nil
}

func (t *LinksTool) handle(w http.ResponseWriter, r *http.Request) {
	q := params.New(r.URL.Query())
	raw := q.String("url", "", true, 2048)
	external := q.Bool("external", true)
	if err := q.Err(); err != nil {
		httpapi.Error(w, r, err)
		return
	}
	res, err := t.Crawl(r.Context(), raw, external)
	if err != nil {
		httpapi.Error(w, r, err)
		return
	}
	httpapi.JSON(w, r, res, &httpapi.Meta{Tool: t.ID(), Vantage: "server", Target: &httpapi.TargetMeta{Input: raw}})
}
