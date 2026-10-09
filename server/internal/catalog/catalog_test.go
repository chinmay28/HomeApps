package catalog

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chinmay28/homeapps/server/internal/hostman"
)

func overview() *hostman.Overview {
	return &hostman.Overview{
		Hosts: []hostman.Host{
			{ID: 2, Name: "pi5", Address: "pi5.local", Status: "online", Latest: &hostman.Sample{CPUPct: 2, MemUsed: 1, MemTotal: 4}},
			{ID: 1, Name: "nakedpi", Address: "nakedpi.local", Status: "online", IsSelf: true},
			{ID: 3, Name: "attic", Address: "attic.local", Status: "offline"},
		},
		Installations: []hostman.Installation{
			{ID: 10, HostID: 1, AppName: "thought Mesh", URL: "http://nakedpi.local:8881/", Ports: []int{8881}, HealthStatus: "passing"},
			{ID: 11, HostID: 1, AppName: "CountRoster", URL: "http://nakedpi.local:8787/", Ports: []int{8787}, HealthStatus: "failing", HealthDetail: "connection refused"},
			{ID: 12, HostID: 1, AppName: "Backup job", URL: "", HealthStatus: "passing"},
			{ID: 13, HostID: 2, AppName: "SAND Vault", URL: "http://pi5.local:8123/", Ports: []int{8123}, HealthStatus: "unchecked", Version: "v2026.9.40"},
			{ID: 14, HostID: 3, AppName: "Tickers", URL: "http://attic.local:8797/", HealthStatus: "passing"},
			{ID: 15, HostID: 99, AppName: "Orphan", URL: "http://x/"},
		},
	}
}

func TestBuildOrdersSelfFirstThenByName(t *testing.T) {
	c := Build(overview())
	var got []string
	for _, h := range c.Hosts {
		got = append(got, h.Name)
	}
	if strings.Join(got, ",") != "nakedpi,attic,pi5" {
		t.Errorf("host order = %v; want the self host first, then case-insensitive by name", got)
	}
	var apps []string
	for _, a := range c.Hosts[0].Apps {
		apps = append(apps, a.Name)
	}
	if strings.Join(apps, ",") != "CountRoster,thought Mesh" {
		t.Errorf("app order = %v; want case-insensitive by name", apps)
	}
}

func TestBuildLeavesOutWhatCannotBeOpened(t *testing.T) {
	for _, h := range Build(overview()).Hosts {
		for _, a := range h.Apps {
			if a.Name == "Backup job" {
				t.Error("an installation with no URL was listed; tapping it would go nowhere")
			}
			if a.Name == "Orphan" {
				t.Error("an installation on an unknown host was listed")
			}
		}
	}
}

func TestBuildHealth(t *testing.T) {
	byName := map[string]App{}
	for _, h := range Build(overview()).Hosts {
		for _, a := range h.Apps {
			byName[a.Name] = a
		}
	}
	cases := []struct{ app, want string }{
		{"thought Mesh", HealthUp},
		{"CountRoster", HealthDown},
		{"SAND Vault", HealthUnknown}, // unchecked
		{"Tickers", HealthUnknown},    // passing, but its host is offline: stale
	}
	for _, c := range cases {
		if got := byName[c.app].Health; got != c.want {
			t.Errorf("%s health = %q, want %q", c.app, got, c.want)
		}
	}
	if byName["CountRoster"].Detail != "connection refused" {
		t.Error("health detail dropped; the UI shows it for a down app")
	}
}

func TestBuildFields(t *testing.T) {
	c := Build(overview())
	pi5 := c.Hosts[2]
	if pi5.CPUPct == nil || *pi5.CPUPct != 2 || pi5.MemPct == nil || *pi5.MemPct != 25 {
		t.Errorf("pi5 metrics = cpu %v mem %v, want 2 and 25", pi5.CPUPct, pi5.MemPct)
	}
	if c.Hosts[0].CPUPct != nil {
		t.Error("a host with no sample should report null metrics, not zero")
	}
	sv := pi5.Apps[0]
	if sv.ID != "hm-13" || sv.Port != 8123 || sv.Version != "v2026.9.40" {
		t.Errorf("SAND Vault = %+v", sv)
	}
}

func TestEmptyHostSerialisesAppsAsArray(t *testing.T) {
	c := Build(&hostman.Overview{Hosts: []hostman.Host{{ID: 1, Name: "bare"}}})
	b, _ := json.Marshal(c)
	if !strings.Contains(string(b), `"apps":[]`) {
		t.Errorf("apps must be [] not null for the client: %s", b)
	}
	b, _ = json.Marshal(Build(&hostman.Overview{}))
	if string(b) != `{"hosts":[]}` {
		t.Errorf("an empty overview should be {\"hosts\":[]}, got %s", b)
	}
}
