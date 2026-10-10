package catalog

import (
	"net/url"
	"strconv"

	"github.com/chinmay28/homeapps/server/internal/myapps"
)

// FromMyApps turns the apps added by URL into launcher entries, in the order
// they were added. health reports each URL's state (probe.Cache.Health);
// Icon and Color are filled in later, as for HostMan's apps.
func FromMyApps(list []myapps.App, health func(url string) string) []App {
	out := make([]App, 0, len(list))
	for _, a := range list {
		out = append(out, App{
			ID:     a.ID,
			Name:   a.Name,
			URL:    a.URL,
			Port:   portOf(a.URL),
			Health: health(a.URL),
		})
	}
	return out
}

// portOf is the port a URL connects to, explicit or implied by its scheme.
func portOf(raw string) int {
	u, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	if p, err := strconv.Atoi(u.Port()); err == nil {
		return p
	}
	switch u.Scheme {
	case "https":
		return 443
	case "http":
		return 80
	}
	return 0
}
