// Package web serves the launcher's static client: hand-written HTML, CSS and
// ES modules embedded in the binary, so there is no front-end build step and
// one file to deploy.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed assets
var embedded embed.FS

// Assets is the embedded client.
func Assets() fs.FS {
	sub, _ := fs.Sub(embedded, "assets") // the directory is compiled in
	return sub
}

// Handler serves files from root. Anything that is not a file gets the shell,
// so a deep link or an old bookmark still opens the launcher.
func Handler(root fs.FS) http.Handler {
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if st, err := fs.Stat(root, name); err != nil || st.IsDir() {
			name = "index.html"
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		h := w.Header()
		// Nothing is content-hashed, so the browser must revalidate every
		// file; ETag/Last-Modified keep that to a 304 on a Pi.
		h.Set("Cache-Control", "no-cache")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		if name == "index.html" {
			h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		}
		if strings.HasSuffix(name, ".webmanifest") {
			h.Set("Content-Type", "application/manifest+json")
		}
		files.ServeHTTP(w, r)
	})
}
