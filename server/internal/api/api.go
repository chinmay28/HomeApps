// Package api is HomeApps' HTTP surface: the catalog the launcher draws, the
// app icons it learned, and its own health. Decoding and encoding only — the
// rules live in catalog, branding and hostman.
//
// The JSON is snake_case with explicit [] and null, the convention of the
// sibling apps; errors are {"error": "..."}.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/chinmay28/homeapps/server/internal/branding"
	"github.com/chinmay28/homeapps/server/internal/catalog"
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

// New returns a server reading from source.
func New(source Source, brands *branding.Cache, version string, log *slog.Logger) *Server {
	return &Server{source: source, brands: brands, version: version, log: log, maxAge: 5 * time.Second, now: time.Now}
}

// Routes mounts the API on mux.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/catalog", s.handleCatalog)
	mux.HandleFunc("GET /api/icons/{key}", s.handleIcon)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no such endpoint")
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
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
			known := s.brands.Lookup(app.URL)
			app.Color = known.Color
			if known.HasIcon {
				app.Icon = "/api/icons/" + known.Key
			}
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
