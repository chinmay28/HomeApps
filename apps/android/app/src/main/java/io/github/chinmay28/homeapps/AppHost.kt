package io.github.chinmay28.homeapps

import android.annotation.SuppressLint
import android.content.Context
import android.net.Uri
import android.view.View
import android.view.ViewGroup
import android.webkit.CookieManager
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import io.github.chinmay28.homeapps.core.Navigation
import io.github.chinmay28.homeapps.core.Sessions

/**
 * Owns one WebView per running app, inside [container]. Sessions decides which
 * apps run; this keeps the WebViews in step with it. Hidden apps keep their
 * WebView — that is what makes switching back instant and seamless.
 */
class AppHost(private val context: Context, private val container: FrameLayout, private val callbacks: Callbacks) {
    interface Callbacks {
        fun openExternally(uri: Uri)
        fun pickFiles(params: WebChromeClient.FileChooserParams, callback: ValueCallback<Array<Uri>>): Boolean
        fun download(url: String, userAgent: String, contentDisposition: String?, mimeType: String?)
    }

    private val views = HashMap<String, WebView>()

    fun webView(id: String): WebView? = views[id]

    /** Brings an app to the front, creating, reloading or tearing down as told. */
    fun show(opened: Sessions.Opened) {
        for (gone in opened.evicted) destroy(gone.id)
        val id = opened.app.id
        val web = views[id] ?: create().also {
            views[id] = it
            container.addView(it, FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT))
            it.loadUrl(opened.app.url)
        }
        if (opened.moved) web.loadUrl(opened.app.url)
        for ((other, view) in views) {
            if (other == id) continue
            view.visibility = View.GONE
            view.onPause() // stops animations and media; JavaScript keeps running
        }
        web.visibility = View.VISIBLE
        web.onResume()
        container.visibility = View.VISIBLE
        web.requestFocus()
    }

    /** Back to the launcher; every app keeps running behind it. */
    fun hideAll() {
        container.visibility = View.GONE
        for (view in views.values) view.onPause()
    }

    fun reload(id: String) {
        views[id]?.reload()
    }

    fun destroy(id: String) {
        val web = views.remove(id) ?: return
        container.removeView(web)
        web.stopLoading()
        web.destroy()
    }

    fun destroyAll() {
        for (id in views.keys.toList()) destroy(id)
    }

    @SuppressLint("SetJavaScriptEnabled") // these are the user's own web apps
    private fun create(): WebView = WebView(context).apply {
        setBackgroundColor(0xFF0C0C11.toInt())
        with(settings) {
            javaScriptEnabled = true
            domStorageEnabled = true
            mediaPlaybackRequiresUserGesture = true
            // target=_blank and window.open load in place: one app, one view.
            setSupportMultipleWindows(false)
            javaScriptCanOpenWindowsAutomatically = false
            allowFileAccess = false
            allowContentAccess = false
            useWideViewPort = true
            loadWithOverviewMode = true
        }
        CookieManager.getInstance().setAcceptCookie(true)
        webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean =
                when (Navigation.decide(request.url.toString())) {
                    Navigation.Decision.STAY -> false
                    Navigation.Decision.SYSTEM -> {
                        callbacks.openExternally(request.url)
                        true
                    }
                    Navigation.Decision.BLOCK -> true
                }
        }
        webChromeClient = object : WebChromeClient() {
            override fun onShowFileChooser(
                view: WebView,
                filePathCallback: ValueCallback<Array<Uri>>,
                fileChooserParams: FileChooserParams,
            ): Boolean = callbacks.pickFiles(fileChooserParams, filePathCallback)
        }
        setDownloadListener { url, userAgent, contentDisposition, mimeType, _ ->
            callbacks.download(url, userAgent, contentDisposition, mimeType)
        }
    }
}
