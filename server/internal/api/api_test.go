package api

import (
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
	"github.com/chinmay28/homeapps/server/internal/custom"
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
	apps, err := custom.Open(filepath.Join(t.TempDir(), "custom-apps.json"))
	if err != nil {
		t.Fatal(err)
	}
	fetchImage := func(_ context.Context, u string) ([]byte, string, error) {
		if u == "http://img.local/logo.png" {
			return pngBytes, "image/png", nil
		}
		return nil, "", errors.New("connection refused")
	}
	s := New(src, apps, brands, fetchImage, "v2026.10.1", slog.New(slog.NewTextHandler(io.Discard, nil)))
	mux := http.NewServeMux()
	s.Routes(mux)
	return s, brands, mux
}

func do(h http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

// send makes a request with a JSON body ("" for none).
func send(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("%d %q: %v", w.Code, w.Body, err)
	}
	return v
}

func TestCustomAppsLifecycle(t *testing.T) {
	// HostMan is down: custom apps must not depend on it.
	_, brands, h := newTestServer(t, &fakeSource{err: errors.New("down")})
	if w := do(h, "/api/custom-apps"); w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"apps":[]}` {
		t.Fatalf("empty list must be an explicit []: %d %s", w.Code, w.Body)
	}

	w := send(h, http.MethodPost, "/api/custom-apps", `{"name":"Router","url":"http://192.168.1.1"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("add: %d %s", w.Code, w.Body)
	}
	added := decode(t, w)
	id, _ := added["id"].(string)
	if !strings.HasPrefix(id, custom.IDPrefix) || added["url"] != "http://192.168.1.1/" {
		t.Errorf("added %v", added)
	}
	if v, ok := added["icon"]; !ok || v != "" {
		t.Errorf("icon must be present, and empty until the manifest is fetched: %v", added)
	}

	brands.Wait()
	list := decode(t, do(h, "/api/custom-apps"))["apps"].([]any)
	app := list[0].(map[string]any)
	want := map[string]any{"id": id, "name": "Router", "url": "http://192.168.1.1/", "color": "#e7d3a8",
		"icon": "/api/icons/" + branding.Key("http://192.168.1.1/"), "custom_icon": false}
	for k, v := range want {
		if app[k] != v {
			t.Errorf("app[%q] = %v, want %v", k, app[k], v)
		}
	}

	if w := send(h, http.MethodPut, "/api/custom-apps/"+id, `{"name":"Gateway","url":"https://192.168.1.1"}`); w.Code != http.StatusOK || decode(t, w)["name"] != "Gateway" {
		t.Errorf("update: %d %s", w.Code, w.Body)
	}
	if w := send(h, http.MethodDelete, "/api/custom-apps/"+id, ""); w.Code != http.StatusNoContent {
		t.Errorf("remove: %d %s", w.Code, w.Body)
	}
	if n := len(decode(t, do(h, "/api/custom-apps"))["apps"].([]any)); n != 0 {
		t.Errorf("%d apps left after remove", n)
	}
}

// pngBytes is a minimal image that sniffs as PNG.
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 24)...)

func TestCustomAppIcons(t *testing.T) {
	_, brands, h := newTestServer(t, &fakeSource{err: errors.New("down")})
	id := decode(t, send(h, http.MethodPost, "/api/custom-apps", `{"name":"Router","url":"http://192.168.1.1"}`))["id"].(string)
	brands.Wait()
	iconPath := "/api/custom-apps/" + id + "/icon"
	if w := do(h, iconPath); w.Code != http.StatusNotFound {
		t.Errorf("an app with no icon of its own has none to serve: %d", w.Code)
	}

	// Uploaded: the picture itself, base64 in JSON.
	body, _ := json.Marshal(map[string]any{"data": pngBytes})
	w := send(h, http.MethodPut, iconPath, string(body))
	if w.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	app := decode(t, w)
	icon, _ := app["icon"].(string)
	if !strings.HasPrefix(icon, iconPath+"?v=") || app["custom_icon"] != true || app["color"] != "#e7d3a8" {
		t.Errorf("after upload: %v; want a versioned icon URL, custom_icon, the manifest's colour", app)
	}
	served := do(h, icon)
	if served.Code != http.StatusOK || served.Header().Get("Content-Type") != "image/png" || served.Body.String() != string(pngBytes) {
		t.Errorf("serving the upload: %d %q", served.Code, served.Header().Get("Content-Type"))
	}
	if !strings.Contains(served.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Error("an uploaded icon is served from our origin; it must be sandboxed")
	}
	if w := send(h, http.MethodPut, "/api/custom-apps/"+id, `{"name":"Gateway","url":"http://192.168.1.1"}`); decode(t, w)["custom_icon"] != true {
		t.Errorf("renaming must keep the icon: %s", w.Body)
	}

	// By address: fetched once and kept, so it needn't stay reachable.
	w = send(h, http.MethodPut, iconPath, `{"url":"http://img.local/logo.png"}`)
	if w.Code != http.StatusOK || decode(t, w)["icon"] != icon {
		t.Errorf("by address: %d %s; the same picture keeps the same URL", w.Code, w.Body)
	}

	// Cleared: back to the manifest's icon.
	w = send(h, http.MethodDelete, iconPath, "")
	if app := decode(t, w); w.Code != http.StatusOK || app["custom_icon"] != false || app["icon"] != "/api/icons/"+branding.Key("http://192.168.1.1/") {
		t.Errorf("clear: %d %v", w.Code, app)
	}
	if do(h, iconPath).Code != http.StatusNotFound {
		t.Error("a cleared icon is still served")
	}
}

