package catalog

import (
	"encoding/json"
	"testing"

	"github.com/chinmay28/homeapps/server/internal/myapps"
)

func TestFromMyApps(t *testing.T) {
	list := []myapps.App{
		{ID: "my-1", Name: "Router", URL: "http://router.lan/"},
		{ID: "my-2", Name: "NAS", URL: "https://nas.lan/admin"},
		{ID: "my-3", Name: "Vault", URL: "http://pi5.local:8123/"},
	}
	health := map[string]string{"http://router.lan/": HealthDown}
	got := FromMyApps(list, func(u string) string {
		if h, ok := health[u]; ok {
			return h
		}
		return HealthUp
	})
	wantPorts := []int{80, 443, 8123}
	for i, a := range got {
		if a.ID != list[i].ID || a.Port != wantPorts[i] {
			t.Errorf("app %d = %+v; want id %s and port %d (order kept, port implied by the scheme)", i, a, list[i].ID, wantPorts[i])
		}
	}
	if got[0].Health != HealthDown || got[1].Health != HealthUp {
		t.Errorf("health not taken from the probe: %+v", got)
	}
	b, _ := json.Marshal(FromMyApps(nil, nil))
	if string(b) != "[]" {
		t.Errorf("no apps must encode as [], got %s", b)
	}
}
