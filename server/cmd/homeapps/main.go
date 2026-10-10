// Command homeapps serves the HomeApps launcher: the web client and an API
// that lists every app HostMan has deployed, with each app's own icon, plus
// the custom apps added by hand.
//
//	homeapps [serve] [--host H] [--port N] [--hostman URL] [--data DIR] [--web-dist DIR]
//	homeapps version
//
// Each flag overrides its environment fallback (HOST, PORT, HOMEAPPS_HOSTMAN,
// HOMEAPPS_DATA, then systemd's STATE_DIRECTORY, WEB_DIST). The HostMan PIN is read only from HOMEAPPS_HOSTMAN_PIN, so it
// never shows up in a process listing.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chinmay28/homeapps/server/internal/api"
	"github.com/chinmay28/homeapps/server/internal/branding"
	"github.com/chinmay28/homeapps/server/internal/custom"
	"github.com/chinmay28/homeapps/server/internal/hostman"
	"github.com/chinmay28/homeapps/server/internal/manifest"
	"github.com/chinmay28/homeapps/server/internal/version"
	"github.com/chinmay28/homeapps/server/internal/web"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-v":
			fmt.Println("homeapps", version.String())
			return
		case "serve":
			args = args[1:]
		}
	}
	if err := serve(args); err != nil {
		fmt.Fprintln(os.Stderr, "homeapps:", err)
		os.Exit(1)
	}
}

type config struct {
	host, hostmanURL, dataDir, webDist string
	port                               int
}

func parseFlags(args []string, getenv func(string) string) (config, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	c := config{}
	fs.StringVar(&c.host, "host", envOr(getenv, "HOST", "0.0.0.0"), "address to listen on")
	port, err := strconv.Atoi(envOr(getenv, "PORT", "8686"))
	if err != nil {
		return c, fmt.Errorf("PORT: %w", err)
	}
	fs.IntVar(&c.port, "port", port, "port to listen on")
	fs.StringVar(&c.hostmanURL, "hostman", envOr(getenv, "HOMEAPPS_HOSTMAN", "http://127.0.0.1:8899"), "HostMan to read hosts and apps from")
	fs.StringVar(&c.dataDir, "data", defaultDataDir(getenv), "directory for the custom apps list")
	fs.StringVar(&c.webDist, "web-dist", getenv("WEB_DIST"), "serve the web client from this directory instead of the embedded copy")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if c.port < 1 || c.port > 65535 {
		return c, fmt.Errorf("port %d out of range", c.port)
	}
	return c, nil
}

// defaultDataDir is HOMEAPPS_DATA, else the directory systemd made for the
// unit's StateDirectory=, else ~/.config/homeapps for a run by hand.
func defaultDataDir(getenv func(string) string) string {
	if v := getenv("HOMEAPPS_DATA"); v != "" {
		return v
	}
	if v := getenv("STATE_DIRECTORY"); v != "" {
		// systemd lists one path per StateDirectory= entry; ours is the first.
		first, _, _ := strings.Cut(v, ":")
		return first
	}
	if home := getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "homeapps")
	}
	return "homeapps-data"
}

func envOr(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}

func serve(args []string) error {
	cfg, err := parseFlags(args, os.Getenv)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	hm, err := hostman.New(cfg.hostmanURL, os.Getenv("HOMEAPPS_HOSTMAN_PIN"))
	if err != nil {
		return err
	}
	apps, err := custom.Open(filepath.Join(cfg.dataDir, "custom-apps.json"))
	if err != nil {
		return fmt.Errorf("custom apps: %w", err)
	}
	fetchClient := &http.Client{Timeout: 10 * time.Second}
	brands := branding.New(func(ctx context.Context, u string) (manifest.Brand, error) {
		return manifest.Fetch(ctx, fetchClient, u)
	}, 6*time.Hour, 10*time.Minute)

	mux := http.NewServeMux()
	fetchImage := func(ctx context.Context, u string) ([]byte, string, error) {
		return manifest.FetchImage(ctx, fetchClient, u)
	}
	api.New(hm, apps, brands, fetchImage, version.String(), log).Routes(mux)
	assets := web.Assets()
	if cfg.webDist != "" {
		assets = os.DirFS(cfg.webDist)
	}
	mux.Handle("/", web.Handler(assets))

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.host, strconv.Itoa(cfg.port)),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() {
		log.Info("homeapps listening", "addr", srv.Addr, "hostman", hm.BaseURL(), "custom_apps", apps.Path(), "version", version.String())
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
