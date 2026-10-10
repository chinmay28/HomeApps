// Package probe answers "is this app up?" for apps HostMan does not watch:
// the ones added by URL. Like branding, it never blocks a request: Health
// returns the last result and, when that is stale, starts a check in the
// background.
package probe

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Results, matching catalog's health words.
const (
	Up      = "up"
	Down    = "down"
	Unknown = "unknown"
)

// Checker reports one URL's state; Check (an HTTP GET) in production.
type Checker func(ctx context.Context, url string) string

// Check calls an app up when it answers at all below 500: a login page, a
// redirect or a 404 at the root are all a server that is running.
func Check(client *http.Client) Checker {
	return func(ctx context.Context, url string) string {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return Down
		}
		resp, err := client.Do(req)
		if err != nil {
			return Down
		}
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			return Down
		}
		return Up
	}
}

type entry struct {
	state    string
	checked  time.Time
	inflight bool
}

// Cache is safe for concurrent use.
type Cache struct {
	check   Checker
	every   time.Duration
	timeout time.Duration
	now     func() time.Time

	mu      sync.Mutex
	entries map[string]*entry
	wg      sync.WaitGroup
}

// New returns a cache that rechecks each URL at most once per every.
func New(check Checker, every time.Duration) *Cache {
	return &Cache{check: check, every: every, timeout: 5 * time.Second, now: time.Now, entries: map[string]*entry{}}
}

// Health returns url's last known state, Unknown before the first check.
func (c *Cache) Health(url string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[url]
	if e == nil {
		e = &entry{state: Unknown}
		c.entries[url] = e
	}
	if !e.inflight && (e.checked.IsZero() || c.now().Sub(e.checked) >= c.every) {
		e.inflight = true
		c.wg.Add(1)
		go c.run(url)
	}
	return e.state
}

func (c *Cache) run(url string) {
	defer c.wg.Done()
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	state := c.check(ctx, url)
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[url]
	e.state, e.checked, e.inflight = state, c.now(), false
}

// Wait blocks until every check started so far has finished (tests).
func (c *Cache) Wait() { c.wg.Wait() }
