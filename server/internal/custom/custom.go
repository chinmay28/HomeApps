// Package custom keeps the apps someone added to HomeApps by hand: any http
// or https address, with a name. They sit beside the apps HostMan deploys,
// for the things HostMan does not run (a router's admin page, a NAS, a site
// on the internet).
//
// It is the one piece of state HomeApps owns. Clean is a pure function of
// its input; Store is a small file-backed list that writes the whole file
// atomically on every change, so a crash leaves either the old list or the
// new one, never half of either.
package custom

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/chinmay28/homeapps/server/internal/manifest"
)

const (
	// MaxApps bounds the file; a launcher with more than this is not a home.
	MaxApps = 200
	// MaxURL matches the Android shell's limit on what it will open.
	MaxURL  = 2048
	MaxName = 64
	// MaxIcon bounds an icon someone gave an app. The client shrinks a picked
	// picture to 192px before sending it, which is a few KiB.
	MaxIcon = 256 << 10
	// IDPrefix tells a custom app's ID from HostMan's "hm-" ones.
	IDPrefix = "ca-"
)

var (
	ErrInvalid   = errors.New("invalid app")
	ErrNotFound  = errors.New("no such app")
	ErrDuplicate = errors.New("an app with that address already exists")
	ErrFull      = fmt.Errorf("at most %d custom apps", MaxApps)
)

// App is one custom app. ID is stable for the app's lifetime; the Android
// shell keys its running WebViews by it. Icon is nil unless someone gave the
// app one; it is then drawn instead of whatever the app's manifest declares.
type App struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Icon *Image `json:"icon,omitempty"`
}

// Image is an icon's bytes and media type. Treat it as read-only: List hands
// out the stored one.
type Image struct {
	Type string `json:"type"`
	Data []byte `json:"data"` // base64 in the file
}

// Draft is what a person typed: an app before it has an ID.
type Draft struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Clean validates and normalises a draft. The URL must be an absolute http
// or https address with a host and no credentials (the list is shown to
// everyone on the network). A blank name becomes the URL's host name.
// Errors wrap ErrInvalid and say what to fix.
func Clean(d Draft) (Draft, error) {
	u, err := cleanURL(d.URL)
	if err != nil {
		return Draft{}, err
	}

	name := strings.Join(strings.Fields(d.Name), " ")
	if strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return Draft{}, fmt.Errorf("%w: the name has control characters", ErrInvalid)
	}
	if utf8.RuneCountInString(name) > MaxName {
		return Draft{}, fmt.Errorf("%w: the name is longer than %d characters", ErrInvalid, MaxName)
	}
	if name == "" {
		name = u.Hostname()
	}
	return Draft{Name: name, URL: u.String()}, nil
}

// CleanURL applies Clean's rules for an address to raw alone: an absolute
// http or https URL with a host and no credentials. It is what an icon's
// address must be too.
func CleanURL(raw string) (string, error) {
	u, err := cleanURL(raw)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func cleanURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%w: an address is required", ErrInvalid)
	}
	if len(raw) > MaxURL {
		return nil, fmt.Errorf("%w: the address is longer than %d characters", ErrInvalid, MaxURL)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not an address", ErrInvalid, raw)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%w: the address must start with http:// or https://", ErrInvalid)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("%w: the address has no host", ErrInvalid)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: the address must not contain a user name or password", ErrInvalid)
	}
	if u.Path == "" && u.RawQuery == "" && u.Fragment == "" {
		u.Path = "/"
	}
	return u, nil
}

// CleanImage checks that data is an image worth keeping as an icon and names
// its media type from the bytes. declared is only needed for SVG, which does
// not sniff as an image. Errors wrap ErrInvalid.
func CleanImage(data []byte, declared string) (Image, error) {
	if len(data) == 0 {
		return Image{}, fmt.Errorf("%w: the icon is empty", ErrInvalid)
	}
	if len(data) > MaxIcon {
		return Image{}, fmt.Errorf("%w: the icon is larger than %d KiB", ErrInvalid, MaxIcon>>10)
	}
	typ := manifest.ImageType(declared, "", data)
	if typ == "" {
		return Image{}, fmt.Errorf("%w: the icon is not a picture", ErrInvalid)
	}
	return Image{Type: typ, Data: data}, nil
}

