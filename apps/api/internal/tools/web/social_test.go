package web

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/digibaro/net-tools-docker/apps/api/internal/tools/tooltest"
)

func TestSocialHandler(t *testing.T) {
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1200, 630)))
	var small bytes.Buffer
	_ = png.Encode(&small, image.NewRGBA(image.Rect(0, 0, 100, 100)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/og.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBuf.Bytes())
		case "/small.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(small.Bytes())
		case "/bare":
			_, _ = w.Write([]byte("<html><head><title>Bare</title></head><body><meta property=\"og:title\" content=\"in body\"></body></html>"))
		case "/smallimg":
			_, _ = w.Write([]byte(`<html><head><meta property="og:image" content="/small.png"></head></html>`))
		default:
			_, _ = w.Write([]byte(`<!doctype html><html><head>
<title>Shop – Example</title>
<meta name="description" content="Plain description">
<meta property="og:title" content="Great shop">
<meta property="og:description" content="Everything you need">
<meta property="og:image" content="` + "http://" + r.Host + `/og.png">
<meta name="twitter:card" content="summary_large_image">
<link rel="canonical" href="/shop">
<link rel="icon" href="/favicon.ico" sizes="32x32">
<meta name="theme-color" content="#123456">
</head><body></body></html>`))
		}
	}))
	t.Cleanup(srv.Close)
	env := tooltest.New(t, &SocialTool{}, tooltest.Options{AllowPrivate: true})

	var r SocialResult
	env.Get("/api/v1/http/social?url="+url.QueryEscape(srv.URL+"/")).Must(t).Decode(t, &r)
	if r.OG["title"] != "Great shop" || r.Twitter["card"] != "summary_large_image" || r.Title != "Shop – Example" || r.Canonical != srv.URL+"/shop" || r.ThemeColor != "#123456" {
		t.Errorf("parsed = %+v", r)
	}
	if r.Image == nil || r.Image.Width != 1200 || r.Image.Height != 630 || !strings.HasPrefix(r.Image.DataURI, "data:image/png;base64,") {
		t.Errorf("image = %+v", r.Image)
	}
	if len(r.Problems) != 0 || !slices.Equal(r.Warnings, []string{"image-http"}) {
		t.Errorf("problems %v warnings %v", r.Problems, r.Warnings)
	}

	r = SocialResult{}
	env.Get("/api/v1/http/social?url="+url.QueryEscape(srv.URL+"/bare")).Must(t).Decode(t, &r)
	if r.OG["title"] != "" || !slices.Contains(r.Problems, "missing-og-image") || !slices.Contains(r.Warnings, "missing-favicon") {
		t.Errorf("bare = %+v", r)
	}
	r = SocialResult{}
	env.Get("/api/v1/http/social?url="+url.QueryEscape(srv.URL+"/smallimg")).Must(t).Decode(t, &r)
	if !slices.Contains(r.Warnings, "image-small") || !slices.Contains(r.Warnings, "image-relative") {
		t.Errorf("small = %+v", r.Warnings)
	}
	tooltest.New(t, &SocialTool{}, tooltest.Options{}).Get("/api/v1/http/social?url="+url.QueryEscape("http://10.0.0.1/")).ExpectError(t, "TARGET_PRIVATE_DENIED")
}
