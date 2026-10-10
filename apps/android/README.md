# HomeApps for Android

A thin native shell. The home screen is the HomeApps server's own web page,
and each app you open runs **full screen** in a WebView of its own.

## How it behaves

- **First run** asks for the HomeApps server address (`pi5.local:8686`). It
  is saved only if `/api/health` answers `"app":"homeapps"`. The sibling apps
  answer health the same way, so HostMan's address (port 8899) is turned away
  with a hint, rather than becoming the home screen. A saved address is
  rechecked on every launch. To change it later: Hosts tab, then
  *Change HomeApps server*.
- **Insets are handled once.** The shell pads the launcher clear of the
  status and navigation bars (and an app clear of the camera), and consumes
  the insets. Pages therefore see `env(safe-area-inset-*)` as zero and don't
  add a second band of padding.
- **Opening an app** hides the system bars. The app gets the whole screen and
  steps around only a camera cutout. Swipe from an edge to see the bars
  briefly.
- **The bubble** is the only HomeApps chrome over an app. Drag it to either
  side; it remembers where you left it and fades when idle. Tap it for the
  switcher: Home, Reload, Browser (open the current page in the real
  browser), Close app, and the running apps.
- **Switching is instant.** Up to four apps stay alive in the background with
  their scroll position, sign-in and unsaved state. Opening a fifth releases
  the least recently used.
- **Back** goes back inside the app, then to the launcher. At the launcher it
  leaves HomeApps without stopping the running apps.
- Links that belong to other apps (`mailto:`, `tel:`, `intent:`) go to the
  system. Every http(s) page stays in the app's WebView, so sign-in and
  cloud-connect redirects come back to it. File pickers and http(s)
  downloads work.

## Layout

| Path | |
|---|---|
| `core/` | Pure Kotlin/JVM, no Android SDK: which apps stay warm (`Sessions`), bridge input validation (`LaunchedApp`), server address parsing and same-origin checks, link policy (`Navigation`), bubble snapping, and tile colours matching the web client. `cd core && ../gradlew test` |
| `app/` | The Android layer: `MainActivity` wires the launcher WebView, `AppHost` (one WebView per app), `BubbleView`, `SwitcherView` and `SetupView`. |

The launcher talks to the shell through `window.HomeAppsNative`:
`open(id, name, url, color, icon)` and `changeServer()`. These calls are
honoured only while the launcher shows the configured server's own page.

## Building

JDK 17 and the Android SDK (`ANDROID_HOME`, or `sdk.dir` in
`local.properties`):

```sh
scripts/build-android.sh                 # debug APK, versioned from scripts/version.sh
scripts/build-android.sh assembleRelease # signed when HOMEAPPS_KEYSTORE* are set
```

CI (`.github/workflows/android.yml`) builds and lints the debug APK and
uploads it as an artifact.

## Known limits

- Blob downloads (generated inside a page) can't be handed to the download
  manager. Use *Browser* for those.
- Google sign-in refuses embedded WebViews. Do a Google connect (such as
  CountRoster's Drive backup) from *Browser*. The token lives on the app's
  server, so the app keeps working in HomeApps afterwards.
- SVG icons show as letter tiles in the bubble and switcher. The launcher
  page shows them.
