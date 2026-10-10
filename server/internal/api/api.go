// Package api is HomeApps' HTTP surface: the catalog the launcher draws, the
// custom apps added by hand, the app icons it learned, and its own health.
// Decoding and encoding only — the rules live in catalog, custom, branding and
// hostman.
//
// The JSON is snake_case with explicit [] and null, the convention of the
// sibling apps; errors are {"error": "..."}.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"sync"
	"time"

	"github.com/chinmay28/homeapps/server/internal/branding"
	"github.com/chinmay28/homeapps/server/internal/catalog"
	"github.com/chinmay28/homeapps/server/internal/custom"
	"github.com/chinmay28/homeapps/server/internal/hostman"
)

// Source is where the hosts and apps come from: a *hostman.Client.
type Source interface {
	Overview(ctx context.Context) (*hostman.Overview, error)
	BaseURL() string
}

// Server holds the dependencies. Build it with New.
type Server struct {
	source  Source
	custom  *custom.Store
	brands  *branding.Cache
	version string
	log     *slog.Logger

	// Phones poll every few seconds and several can be open at once; one
	// HostMan call per window answers all of them.
	mu       sync.Mutex
	cached   *hostman.Overview
	cachedAt time.Time
	maxAge   time.Duration
	now      func() time.Time
}

// New returns a server reading HostMan's apps from source and keeping custom
// apps in apps.
func New(source Source, apps *custom.Store, brands *branding.Cache, version string, log *slog.Logger) *Server {
	return &Server{source: source, custom: apps, brands: brands, version: version, log: log, maxAge: 5 * time.Second, now: time.Now}
}

// Routes mounts the API on mux.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/catalog", s.handleCatalog)
	mux.HandleFunc("GET /api/icons/{key}", s.handleIcon)
	mux.HandleFunc("GET /api/custom-apps", s.handleCustomList)
	mux.HandleFunc("POST /api/custom-apps", s.handleCustomAdd)
	mux.HandleFunc("PUT /api/custom-apps/{id}", s.handleCustomUpdate)
	mux.HandleFunc("DELETE /api/custom-apps/{id}", s.handleCustomRemove)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no such endpoint")
	})
}

// handleHealth names the app as well as its state. Every sibling app answers
// /api/health with {"status","version"}, so without "app" the Android shell
// could not tell HomeApps from, say, HostMan when someone types the wrong port.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"app": "homeapps", "status": "ok", "version": s.version})
}

type catalogResponse struct {
	Hosts     []catalog.Host `json:"hosts"`
	Source    sourceInfo     `json:"source"`
	FetchedAt time.Time      `json:"fetched_at"`
}

type sourceInfo struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	ov, at, err := s.overview(r.Context())
	if err != nil {
		s.log.Warn("catalog: hostman unavailable", "err", err)
		msg := "HostMan is not reachable at " + s.source.BaseURL()
		if errors.Is(err, hostman.ErrUnauthorized) {
			msg = "HostMan asked for a PIN and the one HomeApps has is missing or wrong"
		}
		writeError(w, http.StatusBadGateway, msg)
		return
	}
	c := catalog.Build(ov)
	for i := range c.Hosts {
		for j := range c.Hosts[i].Apps {
			app := &c.Hosts[i].Apps[j]
			app.Icon, app.Color = s.brand(app.URL)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, catalogResponse{
		Hosts:     c.Hosts,
		Source:    sourceInfo{Kind: "hostman", URL: s.source.BaseURL()},
		FetchedAt: at.UTC(),
	})
}

// overview returns a recent overview, fetching one if the cached copy is
// older than maxAge. Holding the lock across the fetch is deliberate: callers
// arriving mid-fetch wait for that answer instead of starting their own.
func (s *Server) overview(ctx context.Context) (*hostman.Overview, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && s.now().Sub(s.cachedAt) < s.maxAge {
		return s.cached, s.cachedAt, nil
	}
	ov, err := s.source.Overview(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	s.cached, s.cachedAt = ov, s.now()
	return ov, s.cachedAt, nil
}

// brand is the icon URL and colour known for an app, either may be "".
func (s *Server) brand(appURL string) (icon, color string) {
	known := s.brands.Lookup(appURL)
	if known.HasIcon {
		icon = "/api/icons/" + known.Key
	}
	return icon, known.Color
}

// customApp is a custom app on the wire: the stored app plus its branding,
// with the same field names as a catalog app so the client draws both alike.
type customApp struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

func (s *Server) wireApp(a custom.App) customApp {
	icon, color := s.brand(a.URL)
	return customApp{ID: a.ID, Name: a.Name, URL: a.URL, Icon: icon, Color: color}
}

// handleCustomList is separate from the catalog on purpose: custom apps do not
// come from HostMan, so they still list, and open, while HostMan is down.
func (s *Server) handleCustomList(w http.ResponseWriter, r *http.Request) {
	apps := s.custom.List()
	out := make([]customApp, 0, len(apps))
	for _, a := range apps {
		out = append(out, s.wireApp(a))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"apps": out})
}

func (s *Server) handleCustomAdd(w http.ResponseWriter, r *http.Request) {
	d, ok := readDraft(w, r)
	if !ok {
		return
	}
	app, err := s.custom.Add(d)
	if err != nil {
		s.writeCustomError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.wireApp(app))
}

func (s *Server) handleCustomUpdate(w http.ResponseWriter, r *http.Request) {
	d, ok := readDraft(w, r)
	if !ok {
		return
	}
	app, err := s.custom.Update(r.PathValue("id"), d)
	if err != nil {
		s.writeCustomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.wireApp(app))
}

func (s *Server) handleCustomRemove(w http.ResponseWriter, r *http.Request) {
	if err := s.custom.Remove(r.PathValue("id")); err != nil {
		s.writeCustomError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// readDraft decodes a {"name","url"} body. It insists on a JSON content type:
// HomeApps has no login, and a page elsewhere can POST a form to it, but it
// cannot send application/json across origins without a CORS preflight that
// this server never answers.
func readDraft(w http.ResponseWriter, r *http.Request) (custom.Draft, bool) {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "send the app as application/json")
		return custom.Draft{}, false
	}
	var d custom.Draft
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&d); err != nil {
		writeError(w, http.StatusBadRequest, "the body must be {\"name\": …, \"url\": …}")
		return custom.Draft{}, false
	}
	return d, true
}

func (s *Server) writeCustomError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, custom.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, custom.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, custom.ErrDuplicate), errors.Is(err, custom.ErrFull):
		writeError(w, http.StatusConflict, err.Error())
	default:
		s.log.Error("custom apps: save failed", "path", s.custom.Path(), "err", err)
		writeError(w, http.StatusInternalServerError, "HomeApps could not save its custom apps")
	}
}

func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	data, typ, ok := s.brands.Icon(r.PathValue("key"))
	if !ok {
		writeError(w, http.StatusNotFound, "no such icon")
		return
	}
	h := w.Header()
	h.Set("Content-Type", typ)
	h.Set("Cache-Control", "public, max-age=86400")
	// Icons are other apps' bytes served from our origin. An SVG opened
	// directly would run its scripts here, so it gets no permissions at all.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	h.Set("X-Content-Type-Options", "nosniff")
	w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
