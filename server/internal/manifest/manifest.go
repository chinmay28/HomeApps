// Package manifest finds an app's own branding: the icon and theme colour it
// declares in its web app manifest, so the launcher shows each app the way it
// looks when installed on its own, instead of a letter tile.
//
// The parsing is pure (ParseHead, ParseManifest, PickIcon) and tested on
// fixtures; Fetch is the thin part that goes over the network. HTML is
// scanned for <link> and <meta> tags rather than parsed into a tree: the head
// of a PWA is small and regular, and this keeps the module dependency-free.
package manifest

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Icon is one candidate image, with its URL already made absolute.
type Icon struct {
	URL     string
	Size    int // largest declared dimension; 0 when unknown; 512 for "any" (SVG)
	Purpose string
}

// Head is what an app's HTML page says about its branding.
type Head struct {
	ManifestURL string
	ThemeColor  string
	Icons       []Icon
}

// Manifest is the part of a web app manifest the launcher reads.
type Manifest struct {
	Name       string
	ShortName  string
	ThemeColor string
	Icons      []Icon
}

var (
	tagRE   = regexp.MustCompile(`(?is)<(link|meta)\b([^>]*)>`)
	attrRE  = regexp.MustCompile(`(?is)([a-z-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	headEnd = regexp.MustCompile(`(?i)</head>`)
)

// ParseHead reads <link rel="manifest">, icon links and <meta name="theme-color">
// from page, resolving URLs against base.
func ParseHead(page []byte, base *url.URL) Head {
	if loc := headEnd.FindIndex(page); loc != nil {
		page = page[:loc[0]]
	}
	var h Head
	for _, m := range tagRE.FindAllSubmatch(page, -1) {
		attrs := parseAttrs(string(m[2]))
		switch strings.ToLower(string(m[1])) {
		case "meta":
			if strings.EqualFold(attrs["name"], "theme-color") && h.ThemeColor == "" {
				h.ThemeColor = cleanColor(attrs["content"])
			}
		case "link":
			href := resolve(base, attrs["href"])
			if href == "" {
				continue
			}
			for _, rel := range strings.Fields(strings.ToLower(attrs["rel"])) {
				switch rel {
				case "manifest":
					if h.ManifestURL == "" {
						h.ManifestURL = href
					}
				case "icon", "apple-touch-icon", "apple-touch-icon-precomposed":
					size := parseSizes(attrs["sizes"])
					if size == 0 && rel != "icon" {
						size = 180 // Apple's default size for an unsized touch icon
					}
					h.Icons = append(h.Icons, Icon{URL: href, Size: size, Purpose: "any"})
				}
			}
		}
	}
	return h
}

// ParseManifest reads a manifest's name, colour and icons, resolving icon URLs
// against the manifest's own URL as the spec requires.
func ParseManifest(data []byte, manifestURL *url.URL) (Manifest, error) {
	var raw struct {
		Name       string `json:"name"`
		ShortName  string `json:"short_name"`
		ThemeColor string `json:"theme_color"`
		Icons      []struct {
			Src     string `json:"src"`
			Sizes   string `json:"sizes"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Manifest{}, err
	}
	m := Manifest{Name: raw.Name, ShortName: raw.ShortName, ThemeColor: cleanColor(raw.ThemeColor)}
	for _, ic := range raw.Icons {
		href := resolve(manifestURL, ic.Src)
		if href == "" {
			continue
		}
		purpose := strings.ToLower(strings.TrimSpace(ic.Purpose))
		if purpose == "" {
			purpose = "any"
		}
		m.Icons = append(m.Icons, Icon{URL: href, Size: parseSizes(ic.Sizes), Purpose: purpose})
	}
	return m, nil
}

// PickIcon chooses the icon to draw in a ~64px tile on a 3x screen. It prefers
// icons meant to be shown as-is ("any") over maskable ones, whose art is
// shrunk into a safe zone; among those, the smallest that is at least 144px,
// else the largest there is. ok is false when there is nothing to choose.
func PickIcon(icons []Icon) (Icon, bool) {
	var best Icon
	found := false
	for _, ic := range icons {
		if !found || better(ic, best) {
			best, found = ic, true
		}
	}
	return best, found
}

const wantSize = 144

func better(a, b Icon) bool {
	if pa, pb := isAny(a), isAny(b); pa != pb {
		return pa
	}
	aBig, bBig := a.Size >= wantSize, b.Size >= wantSize
	switch {
	case aBig && bBig:
		return a.Size < b.Size
	case aBig != bBig:
		return aBig
	default:
		return a.Size > b.Size
	}
}

func isAny(ic Icon) bool {
	for _, p := range strings.Fields(ic.Purpose) {
		if p == "any" {
			return true
		}
	}
	return false
}

// parseSizes returns the largest dimension in a sizes attribute ("48x48
// 192x192", "any"). "any" means scalable, which is as good as large.
func parseSizes(s string) int {
	best := 0
	for _, f := range strings.Fields(strings.ToLower(s)) {
		if f == "any" {
			return 512
		}
		w, h, ok := strings.Cut(f, "x")
		if !ok {
			continue
		}
		wi, err1 := strconv.Atoi(w)
		hi, err2 := strconv.Atoi(h)
		if err1 != nil || err2 != nil {
			continue
		}
		best = max(best, wi, hi)
	}
	return best
}

func parseAttrs(s string) map[string]string {
	out := map[string]string{}
	for _, m := range attrRE.FindAllStringSubmatch(s, -1) {
		key := strings.ToLower(m[1])
		if _, seen := out[key]; !seen {
			out[key] = htmlUnescape(m[2] + m[3] + m[4])
		}
	}
	return out
}

func htmlUnescape(s string) string {
	return strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">").Replace(s)
}

// resolve makes ref absolute against base, keeping only http(s) results: a
// data: or javascript: URL is not something to fetch.
func resolve(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := base.Parse(ref)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	u.Fragment = ""
	return u.String()
}

var colorRE = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// cleanColor keeps hex colours only. The value ends up in a style attribute
// on the client, so anything else is dropped rather than passed along.
func cleanColor(s string) string {
	s = strings.TrimSpace(s)
	if colorRE.MatchString(s) {
		return strings.ToLower(s)
	}
	return ""
}
