package hostman

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeHostMan answers like HostMan: /api/overview behind an optional PIN whose
// sessions it can forget, as a restarted HostMan does.
type fakeHostMan struct {
	pin      string
	sessions map[string]bool
	logins   int
}

func (f *fakeHostMan) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/session":
		f.logins++
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body struct{ PIN string }
		_ = decode(r, &body)
		if body.PIN != f.pin {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.sessions["tok"] = true
		http.SetCookie(w, &http.Cookie{Name: "hostman_session", Value: "tok", Path: "/"})
		w.Write([]byte(`{"authenticated":true}`))
	case "/api/overview":
		if f.pin != "" {
			c, err := r.Cookie("hostman_session")
			if err != nil || !f.sessions[c.Value] {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		w.Write([]byte(`{"hosts":[{"id":1,"name":"pi5","address":"pi5.local","status":"online","isSelf":true,
			"latest":{"cpuPct":2.5,"memUsed":1,"memTotal":4},"hostKey":"ignored"}],
			"installations":[{"id":7,"appId":3,"hostId":1,"appName":"SAND Vault","healthStatus":"passing",
			"ports":[8123],"url":"http://pi5.local:8123/","version":"v2026.9.40","params":{"x":"y"}}]}`))
	default:
		http.NotFound(w, r)
	}
}

func newFake(t *testing.T, pin string) (*fakeHostMan, *Client) {
	t.Helper()
	f := &fakeHostMan{pin: pin, sessions: map[string]bool{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, mustNew(t, srv.URL, pin)
}

func mustNew(t *testing.T, base, pin string) *Client {
	t.Helper()
	c, err := New(base, pin)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestOverviewWithoutPIN(t *testing.T) {
	_, c := newFake(t, "")
	ov, err := c.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Hosts) != 1 || !ov.Hosts[0].IsSelf || ov.Hosts[0].Latest.CPUPct != 2.5 {
		t.Errorf("host fields not decoded from HostMan's camelCase: %+v", ov.Hosts)
	}
	in := ov.Installations[0]
	if in.AppName != "SAND Vault" || in.URL != "http://pi5.local:8123/" || in.Ports[0] != 8123 || in.HealthStatus != "passing" {
		t.Errorf("installation fields not decoded: %+v", in)
	}
}

func TestOverviewLogsInWithPIN(t *testing.T) {
	f, c := newFake(t, "1234")
	if _, err := c.Overview(context.Background()); err != nil {
		t.Fatalf("a 401 should be answered with a login and one retry: %v", err)
	}
	if _, err := c.Overview(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.logins != 1 {
		t.Errorf("logged in %d times; the session cookie should have been reused", f.logins)
	}
}

func TestOverviewLogsInAgainAfterHostManRestart(t *testing.T) {
	f, c := newFake(t, "1234")
	if _, err := c.Overview(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.sessions = map[string]bool{} // HostMan restarted and forgot every session
	if _, err := c.Overview(context.Background()); err != nil {
		t.Fatalf("a forgotten session should be renewed transparently: %v", err)
	}
	if f.logins != 2 {
		t.Errorf("logins = %d, want 2", f.logins)
	}
}

func TestOverviewUnauthorized(t *testing.T) {
	cases := map[string]string{"no PIN configured": "", "wrong PIN": "0000"}
	for name, pin := range cases {
		t.Run(name, func(t *testing.T) {
			f := &fakeHostMan{pin: "1234", sessions: map[string]bool{}}
			srv := httptest.NewServer(f)
			defer srv.Close()
			_, err := mustNew(t, srv.URL, pin).Overview(context.Background())
			if !errors.Is(err, ErrUnauthorized) {
				t.Errorf("err = %v, want ErrUnauthorized so the UI can say the PIN is the problem", err)
			}
		})
	}
}

func TestNewRejectsNonHTTP(t *testing.T) {
	for _, bad := range []string{"", "pi5.local:8899", "ftp://pi5", "http://"} {
		if _, err := New(bad, ""); err == nil {
			t.Errorf("New(%q) accepted a URL it cannot call", bad)
		}
	}
	c := mustNew(t, "http://pi5.local:8899/", "")
	if c.BaseURL() != "http://pi5.local:8899" {
		t.Errorf("trailing slash kept: %q", c.BaseURL())
	}
}

func decode(r *http.Request, v any) error { return json.NewDecoder(r.Body).Decode(v) }
