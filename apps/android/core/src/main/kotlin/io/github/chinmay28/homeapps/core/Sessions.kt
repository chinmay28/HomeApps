package io.github.chinmay28.homeapps.core

/**
 * Which apps are running, which one is in front, and which to let go.
 *
 * Every opened app keeps its WebView alive so switching back is instant — no
 * reload, scroll position and sign-in intact. Memory is finite, so at most
 * [capacity] stay warm and the least recently used is evicted. This class only
 * decides; the activity owns the WebViews and acts on what it returns.
 */
class Sessions(val capacity: Int = 4) {
    init {
        require(capacity >= 1) { "capacity must be at least 1" }
    }

    /** Most recently used first. */
    private val warm = ArrayDeque<LaunchedApp>()

    /** The app on screen, or null when the launcher is. */
    var current: LaunchedApp? = null
        private set

    data class Opened(
        val app: LaunchedApp,
        /** True when the app was already warm: show it, don't load it. */
        val reused: Boolean,
        /** Apps to tear down to stay within capacity. */
        val evicted: List<LaunchedApp>,
        /** The app was warm but its address changed; reload it at the new one. */
        val moved: Boolean,
    )

    fun open(app: LaunchedApp): Opened {
        val existing = warm.firstOrNull { it.id == app.id }
        if (existing != null) warm.remove(existing)
        warm.addFirst(app)
        val evicted = mutableListOf<LaunchedApp>()
        while (warm.size > capacity) evicted += warm.removeLast()
        current = app
        return Opened(app, reused = existing != null, evicted = evicted, moved = existing != null && existing.url != app.url)
    }

    /** Back to the launcher; every app stays warm. */
    fun home() {
        current = null
    }

    /** Stops an app. Returns it if it was running. */
    fun close(id: String): LaunchedApp? {
        val app = warm.firstOrNull { it.id == id } ?: return null
        warm.remove(app)
        if (current?.id == id) current = null
        return app
    }

    /** Running apps, most recently used first. */
    fun running(): List<LaunchedApp> = warm.toList()
}
