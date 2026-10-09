package io.github.chinmay28.homeapps

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.os.Handler
import android.util.LruCache
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.Executor

/**
 * Loads app icons from the HomeApps server for the bubble and the switcher.
 * Raster icons only: an SVG decodes to null and the tile falls back to its
 * monogram, exactly as it would with no icon at all.
 */
class IconLoader(private val io: Executor, private val main: Handler) {
    private val cache = LruCache<String, Bitmap>(32)

    fun load(url: String?, onLoaded: (Bitmap?) -> Unit) {
        if (url == null) {
            onLoaded(null)
            return
        }
        cache.get(url)?.let {
            onLoaded(it)
            return
        }
        io.execute {
            val bitmap = runCatching { fetch(url) }.getOrNull()
            if (bitmap != null) cache.put(url, bitmap)
            main.post { onLoaded(bitmap) }
        }
    }

    private fun fetch(url: String): Bitmap? {
        val conn = URL(url).openConnection() as HttpURLConnection
        conn.connectTimeout = 5_000
        conn.readTimeout = 5_000
        return try {
            if (conn.responseCode != HttpURLConnection.HTTP_OK) null else conn.inputStream.use { BitmapFactory.decodeStream(it) }
        } finally {
            conn.disconnect()
        }
    }
}
