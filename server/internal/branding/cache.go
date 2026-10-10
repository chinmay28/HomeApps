// Package branding remembers each app's icon and colour so the launcher can
// draw them on every poll without fetching anything on the request path.
//
// Lookup never blocks: it answers with whatever is known and, when an entry is
// missing or stale, starts a background fetch. The first catalog after a
// start shows letter tiles; a poll or two later the real icons are there. A
// failed refresh keeps the last good branding — an app that is briefly down
// should not lose its icon.
package branding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/chinmay28/homeapps/server/internal/manifest"
)

// Fetcher loads one app's branding; manifest.Fetch with a client bound in.
type Fetcher func(ctx context.Context, appURL string) (manifest.Brand, error)

// Known is what a Lookup can say about an app right now.
type Known struct {
	Key     string // names the icon for Icon and the /api/icons/{key} URL
	Color   string
	HasIcon bool
}

type entry struct {
	brand    manifest.Brand
	have     bool // brand holds a successful result
	checked  time.Time
	failed   bool
	inflight bool
}

// Cache is safe for concurrent use.
type Cache struct {
	fetch   Fetcher
	now     func() time.Time
	ttl     time.Duration // how long a good result is trusted
	retry   time.Duration // how soon a failed lookup is tried again
	timeout time.Duration
	slots   chan struct{} // bounds concurrent fetches: a Pi, not a crawler

	mu      sync.Mutex
	entries map[string]*entry // by app URL
	byKey   map[string]string // icon key -> app URL
	wg      sync.WaitGroup
}

// New returns a cache that refreshes good results after ttl and retries
// failures after retry.
func New(fetch Fetcher, ttl, retry time.Duration) *Cache {
	return &Cache{
		fetch:   fetch,
		now:     time.Now,
		ttl:     ttl,
		retry:   retry,
		timeout: 15 * time.Second,
		slots:   make(chan struct{}, 4),
		entries: map[string]*entry{},
		byKey:   map[string]string{},
	}
}

// Key is the stable, opaque name of an app URL's icon.
func Key(appURL string) string {
	sum := sha256.Sum256([]byte(appURL))
	return hex.EncodeToString(sum[:8])
}

// Lookup reports what is known about appURL and schedules a fetch if that is
// nothing yet or out of date.
func (c *Cache) Lookup(appURL string) Known {
	key := Key(appURL)
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[appURL]
	if e == nil {
		e = &entry{}
		c.entries[appURL] = e
		c.byKey[key] = appURL
	}
	if !e.inflight && c.due(e) {
		e.inflight = true
		c.wg.Add(1)
		go c.refresh(appURL)
	}
	return Known{Key: key, Color: e.brand.ThemeColor, HasIcon: e.have && e.brand.Icon != nil}
}

func (c *Cache) due(e *entry) bool {
	switch {
	case e.checked.IsZero():
		return true
	case e.failed:
		return c.now().Sub(e.checked) >= c.retry
	default:
		return c.now().Sub(e.checked) >= c.ttl
	}
}

func (c *Cache) refresh(appURL string) {
	defer c.wg.Done()
	c.slots <- struct{}{}
	defer func() { <-c.slots }()

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	b, err := c.fetch(ctx, appURL)

	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[appURL]
	e.inflight = false
	e.checked = c.now()
	e.failed = err != nil
	if err == nil {
		e.brand, e.have = b, true
	}
}

// Icon returns the icon stored under key.
func (c *Cache) Icon(key string) (data []byte, mediaType string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[c.byKey[key]]
	if e == nil || !e.have || e.brand.Icon == nil {
		return nil, "", false
	}
	return e.brand.Icon, e.brand.IconType, true
}

// Wait blocks until every fetch started so far has finished. For tests and
// orderly shutdown.
func (c *Cache) Wait() { c.wg.Wait() }
