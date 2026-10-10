package myapps

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sub", "apps.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC) }
	return s, path
}

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"pi5.local:8123":                 "http://pi5.local:8123/",
		"  HTTP://NAS.local/Admin#tab  ": "http://nas.local/Admin",
		"https://router.lan":             "https://router.lan/",
		"192.168.2.1":                    "http://192.168.2.1/",
		"http://[fd00::1]:8080/app?x=1":  "http://[fd00::1]:8080/app?x=1",
	}
	for in, want := range cases {
		got, err := NormalizeURL(in)
		if err != nil || got != want {
			t.Errorf("NormalizeURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", "ftp://pi5", "javascript:alert(1)", "http://", "pi 5.local"} {
		_, err := NormalizeURL(bad)
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("NormalizeURL(%q) = %v; want a ValidationError the user can act on", bad, err)
		}
	}
}

func TestAddListPersist(t *testing.T) {
	s, path := newStore(t)
	a, err := s.Add("  My   Router ", "router.lan")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "My Router" || a.URL != "http://router.lan/" || !strings.HasPrefix(a.ID, "my-") {
		t.Errorf("added %+v", a)
	}
	b, _ := s.Add("", "pi5.local:8123")
	if b.Name != "pi5.local:8123" {
		t.Errorf("an unnamed app should be called by its host and port, got %q", b.Name)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.List()
	if len(got) != 2 || got[0].ID != a.ID || got[1].ID != b.ID || !got[0].CreatedAt.Equal(a.CreatedAt) {
		t.Errorf("after reopening: %+v; the list must survive a restart in order", got)
	}
}

func TestDuplicateURL(t *testing.T) {
	s, _ := newStore(t)
	if _, err := s.Add("a", "pi5.local:8123"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("b", "http://PI5.local:8123/"); !errors.Is(err, ErrDuplicate) {
		t.Errorf("err = %v; the same address spelled differently is still a duplicate", err)
	}
}

func TestUpdate(t *testing.T) {
	s, _ := newStore(t)
	a, _ := s.Add("Old", "a.lan")
	b, _ := s.Add("B", "b.lan")
	name := "New"
	got, err := s.Update(a.ID, &name, nil)
	if err != nil || got.Name != "New" || got.URL != "http://a.lan/" {
		t.Errorf("rename: %+v, %v", got, err)
	}
	u := "c.lan:9000"
	if got, _ = s.Update(a.ID, nil, &u); got.URL != "http://c.lan:9000/" || got.Name != "New" {
		t.Errorf("move: %+v", got)
	}
	taken := "b.lan"
	if _, err := s.Update(a.ID, nil, &taken); !errors.Is(err, ErrDuplicate) {
		t.Errorf("moving onto another app's address: %v", err)
	}
	if _, err := s.Update(b.ID, nil, &taken); err != nil {
		t.Errorf("an app keeping its own address is not a duplicate: %v", err)
	}
	empty := ""
	if got, _ = s.Update(a.ID, &empty, nil); got.Name != "c.lan:9000" {
		t.Errorf("clearing the name falls back to the host, got %q", got.Name)
	}
	if _, err := s.Update("my-nope", &name, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: %v", err)
	}
	long := strings.Repeat("x", 65)
	var ve *ValidationError
	if _, err := s.Update(a.ID, &long, nil); !errors.As(err, &ve) {
		t.Errorf("an over-long name: %v", err)
	}
}

func TestDelete(t *testing.T) {
	s, path := newStore(t)
	a, _ := s.Add("A", "a.lan")
	if err := s.Delete(a.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleting twice: %v", err)
	}
	reopened, _ := Open(path)
	if len(reopened.List()) != 0 {
		t.Error("delete was not saved")
	}
}

func TestOpenRejectsACorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apps.json")
	os.WriteFile(path, []byte("{not json"), 0o644)
	if _, err := Open(path); err == nil {
		t.Error("a corrupt list was silently treated as empty; the next save would erase it")
	}
}

func TestFailedSaveChangesNothing(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "apps.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.Add("A", "a.lan")
	// Make the directory unwritable (root ignores permissions, so use a file
	// where the directory should be).
	s.path = filepath.Join(dir, "apps.json", "nested", "apps.json")
	if _, err := s.Add("B", "b.lan"); err == nil {
		t.Fatal("expected the save to fail")
	}
	if n := len(s.List()); n != 1 {
		t.Errorf("list has %d apps after a failed save; memory must match what is on disk", n)
	}
}
