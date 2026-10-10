package io.github.chinmay28.homeapps

import android.annotation.SuppressLint
import android.app.DownloadManager
import android.content.ActivityNotFoundException
import android.content.Intent
import android.graphics.Color
import android.net.Uri
import android.os.Bundle
import android.os.Environment
import android.os.Handler
import android.os.Looper
import android.view.View
import android.view.ViewGroup
import android.webkit.CookieManager
import android.webkit.URLUtil
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.addCallback
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.view.ViewCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import io.github.chinmay28.homeapps.core.LaunchedApp
import io.github.chinmay28.homeapps.core.Navigation
import io.github.chinmay28.homeapps.core.ServerAddress
import io.github.chinmay28.homeapps.core.Sessions
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

/**
 * The whole shell. Three layers in one window, bottom to top:
 *
 *  1. the launcher — the HomeApps server's web page, inside the system bars;
 *  2. the running apps — one WebView each, full screen with the system bars
 *     hidden, kept alive while you are elsewhere (AppHost, Sessions);
 *  3. the bubble and the switcher it opens, the only HomeApps chrome drawn
 *     over an app.
 */
class MainActivity : ComponentActivity(), SwitcherView.Listener, AppHost.Callbacks {
    private lateinit var prefs: Prefs
    private val sessions = Sessions(capacity = 4)
    private val main = Handler(Looper.getMainLooper())
    private val io: ExecutorService = Executors.newFixedThreadPool(2)
    private val icons by lazy { IconLoader(io, main) }

    private lateinit var root: FrameLayout
    private lateinit var homeFrame: FrameLayout
    private lateinit var appFrame: FrameLayout
    private lateinit var appHost: AppHost
    private lateinit var bubble: BubbleView
    private lateinit var switcher: SwitcherView
    private var home: WebView? = null
    private var setup: SetupView? = null
    private var lastInsets: WindowInsetsCompat? = null

    private var pendingFiles: ValueCallback<Array<Uri>>? = null
    private val filePicker = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
        pendingFiles?.onReceiveValue(WebChromeClient.FileChooserParams.parseResult(result.resultCode, result.data))
        pendingFiles = null
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(Color.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(Color.TRANSPARENT),
        )
        super.onCreate(savedInstanceState)
        prefs = Prefs(this)

        root = FrameLayout(this).apply { setBackgroundColor(getColor(R.color.bg)) }
        homeFrame = FrameLayout(this)
        appFrame = FrameLayout(this).apply {
            setBackgroundColor(Color.BLACK) // shows only beside a camera cutout
            visibility = View.GONE
        }
        appHost = AppHost(this, appFrame, this)
        bubble = BubbleView(this) { onRight, fraction ->
            prefs.bubbleOnRight = onRight
            prefs.bubbleY = fraction
        }.apply {
            visibility = View.GONE
            setOnClickListener { showSwitcher() }
        }
        switcher = SwitcherView(this, this)