// file is the on-disk shape. Version lets a later format be told apart.
type file struct {
	Version int   `json:"version"`
	Apps    []App `json:"apps"`
}

// Store is the list of custom apps, saved at one path. It is safe for
// concurrent use.
type Store struct {
	path  string
	newID func() string

	mu   sync.Mutex
	apps []App
}

// Open loads the list at path. A missing file is an empty list; the file and
// its directory are created on the first change. A file that cannot be read
// is an error rather than an empty list, so a bad edit is never overwritten.
func Open(path string) (*Store, error) {
	s := &Store{path: path, newID: randomID, apps: []App{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for _, a := range f.Apps {
		if a.ID == "" {
			return nil, fmt.Errorf("%s: an app has no id", path)
		}
		s.apps = append(s.apps, a)
	}
	return s, nil
}

// Path is where the list is saved.
func (s *Store) Path() string { return s.path }

// List returns every app, by name. The slice is the caller's.
func (s *Store) List() []App {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := slices.Clone(s.apps)
	slices.SortStableFunc(out, func(a, b App) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), strings.Compare(a.ID, b.ID))
	})
	return out
}

// Get returns the app with id.
func (s *Store) Get(id string) (App, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.indexOf(id); i >= 0 {
		return s.apps[i], true
	}
	return App{}, false
}

// Add cleans d and saves it as a new app.
func (s *Store) Add(d Draft) (App, error) {
	d, err := Clean(d)
	if err != nil {
		return App{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.apps) >= MaxApps {
		return App{}, ErrFull
	}
	if s.indexOfURL(d.URL, "") >= 0 {
		return App{}, ErrDuplicate
	}
	app := App{ID: s.newID(), Name: d.Name, URL: d.URL}
	next := append(slices.Clone(s.apps), app)
	if err := s.save(next); err != nil {
		return App{}, err
	}
	s.apps = next
	return app, nil
}

// Update replaces the name and address of the app with id, keeping its icon.
func (s *Store) Update(id string, d Draft) (App, error) {
	d, err := Clean(d)
	if err != nil {
		return App{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexOf(id)
	if i < 0 {
		return App{}, ErrNotFound
	}
	if s.indexOfURL(d.URL, id) >= 0 {
		return App{}, ErrDuplicate
	}
	app := s.apps[i]
	app.Name, app.URL = d.Name, d.URL
	return s.replace(i, app)
}

// SetIcon gives the app with id its own icon, cleaned by CleanImage.
func (s *Store) SetIcon(id string, img Image) (App, error) {
	img, err := CleanImage(img.Data, img.Type)
	if err != nil {
		return App{}, err
	}
	return s.setIcon(id, &img)
}

// ClearIcon takes back the icon SetIcon gave, so the app's own shows again.
func (s *Store) ClearIcon(id string) (App, error) { return s.setIcon(id, nil) }

func (s *Store) setIcon(id string, img *Image) (App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexOf(id)
	if i < 0 {
		return App{}, ErrNotFound
	}
	app := s.apps[i]
	app.Icon = img
	return s.replace(i, app)
}

// replace saves the list with app at i. The caller holds mu.
func (s *Store) replace(i int, app App) (App, error) {
	next := slices.Clone(s.apps)
	next[i] = app
	if err := s.save(next); err != nil {
		return App{}, err
	}
	s.apps = next
	return app, nil
}

// Remove deletes the app with id.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexOf(id)
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

func (s *Store) indexOf(id string) int {
	return slices.IndexFunc(s.apps, func(a App) bool { return a.ID == id })
}

func (s *Store) indexOfURL(u, except string) int {
	return slices.IndexFunc(s.apps, func(a App) bool { return a.URL == u && a.ID != except })
}

// save writes apps to a temporary file beside the real one and renames it
// into place. The caller holds mu and swaps s.apps only after this succeeds,
// so memory never runs ahead of the disk.
func (s *Store) save(apps []App) error {
	data, err := json.MarshalIndent(file{Version: 1, Apps: apps}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".custom-apps-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op after a successful rename
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

func randomID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b) // never fails on the platforms Go supports
	return IDPrefix + hex.EncodeToString(b)
}
