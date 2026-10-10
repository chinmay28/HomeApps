// Package hostman reads the hosts and installed apps from a HostMan server.
//
// HostMan already knows every machine, every app deployed to it, the port it
// answers on and whether its health check passes, so HomeApps keeps no list of
// its own: it asks GET /api/overview and shapes the answer. Only the fields
// HomeApps uses are decoded; HostMan may add more without breaking this.
//
// HostMan can be gated by a PIN. Its sessions live in memory, so a HostMan
// restart forgets ours: on a 401 the client logs in again with the PIN it was
// given and retries once.
package hostman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// ErrUnauthorized means HostMan wants a PIN and we have none, or ours is wrong.
var ErrUnauthorized = errors.New("hostman: PIN required or rejected")

// Overview is the part of HostMan's GET /api/overview that HomeApps reads.
type Overview struct {
	Hosts         []Host         `json:"hosts"`
	Installations []Installation `json:"installations"`
}

// Host is one machine HostMan manages.
type Host struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Address    string     `json:"address"`
	Status     string     `json:"status"` // "online" | "offline" | "" (never seen)
	IsSelf     bool       `json:"isSelf"`
	LastSeenAt *time.Time `json:"lastSeenAt"`
	Latest     *Sample    `json:"latest"`
}

// Sample is a host's most recent telemetry point.
type Sample struct {
	CPUPct   float64 `json:"cpuPct"`
	MemUsed  int64   `json:"memUsed"`
	MemTotal int64   `json:"memTotal"`
}

// Installation is one app deployed to one host.
type Installation struct {
	ID           int64  `json:"id"`
	AppID        int64  `json:"appId"`
	HostID       int64  `json:"hostId"`
	AppName      string `json:"appName"`
	HostName     string `json:"hostName"`
	HealthStatus string `json:"healthStatus"` // passing | failing | unknown | unchecked
	HealthDetail string `json:"healthDetail"`
	Ports        []int  `json:"ports"`
	URL          string `json:"url"` // empty when HostMan can't tell where to open it
	Version      string `json:"version"`
}

// Client talks to one HostMan server. The zero value is not usable; use New.
type Client struct {
	base *url.URL
	pin  string
	http *http.Client
}

// New returns a client for the HostMan at baseURL (e.g. http://pi.local:8899).
// pin may be empty for a HostMan that runs without one.
func New(baseURL, pin string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("hostman: %q is not an http(s) URL", baseURL)
	}
	jar, _ := cookiejar.New(nil) // never fails without options
	return &Client{
		base: u,
		pin:  pin,
		http: &http.Client{Jar: jar, Timeout: 10 * time.Second},
	}, nil
}

// BaseURL is the HostMan address, for display.
func (c *Client) BaseURL() string { return c.base.String() }

// Overview fetches the hosts and installations.
func (c *Client) Overview(ctx context.Context) (*Overview, error) {
	resp, err := c.get(ctx, "/api/overview")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if err := c.login(ctx); err != nil {
			return nil, err
		}
		if resp, err = c.get(ctx, "/api/overview"); err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("hostman: overview: HTTP %d", resp.StatusCode)
	}
	var ov Overview
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&ov); err != nil {
		return nil, fmt.Errorf("hostman: overview: %w", err)
	}
	return &ov, nil
}

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base.String()+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hostman: %w", err)
	}
	return resp, nil
}

func (c *Client) login(ctx context.Context) error {
	if c.pin == "" {
		return ErrUnauthorized
	}
	body, _ := json.Marshal(map[string]string{"pin": c.pin})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base.String()+"/api/session", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("hostman: login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ErrUnauthorized
	}
	return nil
}
