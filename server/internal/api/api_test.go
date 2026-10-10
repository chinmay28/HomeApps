package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chinmay28/homeapps/server/internal/branding"
	"github.com/chinmay28/homeapps/server/internal/hostman"
	"github.com/chinmay28/homeapps/server/internal/manifest"
	"github.com/chinmay28/homeapps/server/internal/myapps"
	"github.com/chinmay28/homeapps/server/internal/probe"
)

type fakeSource struct {
	ov    *hostman.Overview
	err   error
	calls int
}

func (f *fakeSource) Overview(context.Context) (*hostman.Overview, error) {
	f.calls++
	return f.ov, f.err
}
func (f *fakeSource) BaseURL() string { return "http://hostman.local:8899" }

func sampleOverview() *hostman.Overview {
	return &hostman.Overview{
		Hosts: []hostman.Host{{ID: 1, Name: "pi5", Status: "online"}},
		Installations: []hostman.Installation{
			{ID: 7, HostID: 1, AppName: "SAND Vault", URL: "http://pi5:8123/", Ports: []int{8123}, HealthStatus: "passing"},
		},
	}
}

func newTestServer(t *testing.T, src *fakeSource) (*Server, *branding.Cache, http.Handler) {
	t.Helper()
	brands := branding.New(func(context.Context, string) (manifest.Brand, error) {
		return manifest.Brand{ThemeColor: "#e7d3a8", Icon: []byte("<svg></svg>"), IconType: "image/svg+xml"}, nil
	}, time.Hour, time.Minute)
	store, err := myapps.Open(filepath.Join(t.TempDir(), "apps.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		MyApps:  store,
		Probes:  probe.New(func(context.Context, string) string { return probe.Up }, time.Minute),
		Brands:  brands,
		NameOf:  func(context.Context, string) string { return "Named By Manifest" },
		Version: "v2026.10.1",
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if src != nil { // a nil *fakeSource must stay a nil interface: no HostMan
		cfg.Source = src
	}
	s := New(cfg)
	mux := http.NewServeMux()
	s.Routes(mux)
	return s, brands, mux
}

func do(h http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestCatalogPinsTheWireShape(t *testing.T) {
	_, brands, h := newTestServer(t, &fakeSource{ov: sampleOverview()})
	do(h, "/api/catalog") // first call schedules the branding fetch
	brands.Wait()
	w := do(h, "/api/catalog")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"my_apps", "hosts", "source", "hostman_error", "fetched_at"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing top-level %q in %s", k, w.Body)
		}
	}
	app := body["hosts"].([]any)[0].(map[string]any)["apps"].([]any)[0].(map[string]any)
	want := map[string]any{"id": "hm-7", "name": "SAND Vault", "url": "http://pi5:8123/", "port": float64(8123),
		"health": "up", "color": "#e7d3a8", "icon": "/api/icons/" + branding.Key("http://pi5:8123/")}
	for k, v := range want {
		if app[k] != v {
			t.Errorf("app[%q] = %v, want %v", k, app[k], v)
		}
	}
	host := body["hosts"].([]any)[0].(map[string]any)
	if v, ok := host["cpu_pct"]; !ok || v != nil {
		t.Errorf("cpu_pct should be an explicit null without a sample, got %v (present %v)", v, ok)
	}
}

func TestCatalogCachesHostManBriefly(t *testing.T) {
	src := &fakeSource{ov: sampleOverview()}
	s, _, h := newTestServer(t, src)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	do(h, "/api/catalog")
	do(h, "/api/catalog")
	if src.calls != 1 {
		t.Errorf("HostMan called %d times within the cache window", src.calls)
	}
	now = now.Add(6 * time.Second)
	do(h, "/api/catalog")
	if src.calls != 2 {
		t.Errorf("HostMan called %d times; a stale copy should be refreshed", src.calls)
	}
}

func TestCatalogWhenHostManFails(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("dial tcp: refused"), "not reachable at http://hostman.local:8899"},
		{hostman.ErrUnauthorized, "PIN"},
	}
	for _, c := range cases {
		_, _, h := newTestServer(t, &fakeSource{err: c.err})
		send(h, http.MethodPost, "/api/apps", `{"url":"router.lan","name":"Router"}`)
		w := do(h, "/api/catalog")
		var body catalogResponse
		json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != http.StatusOK || !strings.Contains(body.HostManError, c.want) {
			t.Errorf("%v -> %d %s; want 200 with hostman_error mentioning %q", c.err, w.Code, w.Body, c.want)
		}
		if len(body.MyApps) != 1 || body.Hosts == nil || len(body.Hosts) != 0 {
			t.Errorf("HostMan down must not take the added apps with it: %s", w.Body)
		}
	}
}

