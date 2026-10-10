package io.github.chinmay28.homeapps

import android.os.Handler
import android.webkit.JavascriptInterface
import io.github.chinmay28.homeapps.core.LaunchedApp

/**
 * window.HomeAppsNative in the launcher page (see the web client's app.js).
 *
 * JavaScript calls arrive on a background thread and from whatever page the
 * home WebView happens to show, so every call hops to the main thread and is
 * honoured only if the page is the HomeApps server's own ([isTrusted]).
 */
class HomeBridge(
    private val main: Handler,
    private val isTrusted: () -> Boolean,
    private val onOpen: (LaunchedApp) -> Unit,
    private val onChangeServer: () -> Unit,
) {
    @JavascriptInterface
    fun open(id: String?, name: String?, url: String?, color: String?, icon: String?) {
        val app = LaunchedApp.from(id, name, url, color, icon) ?: return
        main.post { if (isTrusted()) onOpen(app) }
    }

    @JavascriptInterface
    fun changeServer() {
        main.post { if (isTrusted()) onChangeServer() }
    }

    companion object {
        const val NAME = "HomeAppsNative"
    }
}
