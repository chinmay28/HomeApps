# HomeApps for Android

A thin native shell. The home screen is the HomeApps server's own web page,
and each app you open runs **full screen** in a WebView of its own.

## How it behaves

- **First run** asks for the HomeApps server address (`pi5.local:8686`). It
  is saved only if `/api/health` answers `"app":"homeapps"`. The sibling apps
  answer health the same way, so HostMan's address (port 8899) is turned away
  with a hint, rather than becoming the home screen. A saved address is
  rechecked on every launch.
- **Several servers.** The shell remembers every HomeApps server you connect
  to (up to 16). Hosts tab, then *Switch HomeApps server*, lists them: tap
  one to switch, × to forget it, or type a new address to add one. Each is
  checked again before it is shown. Apps already running keep running
  across a switch, and Back never leads to the previous server's page.
  Builds before this one saved a single address; it becomes the first entry
  of the list.
- **Insets are handled once.** The shell pads the launcher clear of the
  status and navigation bars (and an app clear of the camera), and consumes
  the insets. Pages therefore see `env(safe-area-inset-*)` as zero and don't
  add a second band of padding.
- **The keyboard never covers a field.** The window is edge to edge, so
  Android doesn't shrink it for the keyboard; the shell pads the launcher and
  the app above it instead (`core` `SafeArea`). The WebView gets shorter, the
  page scrolls the field you're typing in back into view, and the bubble
  moves up out of the way.
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
| `core/` | Pure Kotlin/JVM, no Android SDK: which apps stay warm (`Sessions`), bridge input validation (`LaunchedApp`), server address parsing and same-origin checks, the saved servers (`ServerList`), link policy (`Navigation`), bubble snapping, and tile colours matching the web client. `cd core && ../gradlew test` |
| `app/` | The Android layer: `MainActivity` wires the launcher WebView, `AppHost` (one WebView per app), `BubbleView`, `SwitcherView` and `SetupView`. |

The launcher talks to the shell through `window.HomeAppsNative`:
`open(id, name, url, color, icon)` and `changeServer()` (which opens the
server list). These calls are honoured only while the launcher shows the
active server's own page.

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