        root.addView(homeFrame, matchParent())
        root.addView(appFrame, matchParent())
        root.addView(bubble, FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT))
        root.addView(switcher, matchParent())
        setContentView(root)

        ViewCompat.setOnApplyWindowInsetsListener(root) { _, insets ->
            lastInsets = insets
            applyInsets(insets)
            insets
        }
        onBackPressedDispatcher.addCallback(this) { handleBack() }

        val server = prefs.serverUrl
        if (server == null) showSetup(prefill = null, error = null) else showHome(server)
    }

    private fun applyInsets(insets: WindowInsetsCompat) {
        // The launcher sits inside the bars; an app gets the whole screen and
        // only steps around the camera, whose inset is there even with the
        // bars hidden.
        val bars = insets.getInsets(WindowInsetsCompat.Type.systemBars() or WindowInsetsCompat.Type.displayCutout())
        homeFrame.setPadding(bars.left, bars.top, bars.right, bars.bottom)
        setup?.setPadding(bars.left, bars.top, bars.right, bars.bottom)
        val cutout = insets.getInsets(WindowInsetsCompat.Type.displayCutout())
        appFrame.setPadding(cutout.left, cutout.top, cutout.right, cutout.bottom)
        val gestures = insets.getInsetsIgnoringVisibility(WindowInsetsCompat.Type.navigationBars())
        bubble.setInsets(cutout.top, maxOf(cutout.bottom, gestures.bottom))
        switcher.setBottomInset(gestures.bottom)
    }

    // --- The launcher -------------------------------------------------------

    private fun showHome(server: String) {
        setup?.let { root.removeView(it) }
        setup = null
        val web = home ?: createHome().also {
            home = it
            homeFrame.addView(it, matchParent())
        }
        if (!ServerAddress.sameOrigin(web.url, server)) web.loadUrl(server)
    }

    @SuppressLint("SetJavaScriptEnabled", "JavascriptInterface")
    private fun createHome(): WebView = WebView(this).apply {
        setBackgroundColor(getColor(R.color.bg))
        settings.javaScriptEnabled = true
        settings.domStorageEnabled = true
        settings.allowFileAccess = false
        settings.allowContentAccess = false
        addJavascriptInterface(
            HomeBridge(main, isTrusted = { ServerAddress.sameOrigin(home?.url, prefs.serverUrl) }, onOpen = this@MainActivity::openApp) {
                showSetup(prefs.serverUrl, error = null)
            },
            HomeBridge.NAME,
        )
        webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                val url = request.url.toString()
                if (ServerAddress.sameOrigin(url, prefs.serverUrl)) return false
                // Anything else leaving the launcher (a link on the Hosts
                // page, a tile opened without the bridge) goes to the browser.
                if (Navigation.decide(url) != Navigation.Decision.BLOCK) openExternally(request.url)
                return true
            }

            override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
                if (!request.isForMainFrame) return
                val server = prefs.serverUrl ?: return
                showSetup(server, getString(R.string.home_unreachable, server))
            }
        }
    }

    private fun showSetup(prefill: String?, error: String?) {
        setup?.let { root.removeView(it) }
        val view = SetupView(this, prefill, error, ::connect)
        setup = view
        root.addView(view, matchParent())
        lastInsets?.let(::applyInsets)
    }

    private fun connect(input: String) {
        val view = setup ?: return
        val server = ServerAddress.normalize(input)
        if (server == null) {
            view.showError(getString(R.string.setup_invalid))
            return
        }
        view.setBusy(true)
        view.showError(null)
        io.execute {
            val ok = isHomeApps(server)
            main.post {
                if (setup !== view) return@post
                view.setBusy(false)
                if (!ok) {
                    view.showError(getString(R.string.setup_unreachable, server))
                    return@post
                }
                val changed = prefs.serverUrl != server
                prefs.serverUrl = server
                if (changed) home?.loadUrl(server)
                showHome(server)
            }
        }
    }

    /** True when [server] answers /api/health like a HomeApps server. */
    private fun isHomeApps(server: String): Boolean = runCatching {
        val conn = URL(server + "api/health").openConnection() as HttpURLConnection
        conn.connectTimeout = 5_000
        conn.readTimeout = 5_000
        try {
            conn.responseCode == HttpURLConnection.HTTP_OK &&
                conn.inputStream.bufferedReader().use { it.readText() }.contains("\"version\"")
        } finally {
            conn.disconnect()
        }
    }.getOrDefault(false)

    // --- Running apps -------------------------------------------------------

    private fun openApp(app: LaunchedApp) {
        switcher.hide()
        appHost.show(sessions.open(app))
        setFullScreen(true)
        bubble.bind(app, icons)
        if (bubble.visibility != View.VISIBLE) {
            bubble.visibility = View.VISIBLE
            bubble.post { bubble.place(prefs.bubbleOnRight, prefs.bubbleY) }
        }
    }

    private fun goHome() {
        switcher.hide()
        sessions.home()
        appHost.hideAll()
        bubble.visibility = View.GONE
        setFullScreen(false)
    }

    private fun showSwitcher() {
        sessions.current ?: return
        switcher.show(sessions.running(), sessions.current, icons)
    }

    private fun setFullScreen(on: Boolean) {
        val controller = WindowCompat.getInsetsController(window, window.decorView)
        if (on) {
            // Bars stay reachable: a swipe from the edge shows them briefly.
            controller.systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
            controller.hide(WindowInsetsCompat.Type.systemBars())
        } else {
            controller.show(WindowInsetsCompat.Type.systemBars())
        }
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        // Coming back from another app or a dialog restores the bars; hide
        // them again if an app is in front.
        if (hasFocus && sessions.current != null) setFullScreen(true)
    }

    private fun handleBack() {
        val current = sessions.current
        when {
            switcher.isShowing -> switcher.hide()
            setup != null && prefs.serverUrl != null && home != null -> {
                root.removeView(setup)
                setup = null
            }
            current != null -> {
                val web = appHost.webView(current.id)
                if (web != null && web.canGoBack()) web.goBack() else goHome()
            }
            home?.canGoBack() == true -> home?.goBack()
            // Like a home screen: leave without stopping the running apps.
            else -> moveTaskToBack(true)
        }
    }

    // --- Switcher -----------------------------------------------------------

    override fun onSwitcherHome() = goHome()

    override fun onSwitcherReload() {
        switcher.hide()
        sessions.current?.let { appHost.reload(it.id) }
    }

    override fun onSwitcherBrowser() {
        val current = sessions.current ?: return
        switcher.hide()
        val url = appHost.webView(current.id)?.url ?: current.url
        openExternally(Uri.parse(url))
    }

    override fun onSwitcherClose() {
        val current = sessions.current ?: return
        sessions.close(current.id)
        appHost.destroy(current.id)
        goHome()
    }

    override fun onSwitcherPick(app: LaunchedApp) = openApp(app)

    // --- What apps ask of the system ---------------------------------------

    override fun openExternally(uri: Uri) {
        val intent = if (uri.scheme == "intent") {
            // An intent: link may name any component; strip that so a page
            // can only ask for something any app could handle.
            runCatching { Intent.parseUri(uri.toString(), Intent.URI_INTENT_SCHEME) }.getOrNull()?.apply {
                component = null
                selector = null
                addCategory(Intent.CATEGORY_BROWSABLE)
            }
        } else {
            Intent(Intent.ACTION_VIEW, uri).addCategory(Intent.CATEGORY_BROWSABLE)
        }
        try {
            if (intent != null) startActivity(intent) else toast(getString(R.string.no_app_for_link))
        } catch (_: ActivityNotFoundException) {
            toast(getString(R.string.no_app_for_link))
        }
    }

    override fun pickFiles(params: WebChromeClient.FileChooserParams, callback: ValueCallback<Array<Uri>>): Boolean {
        pendingFiles?.onReceiveValue(null)
        pendingFiles = callback
        return try {
            filePicker.launch(params.createIntent())
            true
        } catch (_: ActivityNotFoundException) {
            pendingFiles = null
            false
        }
    }

    override fun download(url: String, userAgent: String, contentDisposition: String?, mimeType: String?) {
        val uri = Uri.parse(url)
        if (uri.scheme != "http" && uri.scheme != "https") {
            // A blob: or data: download exists only inside the page; the
            // download manager cannot fetch it.
            toast(getString(R.string.no_app_for_link))
            return
        }
        val name = URLUtil.guessFileName(url, contentDisposition, mimeType)
        try {
            val request = DownloadManager.Request(uri)
                .setMimeType(mimeType)
                .addRequestHeader("User-Agent", userAgent)
                .setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
                .setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS, name)
            CookieManager.getInstance().getCookie(url)?.let { request.addRequestHeader("Cookie", it) }
            getSystemService(DownloadManager::class.java).enqueue(request)
            toast(getString(R.string.download_started, name))
        } catch (_: RuntimeException) {
            // No public Downloads folder access (older Android): let the
            // browser do it.
            openExternally(uri)
        }
    }

    override fun onDestroy() {
        appHost.destroyAll()
        home?.destroy()
        io.shutdownNow()
        super.onDestroy()
    }

    private fun toast(text: String) = Toast.makeText(this, text, Toast.LENGTH_SHORT).show()

    private fun matchParent() = FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT)
}
