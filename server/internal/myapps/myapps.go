// Package myapps keeps the apps someone added by URL: the ones HostMan does
// not deploy (a router page, an app on a machine HostMan doesn't manage, a
// service on a NAS). It is the one piece of state HomeApps owns, so every
// device sees the same list.
//
// The list is a small JSON file, rewritten whole and atomically on every
// change: a handful of entries, written rarely, needs no database.
package myapps

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// App is one app added by URL. Its JSON is part of the /api/apps contract.
type App struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

// Errors the API maps to statuses.
var (
	ErrNotFound  = errors.New("no such app")
	ErrDuplicate = errors.New("an app with that address is already in the list")
)

// ValidationError is input the caller can fix (400).
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

const maxName = 64

// NormalizeURL accepts what people paste ("pi5.local:8123",
// "http://nas.local/admin#x") and returns an absolute http(s) URL without a
// fragment. A path is kept: an app may live below the root.
func NormalizeURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", &ValidationError{"enter the app's address, for example pi5.local:8123"}
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || strings.ContainsAny(s, " \t") {
		return "", &ValidationError{fmt.Sprintf("%q is not an http or https address", strings.TrimSpace(raw))}
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), nil
}

// CleanName trims a name and enforces its length; empty is allowed (the
// caller fills in a default).
func CleanName(raw string) (string, error) {
	n := strings.Join(strings.Fields(raw), " ")
	if len([]rune(n)) > maxName {
		return "", &ValidationError{fmt.Sprintf("a name can be at most %d characters", maxName)}
	}
	return n, nil
}

// DefaultName is what an app is called when nobody named it and its manifest
// didn't either: its host, with the port if there is one.
func DefaultName(appURL string) string {
	u, err := url.Parse(appURL)
	if err != nil {
		return appURL
	}
	return u.Host
}

// Store is safe for concurrent use.
type Store struct {
	path string
	now  func() time.Time

	mu   sync.Mutex
	apps []App
}

// Open loads the list at path. A missing file is an empty list, not an error.
func Open(path string) (*Store, error) {
	s := &Store{path: path, now: time.Now, apps: []App{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("myapps: %w", err)
	}
	var file struct {
		Apps []App `json:"apps"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("myapps: %s is not a valid app list: %w", path, err)
	}
	if file.Apps != nil {
		s.apps = file.Apps
	}
	return s, nil
}

// List returns the apps, oldest first.
func (s *Store) List() []App {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.apps)
}

// Add stores a new app. name may be empty; the caller resolves a default
// before calling (see DefaultName).
func (s *Store) Add(name, rawURL string) (App, error) {
	u, err := NormalizeURL(rawURL)
	if err != nil {
		return App{}, err
	}
	n, err := CleanName(name)
	if err != nil {
		return App{}, err
	}
	if n == "" {
		n = DefaultName(u)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.indexByURL(u) >= 0 {
		return App{}, ErrDuplicate
	}
	app := App{ID: newID(), Name: n, URL: u, CreatedAt: s.now().UTC().Truncate(time.Second)}
	next := append(slices.Clone(s.apps), app)
	if err := s.save(next); err != nil {
		return App{}, err
	}
	s.apps = next
	return app, nil
}

// Update changes the name and/or URL of an app; nil leaves a field as is.
func (s *Store) Update(id string, name, rawURL *string) (App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.apps, func(a App) bool { return a.ID == id })
	if i < 0 {
		return App{}, ErrNotFound
	}
	app := s.apps[i]
	if rawURL != nil {
		u, err := NormalizeURL(*rawURL)
		if err != nil {
			return App{}, err
		}
		if j := s.indexByURL(u); j >= 0 && j != i {
			return App{}, ErrDuplicate
		}
		app.URL = u
	}
	if name != nil {
		n, err := CleanName(*name)
		if err != nil {
			return App{}, err
		}
		if n == "" {
			n = DefaultName(app.URL)
		}
		app.Name = n
	}
	next := slices.Clone(s.apps)
	next[i] = app
	if err := s.save(next); err != nil {
		return App{}, err
	}
	s.apps = next
	return app, nil
}

// Delete removes an app.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.apps, func(a App) bool { return a.ID == id })
	if i < 0 {
		return ErrNotFound
	}
	next := slices.Delete(slices.Clone(s.apps), i, i+1)
	if err := s.save(next); err != nil {
		return err
	}
	s.apps = next
	return nil
}

func (s *Store) indexByURL(u string) int {
	return slices.IndexFunc(s.apps, func(a App) bool { return a.URL == u })
}

// save writes the whole list to a temporary file and renames it over the old
// one, so a crash or a full disk never leaves a half-written list.
func (s *Store) save(apps []App) error {
	data, err := json.MarshalIndent(struct {
		Apps []App `json:"apps"`
	}{apps}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("myapps: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".myapps-*.json")
	if err != nil {
		return fmt.Errorf("myapps: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("myapps: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("myapps: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("myapps: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("myapps: %w", err)
	}
	return nil
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b) // crypto/rand does not fail on supported platforms
	return "my-" + hex.EncodeToString(b)
}
