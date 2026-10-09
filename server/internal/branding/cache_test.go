package branding

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/chinmay28/homeapps/server/internal/manifest"
)

// scripted is a Fetcher whose answers the test sets, counting calls.
type scripted struct {
	mu    sync.Mutex
	calls int
	brand manifest.Brand
	err   error
}

func (s *scripted) fetch(context.Context, string) (manifest.Brand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.brand, s.err
}

func (s *scripted) set(b manifest.Brand, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.brand, s.err = b, err
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newCache(s *scripted) (*Cache, *clock) {
	clk := &clock{t: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}
	c := New(s.fetch, time.Hour, time.Minute)
	c.now = clk.now
	return c, clk
}

var vault = manifest.Brand{ThemeColor: "#e7d3a8", Icon: []byte("png"), IconType: "image/png"}

func TestLookupIsNonBlockingThenFilledIn(t *testing.T) {
	s := &scripted{brand: vault}
	c, _ := newCache(s)
	first := c.Lookup("http://pi5:8123/")
	if first.HasIcon || first.Color != "" {
		t.Fatalf("first lookup = %+v; nothing can be known before the fetch", first)
	}
	c.Wait()
	got := c.Lookup("http://pi5:8123/")
	if !got.HasIcon || got.Color != "#e7d3a8" || got.Key != first.Key {
		t.Errorf("after the fetch = %+v", got)
	}
	data, typ, ok := c.Icon(got.Key)
	if !ok || string(data) != "png" || typ != "image/png" {
		t.Errorf("Icon(%s) = %q %q %v", got.Key, data, typ, ok)
	}
}

func TestRefreshSchedule(t *testing.T) {
	s := &scripted{brand: vault}
	c, clk := newCache(s)
	c.Lookup("u")
	c.Wait()
	clk.t = clk.t.Add(59 * time.Minute)
	c.Lookup("u")
	c.Wait()
	if s.calls != 1 {
		t.Fatalf("calls = %d; a good result is trusted for the TTL", s.calls)
	}
	clk.t = clk.t.Add(2 * time.Minute)
	c.Lookup("u")
	c.Wait()
	if s.calls != 2 {
		t.Errorf("calls = %d; a stale result should be refreshed", s.calls)
	}
}

func TestFailedRefreshKeepsLastGoodBranding(t *testing.T) {
	s := &scripted{brand: vault}
	c, clk := newCache(s)
	c.Lookup("u")
	c.Wait()
	s.set(manifest.Brand{}, errors.New("connection refused"))
	clk.t = clk.t.Add(2 * time.Hour)
	c.Lookup("u")
	c.Wait()
	if got := c.Lookup("u"); !got.HasIcon {
		t.Error("an app that is briefly down lost its icon")
	}
	clk.t = clk.t.Add(30 * time.Second)
	c.Lookup("u")
	c.Wait()
	if s.calls != 2 {
		t.Errorf("calls = %d; a failure is retried after the retry interval, not on every poll", s.calls)
	}
	clk.t = clk.t.Add(time.Minute)
	c.Lookup("u")
	c.Wait()
	if s.calls != 3 {
		t.Errorf("calls = %d; want a retry once the interval passed", s.calls)
	}
}

func TestConcurrentLookupsShareOneFetch(t *testing.T) {
	block := make(chan struct{})
	var calls int
	var mu sync.Mutex
	c := New(func(context.Context, string) (manifest.Brand, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		<-block
		return vault, nil
	}, time.Hour, time.Minute)
	for range 10 {
		c.Lookup("u")
	}
	close(block)
	c.Wait()
	if calls != 1 {
		t.Errorf("calls = %d; lookups while a fetch is in flight must not start another", calls)
	}
}

func TestIconUnknownKey(t *testing.T) {
	c, _ := newCache(&scripted{})
	if _, _, ok := c.Icon("nope"); ok {
		t.Error("served an icon for a key never handed out")
	}
	if Key("a") == Key("b") || len(Key("a")) != 16 {
		t.Error("keys should be 16 hex chars and distinct per URL")
	}
}
