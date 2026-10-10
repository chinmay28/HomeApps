// Package catalog turns HostMan's overview into what the launcher shows: hosts,
// each with the apps on it that can be opened, in a stable order.
//
// It is a pure function of the overview, so every rule here (what counts as
// openable, how health reads, how things sort) is tested without a network.
package catalog

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/chinmay28/homeapps/server/internal/hostman"
)

// Health is an app's state as the launcher draws it.
const (
	HealthUp      = "up"
	HealthDown    = "down"
	HealthUnknown = "unknown"
)

// Catalog is every host and the apps on it. It is the /api/catalog body, so
// its JSON shape is the client's contract.
type Catalog struct {
	Hosts []Host `json:"hosts"`
}

// Host is one machine and its openable apps. Apps is never null.
type Host struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Address string   `json:"address"`
	Online  bool     `json:"online"`
	Self    bool     `json:"self"`
	CPUPct  *float64 `json:"cpu_pct"`
	MemPct  *float64 `json:"mem_pct"`
	Apps    []App    `json:"apps"`
}

// App is one installation that has an address to open.
type App struct {
	// ID is stable for as long as the installation exists in HostMan; the
	// Android shell keys its running WebViews by it.
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Port    int    `json:"port"`
	Health  string `json:"health"`
	Detail  string `json:"detail"`
	Version string `json:"version"`

	// Filled in by the API from the app's own web manifest, when known.
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

// Build shapes an overview. Installations without a URL are left out: there
// is nothing to open. Hosts are kept even when empty, so the Hosts view can
// still show a machine with nothing on it.
func Build(ov *hostman.Overview) Catalog {
	hosts := make([]Host, 0, len(ov.Hosts))
	index := make(map[int64]int, len(ov.Hosts))
	for _, h := range ov.Hosts {
		index[h.ID] = len(hosts)
		hosts = append(hosts, Host{
			ID:      h.ID,
			Name:    h.Name,
			Address: h.Address,
			Online:  h.Status == "online",
			Self:    h.IsSelf,
			CPUPct:  cpu(h.Latest),
			MemPct:  mem(h.Latest),
			Apps:    []App{},
		})
	}
	for _, in := range ov.Installations {
		i, ok := index[in.HostID]
		if !ok || in.URL == "" {
			continue
		}
		hosts[i].Apps = append(hosts[i].Apps, App{
			ID:      "hm-" + strconv.FormatInt(in.ID, 10),
			Name:    in.AppName,
			URL:     in.URL,
			Port:    firstPort(in.Ports),
			Health:  health(in.HealthStatus, hosts[i].Online),
			Detail:  in.HealthDetail,
			Version: in.Version,
		})
	}
	for i := range hosts {
		slices.SortStableFunc(hosts[i].Apps, func(a, b App) int {
			return cmp.Or(compareFold(a.Name, b.Name), strings.Compare(a.ID, b.ID))
		})
	}
	// The machine HomeApps is most likely running beside comes first, then
	// the rest by name: the same order HostMan's dashboard uses.
	slices.SortStableFunc(hosts, func(a, b Host) int {
		if a.Self != b.Self {
			if a.Self {
				return -1
			}
			return 1
		}
		return cmp.Or(compareFold(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
	return Catalog{Hosts: hosts}
}

// health maps HostMan's check result. A host that is offline makes its last
// result stale, so it reads as unknown rather than as a confident green.
func health(status string, hostOnline bool) string {
	if !hostOnline {
		return HealthUnknown
	}
	switch status {
	case "passing":
		return HealthUp
	case "failing":
		return HealthDown
	}
	return HealthUnknown
}

func cpu(s *hostman.Sample) *float64 {
	if s == nil {
		return nil
	}
	v := s.CPUPct
	return &v
}

func mem(s *hostman.Sample) *float64 {
	if s == nil || s.MemTotal <= 0 {
		return nil
	}
	v := float64(s.MemUsed) * 100 / float64(s.MemTotal)
	return &v
}

func firstPort(ports []int) int {
	if len(ports) == 0 {
		return 0
	}
	return ports[0]
}

func compareFold(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }
