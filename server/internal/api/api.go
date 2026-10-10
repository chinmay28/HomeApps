// Package api is HomeApps' HTTP surface: the catalog the launcher draws, the
// apps added by URL, the app icons it learned, and its own health. Decoding
// and encoding only: the rules live in catalog, myapps, branding and hostman.
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
	"github.com/chinmay28/homeapps/server/internal/myapps"
	"github.com/chinmay28/homeapps/server/internal/probe"
)

// Source is where the hosts and apps come from: a *hostman.Client.
type Source interface {
	Overview(ctx context.Context) (*hostman.Overview, error)
	BaseURL() string
}

// Config is what a Server is built from. Source and MyApps may be nil: with
// no HostMan the launcher shows only the apps added by URL, and with no store
// it shows only HostMan's.
type Config struct {
	Source  Source
	MyApps  *myapps.Store
	Probes  *probe.Cache // health of the apps added by URL; needed with MyApps
	Brands  *branding.Cache
	NameOf  func(ctx context.Context, url string) string // an app's own name, for an unnamed add; may be nil
	Version string
	Log     *slog.Logger
}

// Server holds the dependencies. Build it with New.
type Server struct {
	Config

	// Phones poll every few seconds and several can be open at once; one
	// HostMan call per window answers all of them.
	mu       sync.Mutex
	cached   *hostman.Overview
	cachedAt time.Time
	maxAge   time.Duration
	now      func() time.Time
}

// New returns a server for cfg.
func New(cfg Config) *Server {
	return &Server{Config: cfg, maxAge: 5 * time.Second, now: time.Now}
}

// Routes mounts the API on mux.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/catalog", s.handleCatalog)
	mux.HandleFunc("GET /api/icons/{key}", s.handleIcon)
	if s.MyApps != nil {
		mux.HandleFunc("GET /api/apps", s.handleListMyApps)
		mux.HandleFunc("POST /api/apps", s.handleAddMyApp)
		mux.HandleFunc("PATCH /api/apps/{id}", s.handleUpdateMyApp)
		mux.HandleFunc("DELETE /api/apps/{id}", s.handleDeleteMyApp)
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no such endpoint")
	})
}

// handleHealth names the app as well as its state. Every sibling app answers
// /api/health with {"status","version"}, so without "app" the Android shell
// could not tell HomeApps from, say, HostMan when someone types the wrong port.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"app": "homeapps", "status": "ok", "version": s.Version})
}

type catalogResponse struct {
	// MyApps are the apps added by URL; Hosts are HostMan's. Both are always
	// arrays. HostManError says why Hosts is empty when HostMan could not be
	// read, so the client can keep showing the last hosts it had.
	MyApps       []catalog.App  `json:"my_apps"`
	Hosts        []catalog.Host `json:"hosts"`
	Source       *sourceInfo    `json:"source"`
	HostManError string         `json:"hostman_error"`
	FetchedAt    time.Time      `json:"fetched_at"`
}

type sourceInfo struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

// handleCatalog answers 200 even when HostMan is down: the apps added by URL
// do not depend on it, and a launcher that empties itself because one source
// failed is a launcher nobody trusts.
func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	resp := catalogResponse{MyApps: []catalog.App{}, Hosts: []catalog.Host{}, FetchedAt: s.now().UTC()}
	if s.MyApps != nil {
		resp.MyApps = catalog.FromMyApps(s.MyApps.List(), s.Probes.Health)
		s.brand(resp.MyApps)
	}
	if s.Source != nil {
		resp.Source = &sourceInfo{Kind: "hostman", URL: s.Source.BaseURL()}
		ov, at, err := s.overview(r.Context())
		if err != nil {
			s.Log.Warn("catalog: hostman unavailable", "err", err)
			resp.HostManError = "HostMan is not reachable at " + s.Source.BaseURL() + "."
			if errors.Is(err, hostman.ErrUnauthorized) {
				resp.HostManError = "HostMan asked for a PIN and the one HomeApps has is missing or wrong."
			}
		} else {
			resp.Hosts = catalog.Build(ov).Hosts
			resp.FetchedAt = at.UTC()
			for i := range resp.Hosts {
				s.brand(resp.Hosts[i].Apps)
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, resp)
}

// brand fills in each app's icon and colour from what its manifest declared.
func (s *Server) brand(apps []catalog.App) {
	for i := range apps {
		known := s.Brands.Lookup(apps[i].URL)
		apps[i].Color = known.Color
		if known.HasIcon {
			apps[i].Icon = "/api/icons/" + known.Key
		}
	}
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
	ov, err := s.Source.Overview(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	s.cached, s.cachedAt = ov, s.now()
	return ov, s.cachedAt, nil
}

func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	data, typ, ok := s.Brands.Icon(r.PathValue("key"))
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
