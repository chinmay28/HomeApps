package custom

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestClean(t *testing.T) {
	cases := []struct {
		in   Draft
		want Draft
	}{
		{Draft{"Router", "http://192.168.1.1"}, Draft{"Router", "http://192.168.1.1/"}},
		{Draft{"  My   NAS ", " HTTPS://nas.local:5001/ui "}, Draft{"My NAS", "https://nas.local:5001/ui"}},
		{Draft{"", "https://example.com/app?x=1"}, Draft{"example.com", "https://example.com/app?x=1"}},
		{Draft{"Grafana", "http://pi5.local:3000/#/home"}, Draft{"Grafana", "http://pi5.local:3000/#/home"}},
		{Draft{"v6", "http://[fd00::1]:8080"}, Draft{"v6", "http://[fd00::1]:8080/"}},
	}
	for _, c := range cases {
		got, err := Clean(c.in)
		if err != nil || got != c.want {
			t.Errorf("Clean(%+v) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}
}

func TestCleanRejects(t *testing.T) {
	cases := []struct {
		in   Draft
		says string
	}{
		{Draft{"x", ""}, "required"},
		{Draft{"x", "   "}, "required"},
		{Draft{"x", "pi5.local:8080"}, "http://"},
		{Draft{"x", "javascript:alert(1)"}, "http://"},
		{Draft{"x", "file:///etc/passwd"}, "http://"},
		{Draft{"x", "ftp://nas.local/"}, "http://"},
		{Draft{"x", "http://"}, "no host"},
		{Draft{"x", "http:///path"}, "no host"},
		{Draft{"x", "http://admin:hunter2@router/"}, "password"},
		{Draft{"x", "http://a b/"}, "not an address"},
		{Draft{"x", "http://h/" + strings.Repeat("a", MaxURL)}, "longer"},
		{Draft{strings.Repeat("n", MaxName+1), "http://h/"}, "longer"},
		{Draft{"bad\x00name", "http://h/"}, "control"},
	}
	for _, c := range cases {
		_, err := Clean(c.in)
		if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), c.says) {
			t.Errorf("Clean(%q, %q) = %v; want ErrInvalid saying %q", c.in.Name, c.in.URL, err, c.says)
		}
	}
	if _, err := Clean(Draft{strings.Repeat("é", MaxName), "http://h/"}); err != nil {
		t.Errorf("the name limit counts characters, not bytes: %v", err)
	}
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "state", "custom-apps.json"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	s.newID = func() string { n++; return fmt.Sprintf("ca-%d", n) }
	return s
}

func TestStoreLifecycleSurvivesReopen(t *testing.T) {
	s := newStore(t)
	if got := s.List(); got == nil || len(got) != 0 {
		t.Fatalf("a new store lists %#v; want an empty, non-nil list", got)
	}
	nas, err := s.Add(Draft{"nas", "http://nas.local:5000"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(Draft{"Router", "http://192.168.1.1/"}); err != nil {
		t.Fatal(err)
	}
	if nas.ID != "ca-1" || nas.URL != "http://nas.local:5000/" {
		t.Errorf("added %+v", nas)
	}
	if _, err := s.Update(nas.ID, Draft{"NAS", "https://nas.local:5001"}); err != nil {
		t.Fatal(err)
	}

	again, err := Open(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	want := []App{{ID: "ca-1", Name: "NAS", URL: "https://nas.local:5001/"}, {ID: "ca-2", Name: "Router", URL: "http://192.168.1.1/"}}
	if got := again.List(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("after reopen: %v; want %v (by name, case-insensitively)", got, want)
	}

	if err := again.Remove("ca-1"); err != nil {
		t.Fatal(err)
	}
	if got := again.List(); len(got) != 1 || got[0].ID != "ca-2" {
		t.Errorf("after remove: %v", got)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(s.Path()), ".custom-apps-*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func TestStoreErrors(t *testing.T) {
	s := newStore(t)
	a, _ := s.Add(Draft{"A", "http://a/"})
	b, _ := s.Add(Draft{"B", "http://b/"})

	if _, err := s.Add(Draft{"again", "HTTP://a"}); !errors.Is(err, ErrDuplicate) {
		t.Errorf("same address after cleaning: %v; want ErrDuplicate", err)
	}
	if _, err := s.Update(b.ID, Draft{"B", "http://a/"}); !errors.Is(err, ErrDuplicate) {
		t.Errorf("update onto another app's address: %v; want ErrDuplicate", err)
	}
	if _, err := s.Update(a.ID, Draft{"A renamed", "http://a/"}); err != nil {
		t.Errorf("an app keeping its own address is not a duplicate: %v", err)
	}
	if _, err := s.Update("ca-404", Draft{"x", "http://x/"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("update unknown: %v", err)
	}
	if err := s.Remove("ca-404"); !errors.Is(err, ErrNotFound) {
		t.Errorf("remove unknown: %v", err)
	}
	if _, err := s.Add(Draft{"x", "nope"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("invalid draft: %v", err)
	}
	if len(s.List()) != 2 {
		t.Errorf("failed changes must not change the list: %v", s.List())
	}
}

func TestStoreIsBounded(t *testing.T) {
	s := newStore(t)
	for i := range MaxApps {
		if _, err := s.Add(Draft{"", fmt.Sprintf("http://h%d/", i)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Add(Draft{"", "http://one-more/"}); !errors.Is(err, ErrFull) {
		t.Errorf("past the limit: %v; want ErrFull", err)
	}
}

func TestFailedSaveLeavesMemoryAlone(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(filepath.Join(dir, "custom-apps.json"))
	if _, err := s.Add(Draft{"A", "http://a/"}); err != nil {
		t.Fatal(err)
	}
	// Turn the target into a directory: the rename fails, the list must not move.
	os.Remove(s.Path())
	os.Mkdir(s.Path(), 0o755)
	if _, err := s.Add(Draft{"B", "http://b/"}); err == nil {
		t.Fatal("save into a directory succeeded")
	}
	if len(s.List()) != 1 {
		t.Errorf("memory ran ahead of the disk: %v", s.List())
	}
}

func TestOpenRefusesABadFile(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"garbage": "{nope", "no-id": `{"apps":[{"name":"x","url":"http://x/"}]}`} {
		p := filepath.Join(dir, name+".json")
		os.WriteFile(p, []byte(body), 0o644)
		if _, err := Open(p); err == nil {
			t.Errorf("%s: opened; a bad file must stop the server, not be overwritten", name)
		}
	}
}

// pngBytes is a minimal image that sniffs as PNG.
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 24)...)

func TestCleanImage(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	cases := []struct {
		name     string
		data     []byte
		declared string
		want     string // media type, or the error's words
		ok       bool
	}{
		{"png sniffed", pngBytes, "", "image/png", true},
		{"png declared as anything", pngBytes, "text/plain", "image/png", true},
		{"svg declared", svg, "image/svg+xml", "image/svg+xml", true},
		{"svg undeclared", svg, "", "not a picture", false},
		{"html", []byte("<html>hi</html>"), "image/png", "not a picture", false},
		{"empty", nil, "image/png", "empty", false},
		{"too big", append(slices.Clone(pngBytes), make([]byte, MaxIcon)...), "", "larger", false},
	}
	for _, c := range cases {
		img, err := CleanImage(c.data, c.declared)
		switch {
		case c.ok && (err != nil || img.Type != c.want):
			t.Errorf("%s: %q, %v; want %q", c.name, img.Type, err, c.want)
		case !c.ok && (!errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: %v; want ErrInvalid saying %q", c.name, err, c.want)
		}
	}
}

func TestCleanURL(t *testing.T) {
	if got, err := CleanURL(" HTTPS://cdn.example/logo.png "); err != nil || got != "https://cdn.example/logo.png" {
		t.Errorf("CleanURL = %q, %v", got, err)
	}
	if _, err := CleanURL("data:image/png;base64,AAAA"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a data: URL is not an address: %v", err)
	}
}

func TestStoreIcons(t *testing.T) {
	s := newStore(t)
	a, _ := s.Add(Draft{"A", "http://a/"})

	got, err := s.SetIcon(a.ID, Image{Data: pngBytes})
	if err != nil || got.Icon == nil || got.Icon.Type != "image/png" {
		t.Fatalf("SetIcon = %+v, %v", got, err)
	}
	if got, ok := s.Get(a.ID); !ok || got.Icon == nil {
		t.Errorf("Get = %+v, %v; want the app with its icon", got, ok)
	}
	if _, ok := s.Get("ca-404"); ok {
		t.Error("Get found an app that was never added")
	}
	if got, _ := s.Update(a.ID, Draft{"A2", "http://a2/"}); got.Icon == nil {
		t.Error("renaming an app must keep the icon it was given")
	}
	again, err := Open(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if l := again.List(); len(l) != 1 || l[0].Icon == nil || !slices.Equal(l[0].Icon.Data, pngBytes) {
		t.Errorf("the icon must survive a reopen: %+v", l)
	}
	if got, err := again.ClearIcon(a.ID); err != nil || got.Icon != nil {
		t.Errorf("ClearIcon = %+v, %v", got, err)
	}
	if data, _ := os.ReadFile(s.Path()); strings.Contains(string(data), `"icon"`) {
		t.Errorf("an app without its own icon writes no icon field:\n%s", data)
	}

	if _, err := s.SetIcon("ca-404", Image{Data: pngBytes}); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetIcon unknown: %v", err)
	}
	if _, err := s.ClearIcon("ca-404"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ClearIcon unknown: %v", err)
	}
	if _, err := s.SetIcon(a.ID, Image{Data: []byte("nope")}); !errors.Is(err, ErrInvalid) {
		t.Errorf("SetIcon non-image: %v", err)
	}
}
