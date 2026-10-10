package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheck(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
	defer srv.Close()
	check := Check(srv.Client())
	cases := map[int]string{200: Up, 302: Up, 401: Up, 404: Up, 500: Down, 503: Down}
	for code, want := range cases {
		status = code
		if got := check(context.Background(), srv.URL); got != want {
			t.Errorf("HTTP %d -> %s, want %s", code, got, want)
		}
	}
	if got := check(context.Background(), "http://127.0.0.1:1/"); got != Down {
		t.Errorf("nothing listening -> %s, want down", got)
	}
}

func TestCacheIsNonBlockingAndRechecksOnSchedule(t *testing.T) {
	var calls atomic.Int32
	c := New(func(context.Context, string) string { calls.Add(1); return Up }, 30*time.Second)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }

	if got := c.Health("u"); got != Unknown {
		t.Errorf("before any check = %s, want unknown", got)
	}
	c.Wait()
	if got := c.Health("u"); got != Up {
		t.Errorf("after the check = %s, want up", got)
	}
	c.Wait()
	now = now.Add(29 * time.Second)
	c.Health("u")
	c.Wait()
	if calls.Load() != 1 {
		t.Errorf("checked %d times inside the interval", calls.Load())
	}
	now = now.Add(2 * time.Second)
	c.Health("u")
	c.Wait()
	if calls.Load() != 2 {
		t.Errorf("checked %d times; a stale result should be rechecked", calls.Load())
	}
}
