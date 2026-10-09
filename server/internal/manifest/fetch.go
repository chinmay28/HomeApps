package manifest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

// Brand is what Fetch learned about one app.
type Brand struct {
	Name       string // the manifest's short_name, else name; may be empty
	ThemeColor string // "#rrggbb" or "#rgb"; may be empty
	Icon       []byte // nil when no usable icon was found
	IconType   string // the icon's media type, e.g. "image/png"
}

const (
	maxPage     = 1 << 20
	maxManifest = 256 << 10
	maxIcon     = 512 << 10
)

// ErrNoBranding means the app answered but declares nothing usable.
var ErrNoBranding = errors.New("manifest: no icon or theme colour found")

// Fetch loads appURL's page, its manifest if it links one, and the best icon.
// A missing manifest is not an error as long as the page names an icon; the
// conventional /favicon.ico is tried last.
func Fetch(ctx context.Context, client *http.Client, appURL string) (Brand, error) {
	base, err := url.Parse(appURL)
	if err != nil {
		return Brand{}, err
	}
	page, _, err := get(ctx, client, appURL, maxPage)
	if err != nil {
		return Brand{}, err
	}
	head := ParseHead(page, base)

	var b Brand
	icons := []Icon{}
	if head.ManifestURL != "" {
		mu, _ := url.Parse(head.ManifestURL) // ParseHead only returns parseable URLs
		if data, _, err := get(ctx, client, head.ManifestURL, maxManifest); err == nil {
			if m, err := ParseManifest(data, mu); err == nil {
				b.Name = firstNonEmpty(m.ShortName, m.Name)
				b.ThemeColor = m.ThemeColor
				icons = append(icons, m.Icons...)
			}
		}
	}
	if b.ThemeColor == "" {
		b.ThemeColor = head.ThemeColor
	}
	icons = append(icons, head.Icons...)

	// Try candidates best first, so one broken icon URL doesn't lose the rest.
	if fav := resolve(base, "/favicon.ico"); fav != "" {
		icons = append(icons, Icon{URL: fav, Size: 1, Purpose: "any"})
	}
	for len(icons) > 0 {
		ic, _ := PickIcon(icons)
		icons = remove(icons, ic)
		data, typ, err := get(ctx, client, ic.URL, maxIcon)
		if err != nil {
			continue
		}
		if typ = imageType(typ, ic.URL, data); typ != "" {
			b.Icon, b.IconType = data, typ
			break
		}
	}
	if b.Icon == nil && b.ThemeColor == "" {
		return b, ErrNoBranding
	}
	return b, nil
}

func get(ctx context.Context, client *http.Client, u string, limit int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("manifest: GET %s: HTTP %d", u, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > limit {
		return nil, "", fmt.Errorf("manifest: GET %s: larger than %d bytes", u, limit)
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// imageType returns the media type to serve an icon as, or "" if the bytes
// are not an image. The declared type is trusted only when the content agrees,
// except for SVG, which sniffs as XML.
func imageType(declared, u string, data []byte) string {
	mt, _, _ := mime.ParseMediaType(declared)
	if mt == "image/svg+xml" || (mt == "" && strings.HasSuffix(strings.ToLower(u), ".svg")) {
		if strings.Contains(string(data[:min(len(data), 1024)]), "<svg") {
			return "image/svg+xml"
		}
		return ""
	}
	if sniffed := http.DetectContentType(data); strings.HasPrefix(sniffed, "image/") {
		return sniffed
	}
	return ""
}

func remove(icons []Icon, ic Icon) []Icon {
	out := icons[:0]
	for _, x := range icons {
		if x != ic {
			out = append(out, x)
		}
	}
	return out
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}
