# Working in this repository

HomeApps is a launcher for the self-hosted apps on a home network: a Go server
(`server/`) that reads hosts and apps from HostMan and serves a hand-written
web client, plus an Android shell (`apps/android`) that opens each app full
screen in its own WebView. `README.md` has the overview and the API.

## Commands

```bash
make test            # go test -race · node --test web/*.test.mjs · gradle test in apps/android/core
make test-installer  # runs scripts/quickstart.sh for real in a sandbox (root)
make vet             # go vet + gofmt check
make run             # serve against $HOMEAPPS_HOSTMAN with the client read from disk
make build           # bin/homeapps
make android         # debug APK (needs the Android SDK)
```

## Rules that shape changes

- **Two sources of apps, one small piece of state.** HostMan's deployed apps
  come from its API and are never copied. The apps added by address
  (`internal/myapps`) are the only thing HomeApps stores: one JSON file,
  rewritten atomically, in systemd's StateDirectory (`/var/lib/homeapps`).
  HostMan is optional (`--hostman off`), so `/api/catalog` answers 200 with
  the added apps even when HostMan is down (`hostman_error` says why). Don't
  grow this into a database; anything else should still come from HostMan or
  from the apps themselves (their manifests).
- **Small, pure, tested components.** `catalog`, `myapps` (validation), `manifest` (parsing),
  `web/assets/view.js` and everything in `apps/android/core` are pure
  functions with table-driven tests. Keep logic there. The HTTP handlers,
  `app.js` and the Android `app/` module are glue.
- **The wire contract is `/api/catalog`.** Snake_case, explicit `[]` and
  `null`, `{"error": …}` bodies. `api_test.go` pins it; change it together
  with `view.js` and `app.js`.
- **The bridge is input.** `window.HomeAppsNative.open(id, name, url, color,
  icon)` is reachable from JavaScript. `LaunchedApp.from` validates every
  argument, and the shell honours calls only from the configured server's
  origin. Keep both.
- **Monogram tiles match everywhere.** `view.js` `monogram()`/`hue()` and
  `core` `LaunchedApp.monogramOf`/`Tile` are the same rules. `TileTest` pins
  the hue against values printed by `view.js`.
- **No front-end build step, no Go dependencies.** The client is ES modules
  embedded with `go:embed`. The server is stdlib only and builds with
  `CGO_ENABLED=0` for a Raspberry Pi.
- **The service worker never touches `/api`.**

## Version

`vYEAR.MONTH.PATCH`. `Year`/`Month` live in
`server/internal/version/version.go` and PATCH is the commit count
(`scripts/version.sh`). **Before the first commit on a new branch, run `make
bump-version`**, and never bump again on the same branch. Never read the
version from the build clock. Don't assert the literal version in tests.

## Commits

Sign off every commit (`git commit -s`), as in the sibling repositories.
