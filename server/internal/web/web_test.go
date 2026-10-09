package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func TestEmbeddedShellIsComplete(t *testing.T) {
	// Every file the service worker precaches must be in the binary.
	for _, name := range []string{"", "app.js", "view.js", "app.css", "sw.js", "icon.svg", "manifest.webmanifest"} {
		if w := get(Handler(Assets()), http.MethodGet, "/"+name); w.Code != http.StatusOK {
			t.Errorf("/%s: %d", name, w.Code)
		}
	}
}

func TestHandler(t *testing.T) {
	root := fstest.MapFS{
		"index.html":           {Data: []byte("<title>shell</title>")},
		"app.js":               {Data: []byte("export {}")},
		"manifest.webmanifest": {Data: []byte("{}")},
	}
	h := Handler(root)
	if w := get(h, http.MethodGet, "/some/deep/link"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "shell") {
		t.Errorf("unknown path should get the shell: %d %q", w.Code, w.Body)
	}
	w := get(h, http.MethodGet, "/")
	if w.Header().Get("Cache-Control") != "no-cache" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Errorf("shell headers: %v", w.Header())
	}
	if ct := get(h, http.MethodGet, "/manifest.webmanifest").Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("manifest content type %q", ct)
	}
	if ct := get(h, http.MethodGet, "/app.js").Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("module served as %q; browsers refuse a module without a JS type", ct)
	}
	if w := get(h, http.MethodPost, "/"); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d", w.Code)
	}
	if w := get(h, http.MethodGet, "/../../etc/passwd"); !strings.Contains(w.Body.String(), "shell") {
		t.Errorf("traversal should resolve inside the root: %q", w.Body)
	}
}
