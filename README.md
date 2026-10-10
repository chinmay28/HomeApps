# HomeApps

**Every app on your home network, one tap away.** HomeApps is a launcher for
the self-hosted apps on your LAN: one home screen with every app on every
host, each with its own icon and a live health dot. On Android, an app opens
**full screen** inside HomeApps, and switching between apps is instant.

Apps come from two places: whatever [HostMan](https://github.com/chinmay28/hostman)
has deployed shows up on its own, and anything else (a router page, a NAS, an
app on a machine HostMan doesn't manage) you add by its address with **+**.
HostMan is optional: without it, HomeApps is a launcher for the apps you add.

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
Settings are remembered in `/etc/homeapps.env`.

### Deploy it with HostMan

HomeApps can be one of HostMan's apps, so it shows up, and updates, like the
rest. In HostMan, **Apps → add**:

| Field | Value |
|---|---|
| Name | `HomeApps` |
| Install command | `curl -fsSL https://raw.githubusercontent.com/chinmay28/homeapps/main/scripts/quickstart.sh \| sudo bash` |
| Uninstall command | the same, ending `sudo bash -s -- --uninstall` |
| Health check | HTTP, `http://{HOST}:8686/api/health` |

Deploy it to the **Home** host (the machine HostMan runs on), so the default
`HOMEAPPS_HOSTMAN=http://127.0.0.1:8899` reaches HostMan. To put it anywhere
else, add `HOMEAPPS_HOSTMAN=http://<hostman-machine>:8899` after `sudo`. If
HostMan has a PIN, also add `HOMEAPPS_HOSTMAN_PIN=…`.

### Adding apps

There are two ways to add an app:

- **By address:** tap **+**, paste the address (`pi5.local:8123`,
  `http://nas.local/admin`) and optionally give it a name. Leave the name
  blank and the app's own manifest names it. These apps form the **My apps**
  section. Edit or remove them from the Hosts tab (removing takes a second
  tap). The list lives on the HomeApps server (`/var/lib/homeapps/apps.json`),
  so every phone and browser shares it, and HomeApps checks their health
  itself.
- **From HostMan:** deploy the app there. It appears within 15 seconds, as
  long as HostMan knows its address: an HTTP health check, or a parameter
  named `port`.

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
| GET | `/api/health` | `{"app":"homeapps","status":"ok","version":"v2026.10.12"}`. Answers even while HostMan is down. |
| GET | `/api/catalog` | Your apps and HostMan's hosts (see below). Always 200: if HostMan can't be read, `hosts` is `[]` and `hostman_error` says why. |
| GET | `/api/apps` | The apps added by address. |
| POST | `/api/apps` | Add one: `{"url": "pi5.local:8123", "name": "optional"}`. Returns 201 with the app, 400 for a bad address, 409 if the address is already listed. |
| PATCH | `/api/apps/{id}` | Change `name` and/or `url`. |
| DELETE | `/api/apps/{id}` | Remove one (204). |
| GET | `/api/icons/{key}` | An app's icon, as learned from its manifest. |

```json
{
  "my_apps": [{
    "id": "my-3f9a0c12b4de", "name": "Router", "url": "http://router.lan/", "port": 80,
    "health": "up", "detail": "", "version": "", "icon": "", "color": ""
  }],
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
  "hostman_error": "",
  "fetched_at": "2026-10-09T17:02:11Z"
}
```

`source` is `null` when HomeApps runs without HostMan.

`health` is `up`, `down` or `unknown`. Installations without an address to
open are left out. The self host comes first, then hosts and apps by name.

## Configuration

`homeapps serve` flags override environment variables, which override the
defaults:

| Flag | Env | Default |
|---|---|---|
| `--host` | `HOST` | `0.0.0.0` |
| `--port` | `PORT` | `8686` |
| `--hostman` | `HOMEAPPS_HOSTMAN` | `http://127.0.0.1:8899`; `off` for none |
| `--data` | `HOMEAPPS_DATA` | `$STATE_DIRECTORY/apps.json` under systemd, else `./homeapps-apps.json` |
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
| `server/internal/catalog` | Pure: overview to hosts and apps, health, ordering; added apps to launcher entries |
| `server/internal/myapps` | The apps added by address: validation, and a JSON file written atomically |
| `server/internal/probe` | Non-blocking health checks for the added apps |
| `server/internal/manifest` | Pure parsing of a page head and web manifest, icon choice, plus the fetch |
| `server/internal/branding` | Non-blocking icon/colour cache with refresh and retry |
| `server/internal/api` | HTTP: catalog, icons, health |
| `server/internal/web` | The embedded client: hand-written HTML/CSS/ES modules, no build step |
| `web/` | Tests for the client's pure view logic |
| `apps/android` | The Android shell; `core/` is its pure logic, tested without the SDK |

Every merge to `main` is released automatically by
`.github/workflows/release.yml`; it can also be run from the Actions tab, or
by pushing a matching `v*` tag. It tests the commit, then publishes the
server binaries for arm64 and amd64 and the Android APK, each with a
`.sha256`. The APK is
release-signed when the `HOMEAPPS_KEYSTORE_*` secrets are set, and a debug
build otherwise.

The version is `vYEAR.MONTH.PATCH`. YEAR and MONTH are constants in
`server/internal/version/version.go`, and PATCH is the commit count. Run
`make bump-version` before the first commit on a new branch.

## License

AGPL-3.0. A modified HomeApps offered over a network must make its source
available (§13).
