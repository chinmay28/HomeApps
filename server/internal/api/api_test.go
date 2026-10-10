package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chinmay28/homeapps/server/internal/branding"
	"github.com/chinmay28/homeapps/server/internal/hostman"
	"github.com/chinmay28/homeapps/server/internal/manifest"
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
	s := New(src, brands, "v2026.10.1", slog.New(slog.NewTextHandler(io.Discard, nil)))
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
	for _, k := range []string{"hosts", "source", "fetched_at"} {
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

func TestCatalogErrors(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("dial tcp: refused"), "not reachable at http://hostman.local:8899"},
		{hostman.ErrUnauthorized, "PIN"},
	}
	for _, c := range cases {
		_, _, h := newTestServer(t, &fakeSource{err: c.err})
		w := do(h, "/api/catalog")
		if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), c.want) {
			t.Errorf("%v -> %d %s; want 502 mentioning %q", c.err, w.Code, w.Body, c.want)
		}
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
	if w := do(h, "/api/nope"); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"error"`) {
		t.Errorf("unknown API path: %d %s", w.Code, w.Body)
	}
}