func TestCatalogWithoutHostMan(t *testing.T) {
	_, _, h := newTestServer(t, nil)
	w := do(h, "/api/catalog")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"source":null`) || !strings.Contains(w.Body.String(), `"hostman_error":""`) {
		t.Errorf("no HostMan configured: %d %s", w.Code, w.Body)
	}
}

func TestIconHeaders(t *testing.T) {
	_, brands, h := newTestServer(t, &fakeSource{ov: sampleOverview()})
	do(h, "/api/catalog")
	brands.Wait()
	w := do(h, "/api/icons/"+branding.Key("http://pi5:8123/"))
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("icon: %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("an SVG from another app is served from our origin; it must be sandboxed")
	}
	if do(h, "/api/icons/unknown").Code != http.StatusNotFound {
		t.Error("unknown icon key should 404")
	}
}

func TestHealthAndUnknownRoutes(t *testing.T) {
	_, _, h := newTestServer(t, &fakeSource{err: errors.New("down")})
	w := do(h, "/api/health")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"version":"v2026.10.1"`) {
		t.Errorf("health must answer even while HostMan is down: %d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"app":"homeapps"`) {
		t.Errorf("health must name the app so the shell can tell it from a sibling app: %s", w.Body)
	}
	if w := do(h, "/api/nope"); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"error"`) {
		t.Errorf("unknown API path: %d %s", w.Code, w.Body)
	}
}

func send(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
	return w
}

func TestMyAppsLifecycle(t *testing.T) {
	_, brands, h := newTestServer(t, &fakeSource{ov: sampleOverview()})

	w := send(h, http.MethodPost, "/api/apps", `{"url":"router.lan","name":"Router"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("add: %d %s", w.Code, w.Body)
	}
	var added myapps.App
	json.Unmarshal(w.Body.Bytes(), &added)
	if added.URL != "http://router.lan/" || added.Name != "Router" {
		t.Errorf("added %+v", added)
	}

	w = send(h, http.MethodPost, "/api/apps", `{"url":"nas.lan:5000"}`)
	var unnamed myapps.App
	json.Unmarshal(w.Body.Bytes(), &unnamed)
	if unnamed.Name != "Named By Manifest" {
		t.Errorf("an unnamed app should take its manifest's name, got %q", unnamed.Name)
	}

	do(h, "/api/catalog")
	brands.Wait()
	var cat catalogResponse
	json.Unmarshal(do(h, "/api/catalog").Body.Bytes(), &cat)
	if len(cat.MyApps) != 2 || cat.MyApps[0].ID != added.ID || cat.MyApps[0].Port != 80 || cat.MyApps[1].Port != 5000 {
		t.Errorf("catalog my_apps = %+v", cat.MyApps)
	}
	if cat.MyApps[0].Icon == "" || cat.MyApps[0].Color != "#e7d3a8" {
		t.Errorf("an added app gets its manifest icon like any other: %+v", cat.MyApps[0])
	}
	if len(cat.Hosts) != 1 {
		t.Errorf("HostMan's hosts still listed beside: %+v", cat.Hosts)
	}

	if w = send(h, http.MethodPatch, "/api/apps/"+added.ID, `{"name":"Home Router"}`); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Home Router") {
		t.Errorf("rename: %d %s", w.Code, w.Body)
	}
	if w = send(h, http.MethodDelete, "/api/apps/"+added.ID, ""); w.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", w.Code, w.Body)
	}
	var list []myapps.App
	json.Unmarshal(do(h, "/api/apps").Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != unnamed.ID {
		t.Errorf("list after delete = %+v", list)
	}
}

func TestMyAppsErrors(t *testing.T) {
	_, _, h := newTestServer(t, nil)
	send(h, http.MethodPost, "/api/apps", `{"url":"router.lan"}`)
	cases := []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/api/apps", `{"url":"ftp://x"}`, http.StatusBadRequest},
		{http.MethodPost, "/api/apps", `{"name":"no url"}`, http.StatusBadRequest},
		{http.MethodPost, "/api/apps", `not json`, http.StatusBadRequest},
		{http.MethodPost, "/api/apps", `{"url":"a.lan","extra":1}`, http.StatusBadRequest},
		{http.MethodPost, "/api/apps", `{"url":"http://ROUTER.lan/"}`, http.StatusConflict},
		{http.MethodPatch, "/api/apps/my-nope", `{"name":"x"}`, http.StatusNotFound},
		{http.MethodPatch, "/api/apps/my-nope", `{}`, http.StatusBadRequest},
		{http.MethodDelete, "/api/apps/my-nope", ``, http.StatusNotFound},
	}
	for _, c := range cases {
		w := send(h, c.method, c.path, c.body)
		if w.Code != c.status || !strings.Contains(w.Body.String(), `"error"`) {
			t.Errorf("%s %s %s -> %d %s; want %d with an error body", c.method, c.path, c.body, w.Code, w.Body, c.status)
		}
	}
}
