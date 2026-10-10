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

// userAgent names the fetcher. Some servers refuse Go's default one.
const userAgent = "Mozilla/5.0 (compatible; HomeApps; +https://github.com/chinmay28/homeapps)"

// Fetch loads appURL's page, its manifest if it links one, and the best icon.
// A missing manifest is not an error as long as the page names an icon; the
// conventional /favicon.ico is tried last. Links on the page resolve against
// where it was finally served from, after redirects. A page that answers but
// refuses (a router's login wall, a 401) still gets its /favicon.ico tried.
func Fetch(ctx context.Context, client *http.Client, appURL string) (Brand, error) {
	base, err := url.Parse(appURL)
	if err != nil {
		return Brand{}, err
	}
	var head Head
	page, err := get(ctx, client, appURL, maxPage)
	switch {
	case err == nil:
		base = page.url
		head = ParseHead(page.data, base)
	case !errors.As(err, new(statusError)):
		return Brand{}, err
	}

	var b Brand
	icons := []Icon{}
	if head.ManifestURL != "" {
		mu, _ := url.Parse(head.ManifestURL) // ParseHead only returns parseable URLs
		if r, err := get(ctx, client, head.ManifestURL, maxManifest); err == nil {
			if m, err := ParseManifest(r.data, mu); err == nil {
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
		if data, typ, err := FetchImage(ctx, client, ic.URL); err == nil {
			b.Icon, b.IconType = data, typ
			break
		}
	}
	if b.Icon == nil && b.ThemeColor == "" {
		return b, ErrNoBranding
	}
	return b, nil
}

// ErrNotImage means an address answered with something other than an image.
var ErrNotImage = errors.New("manifest: not an image")

// FetchImage loads one image of at most 512 KiB and names its media type.
func FetchImage(ctx context.Context, client *http.Client, imageURL string) ([]byte, string, error) {
	r, err := get(ctx, client, imageURL, maxIcon)
	if err != nil {
		return nil, "", err
	}
	typ := ImageType(r.typ, r.url.Path, r.data)
	if typ == "" {
		return nil, "", fmt.Errorf("%w: %s", ErrNotImage, imageURL)
	}
	return r.data, typ, nil
}

// statusError is a server that answered, but not with 200 OK.
type statusError struct {
	url  string
	code int
}

func (e statusError) Error() string { return fmt.Sprintf("manifest: GET %s: HTTP %d", e.url, e.code) }

type response struct {
	data []byte
	typ  string   // the declared Content-Type
	url  *url.URL // where the body came from, after redirects
}

func get(ctx context.Context, client *http.Client, u string, limit int64) (response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return response{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return response{}, statusError{url: u, code: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return response{}, err
	}
	if int64(len(data)) > limit {
		return response{}, fmt.Errorf("manifest: GET %s: larger than %d bytes", u, limit)
	}
	return response{data: data, typ: resp.Header.Get("Content-Type"), url: resp.Request.URL}, nil
}

// ImageType returns the media type to serve an icon as, or "" if the bytes
// are not an image. The declared type is trusted only when the content agrees,
// except for SVG, which sniffs as XML: that takes a declared type or a path
// ending in .svg. Pure.
func ImageType(declared, path string, data []byte) string {
	mt, _, _ := mime.ParseMediaType(declared)
	if mt == "image/svg+xml" || (mt == "" && strings.HasSuffix(strings.ToLower(path), ".svg")) {
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
