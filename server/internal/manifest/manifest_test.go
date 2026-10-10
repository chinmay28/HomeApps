package manifest

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestParseHead(t *testing.T) {
	page := []byte(`<!doctype html><html><head>
<meta charset="utf-8">
<META NAME="theme-color" content="#5EEAD4">
<link rel="icon" href="/favicon.svg" type="image/svg+xml" sizes="any">
<link rel='apple-touch-icon' href='icons/touch.png'>
<link rel="manifest" href="manifest.webmanifest?v=2&amp;x=1">
<link rel="icon" href="javascript:alert(1)">
</head><body><link rel="manifest" href="/not-in-head.json"></body></html>`)
	h := ParseHead(page, mustURL(t, "http://pi5.local:8123/app/"))
	if h.ManifestURL != "http://pi5.local:8123/app/manifest.webmanifest?v=2&x=1" {
		t.Errorf("manifest URL = %q; want it resolved against the page and entity-decoded", h.ManifestURL)
	}
	if h.ThemeColor != "#5eead4" {
		t.Errorf("theme colour = %q; tag and attribute names are case-insensitive", h.ThemeColor)
	}
	want := []Icon{
		{URL: "http://pi5.local:8123/favicon.svg", Size: 512, Purpose: "any"},
		{URL: "http://pi5.local:8123/app/icons/touch.png", Size: 180, Purpose: "any"},
	}
	if len(h.Icons) != len(want) {
		t.Fatalf("icons = %+v; a javascript: URL must be dropped", h.Icons)
	}
	for i := range want {
		if h.Icons[i] != want[i] {
			t.Errorf("icon %d = %+v, want %+v", i, h.Icons[i], want[i])
		}
	}
}

func TestParseManifest(t *testing.T) {
	data := []byte(`{"name":"CountRoster","short_name":"Roster","theme_color":"expression(evil)",
		"icons":[{"src":"icon-192.png","sizes":"192x192"},{"src":"/m.png","sizes":"512x512","purpose":"maskable"}]}`)
	m, err := ParseManifest(data, mustURL(t, "http://h:8787/assets/manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if m.ShortName != "Roster" || m.Name != "CountRoster" {
		t.Errorf("names = %q/%q", m.ShortName, m.Name)
	}
	if m.ThemeColor != "" {
		t.Errorf("a non-hex theme colour must be dropped, it lands in a style attribute: %q", m.ThemeColor)
	}
	if m.Icons[0].URL != "http://h:8787/assets/icon-192.png" || m.Icons[0].Purpose != "any" {
		t.Errorf("icon src resolves against the manifest URL and purpose defaults to any: %+v", m.Icons[0])
	}
	if _, err := ParseManifest([]byte("not json"), mustURL(t, "http://h/")); err == nil {
		t.Error("garbage manifest accepted")
	}
}

func TestPickIcon(t *testing.T) {
	cases := []struct {
		name  string
		icons []Icon
		want  string
	}{
		{"smallest at least 144", []Icon{{URL: "512", Size: 512, Purpose: "any"}, {URL: "192", Size: 192, Purpose: "any"}, {URL: "48", Size: 48, Purpose: "any"}}, "192"},
		{"largest when all small", []Icon{{URL: "32", Size: 32, Purpose: "any"}, {URL: "96", Size: 96, Purpose: "any"}}, "96"},
		{"any beats maskable", []Icon{{URL: "mask", Size: 192, Purpose: "maskable"}, {URL: "any", Size: 64, Purpose: "any"}}, "any"},
		{"maskable when nothing else", []Icon{{URL: "mask", Size: 192, Purpose: "maskable"}}, "mask"},
		{"both purposes count as any", []Icon{{URL: "mask", Size: 192, Purpose: "maskable"}, {URL: "both", Size: 192, Purpose: "any maskable"}}, "both"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := PickIcon(c.icons)
			if !ok || got.URL != c.want {
				t.Errorf("picked %q, want %q", got.URL, c.want)
			}
		})
	}
	if _, ok := PickIcon(nil); ok {
		t.Error("picked an icon from nothing")
	}
}

func TestParseSizes(t *testing.T) {
	cases := map[string]int{"": 0, "192x192": 192, "16x16 48x48": 48, "ANY": 512, "120x60": 120, "bogus 32x32": 32}
	for in, want := range cases {
		if got := parseSizes(in); got != want {
			t.Errorf("parseSizes(%q) = %d, want %d", in, got, want)
		}
	}
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFetchFollowsManifestAndSkipsBrokenIcons(t *testing.T) {
	icon := pngBytes(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`<head><link rel="manifest" href="/m.json"><meta name="theme-color" content="#000000"></head>`))
	})
	mux.HandleFunc("/m.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"short_name":"Vault","theme_color":"#e7d3a8","icons":[
			{"src":"/missing-192.png","sizes":"192x192"},{"src":"/ok-512.png","sizes":"512x512"}]}`))
	})
	mux.HandleFunc("/ok-512.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain") // lying server: the bytes decide
		w.Write(icon)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	b, err := Fetch(context.Background(), srv.Client(), srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "Vault" || b.ThemeColor != "#e7d3a8" {
		t.Errorf("brand = %q %q; the manifest's short_name and colour win over the page's", b.Name, b.ThemeColor)
	}
	if !bytes.Equal(b.Icon, icon) || b.IconType != "image/png" {
		t.Errorf("icon = %d bytes %q; the 404ing best candidate should fall through to the next", len(b.Icon), b.IconType)
	}
}

func TestFetchRejectsNonImageIcon(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			w.Write([]byte("<html>login page</html>"))
			return
		}
		w.Write([]byte(`<head><title>bare</title></head>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	_, err := Fetch(context.Background(), srv.Client(), srv.URL+"/")
	if !errors.Is(err, ErrNoBranding) {
		t.Errorf("err = %v; an HTML page served as the favicon is not an icon", err)
	}
}

func TestImageTypeSVG(t *testing.T) {
	svg := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	if got := imageType("image/svg+xml", "http://h/i", svg); got != "image/svg+xml" {
		t.Errorf("declared SVG = %q", got)
	}
	if got := imageType("", "http://h/i.SVG", svg); got != "image/svg+xml" {
		t.Errorf("undeclared .svg = %q", got)
	}
	if got := imageType("image/svg+xml", "http://h/i", []byte("<html>")); got != "" {
		t.Errorf("non-SVG bytes declared as SVG accepted: %q", got)
	}
}
