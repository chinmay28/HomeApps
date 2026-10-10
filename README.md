# HomeApps

**Every app on your home network, one tap away.** HomeApps is a launcher for
the self-hosted apps on your LAN: one home screen with every app on every
host, each with its own icon and a live health dot. On Android, an app opens
**full screen** inside HomeApps, and switching between apps is instant.

It reads everything from [HostMan](https://github.com/chinmay28/hostman), so
there is nothing to set up: whatever HostMan has deployed shows up here.

```
 phone ──▶ HomeApps (Go, :8686) ──▶ HostMan /api/overview   hosts, apps, ports, health
                    │
                    └──────────────▶ each app's web manifest   icon, theme colour
```

## Install

On any machine that can reach HostMan. Next to HostMan itself is simplest:

```sh
curl -fsSL https://raw.githubusercontent.com/chinmay28/homeapps/main/scripts/quickstart.sh | sudo bash
```

Then open `http://<that machine>:8686`. If HostMan is on another machine or
has a PIN:

```sh
curl -fsSL .../quickstart.sh | sudo HOMEAPPS_HOSTMAN=http://pi5.local:8899 HOMEAPPS_HOSTMAN_PIN=1234 bash
```

Re-run the same command to upgrade. The new build is health-checked after it
starts, and the previous one comes back if it fails. `--uninstall` removes it.
Settings are remembered in `/etc/homeapps.env`. You can also add the same
one-liner to HostMan as an app, with an HTTP health check on port 8686.

## Using it

- **Apps** is the icon grid, one section per host. Each tile has a dot: green
  for healthy, amber for not responding, grey for unknown (no check, or the
  host is offline). Search matches app and host names, so `pi5 va` finds SAND
  Vault on pi5, and Enter opens the first match.
- **Hosts** lists each machine with CPU and memory, its apps, their ports,
  and why a down app is down.
- In a **browser** (or the installed PWA), a tile opens the app in a new tab.
- In the **Android app**, a tile opens the app full screen in its own
  WebView. A small bubble floats over the app: drag it to either side, and
  tap it for Home, Reload, Open in browser, Close app, and the other running
  apps. Switching keeps each app exactly where you left it. See
  [apps/android/README.md](apps/android/README.md).

Icons come from each app's own web manifest, the same icon it gets when
installed on its own. An app that declares none gets a two-letter tile in a
colour that is stable for its name.

## API

| Method | Path | |
|---|---|---|
| GET | `/api/health` | `{"status":"ok","version":"v2026.10.12"}`. Answers even while HostMan is down. |
| GET | `/api/catalog` | Hosts and their openable apps (see below). 502 `{"error": …}` when HostMan can't be read. |
| GET | `/api/icons/{key}` | An app's icon, as learned from its manifest. |

```json
{
  "hosts": [{
    "id": 1, "name": "pi5", "address": "pi5.local", "online": true, "self": false,
    "cpu_pct": 2.1, "mem_pct": 31.0,
    "apps": [{
      "id": "hm-7", "name": "SAND Vault", "url": "http://pi5.local:8123/", "port": 8123,
      "health": "up", "detail": "", "version": "v2026.9.40",
      "icon": "/api/icons/3f9a…", "color": "#e7d3a8"
    }]
  }],
  "source": {"kind": "hostman", "url": "http://127.0.0.1:8899"},
  "fetched_at": "2026-10-09T17:02:11Z"
}
```

`health` is `up`, `down` or `unknown`. Installations without an address to
open are left out. The self host comes first, then hosts and apps by name.

## Configuration

`homeapps serve` flags override environment variables, which override the
defaults:

| Flag | Env | Default |
|---|---|---|
| `--host` | `HOST` | `0.0.0.0` |
| `--port` | `PORT` | `8686` |
| `--hostman` | `HOMEAPPS_HOSTMAN` | `http://127.0.0.1:8899` |
| `--web-dist` | `WEB_DIST` | embedded client |
| | `HOMEAPPS_HOSTMAN_PIN` | none. Env only, so it never shows in `ps`. |

There is no authentication. Like the apps it launches, HomeApps belongs on a
trusted network (LAN, Tailscale, VPN).

## Development

```sh
make test        # Go (-race), web client (node --test), Android shell logic (gradle)
make run         # serve against HOMEAPPS_HOSTMAN, web client read from disk
make build       # bin/homeapps: one static binary, client embedded
make android     # debug APK (needs the Android SDK)
make version     # the version this tree builds as
```

| Path | What it is |
|---|---|
| `server/internal/hostman` | HostMan client: overview, PIN login, re-login after HostMan restarts |
| `server/internal/catalog` | Pure: overview to hosts and apps, health, ordering |
| `server/internal/manifest` | Pure parsing of a page head and web manifest, icon choice, plus the fetch |
| `server/internal/branding` | Non-blocking icon/colour cache with refresh and retry |
| `server/internal/api` | HTTP: catalog, icons, health |
| `server/internal/web` | The embedded client: hand-written HTML/CSS/ES modules, no build step |
| `web/` | Tests for the client's pure view logic |
| `apps/android` | The Android shell; `core/` is its pure logic, tested without the SDK |

Releases come from `.github/workflows/release.yml`: run it from the Actions
tab on `main` (or push a matching `v*` tag). It publishes the server binaries
for arm64 and amd64 and the Android APK, each with a `.sha256`. The APK is
release-signed when the `HOMEAPPS_KEYSTORE_*` secrets are set, and a debug
build otherwise.

The version is `vYEAR.MONTH.PATCH`. YEAR and MONTH are constants in
`server/internal/version/version.go`, and PATCH is the commit count. Run
`make bump-version` before the first commit on a new branch.

## License

AGPL-3.0. A modified HomeApps offered over a network must make its source
available (§13).