func TestCustomAppIconErrors(t *testing.T) {
	_, _, h := newTestServer(t, &fakeSource{ov: sampleOverview()})
	id := decode(t, send(h, http.MethodPost, "/api/custom-apps", `{"name":"A","url":"http://a/"}`))["id"].(string)
	iconPath := "/api/custom-apps/" + id + "/icon"
	notImage, _ := json.Marshal(map[string]any{"data": []byte("<html>")})
	png, _ := json.Marshal(map[string]any{"data": pngBytes})

	formPut := httptest.NewRequest(http.MethodPut, iconPath, strings.NewReader("url=http://img.local/logo.png"))
	formPut.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formW := httptest.NewRecorder()
	h.ServeHTTP(formW, formPut)

	cases := []struct {
		name string
		w    *httptest.ResponseRecorder
		code int
		says string
	}{
		{"form put", formW, http.StatusUnsupportedMediaType, "application/json"},
		{"neither", send(h, http.MethodPut, iconPath, `{}`), http.StatusBadRequest, "either"},
		{"both", send(h, http.MethodPut, iconPath, `{"url":"http://img.local/logo.png","data":"AAAA"}`), http.StatusBadRequest, "either"},
		{"not base64", send(h, http.MethodPut, iconPath, `{"data":"%%%"}`), http.StatusBadRequest, "body"},
		{"not an image", send(h, http.MethodPut, iconPath, string(notImage)), http.StatusBadRequest, "not a picture"},
		{"bad address", send(h, http.MethodPut, iconPath, `{"url":"javascript:alert(1)"}`), http.StatusBadRequest, "http://"},
		{"unreachable", send(h, http.MethodPut, iconPath, `{"url":"http://gone.local/x.png"}`), http.StatusBadRequest, "could not load"},
		{"unknown app", send(h, http.MethodPut, "/api/custom-apps/ca-nope/icon", string(png)), http.StatusNotFound, "no such app"},
		{"clear unknown", send(h, http.MethodDelete, "/api/custom-apps/ca-nope/icon", ""), http.StatusNotFound, "no such app"},
		{"get unknown", do(h, "/api/custom-apps/ca-nope/icon"), http.StatusNotFound, "no such icon"},
	}
	for _, c := range cases {
		if c.w.Code != c.code || !strings.Contains(decode(t, c.w)["error"].(string), c.says) {
			t.Errorf("%s: %d %s; want %d mentioning %q", c.name, c.w.Code, c.w.Body, c.code, c.says)
		}
	}
}

func TestCustomAppsErrors(t *testing.T) {
	_, _, h := newTestServer(t, &fakeSource{ov: sampleOverview()})
	send(h, http.MethodPost, "/api/custom-apps", `{"name":"A","url":"http://a/"}`)

	formPost := httptest.NewRequest(http.MethodPost, "/api/custom-apps", strings.NewReader("name=x&url=http://evil/"))
	formPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formW := httptest.NewRecorder()
	h.ServeHTTP(formW, formPost)

	cases := []struct {
		name string
		w    *httptest.ResponseRecorder
		code int
		says string
	}{
		{"form post (a cross-site page can send one)", formW, http.StatusUnsupportedMediaType, "application/json"},
		{"bad json", send(h, http.MethodPost, "/api/custom-apps", `{`), http.StatusBadRequest, "body"},
		{"bad scheme", send(h, http.MethodPost, "/api/custom-apps", `{"url":"javascript:alert(1)"}`), http.StatusBadRequest, "http://"},
		{"duplicate", send(h, http.MethodPost, "/api/custom-apps", `{"url":"http://a"}`), http.StatusConflict, "already exists"},
		{"update unknown", send(h, http.MethodPut, "/api/custom-apps/ca-nope", `{"url":"http://b/"}`), http.StatusNotFound, "no such app"},
		{"remove unknown", send(h, http.MethodDelete, "/api/custom-apps/ca-nope", ""), http.StatusNotFound, "no such app"},
	}
	for _, c := range cases {
		if c.w.Code != c.code || !strings.Contains(decode(t, c.w)["error"].(string), c.says) {
			t.Errorf("%s: %d %s; want %d mentioning %q", c.name, c.w.Code, c.w.Body, c.code, c.says)
		}
	}
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
	if !strings.Contains(w.Body.String(), `"app":"homeapps"`) {
		t.Errorf("health must name the app so the shell can tell it from a sibling app: %s", w.Body)
	}
	if w := do(h, "/api/nope"); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"error"`) {
		t.Errorf("unknown API path: %d %s", w.Code, w.Body)
	}
}
