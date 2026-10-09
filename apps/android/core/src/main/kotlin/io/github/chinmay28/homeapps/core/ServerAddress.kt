package io.github.chinmay28.homeapps.core

import java.net.URI

/**
 * The HomeApps server as typed by a person on first run: "pi5.local:8686",
 * "http://192.168.2.10:8686/", "https://home.example". Normalised to
 * scheme://host[:port]/ — the launcher is served at the root.
 */
object ServerAddress {
    fun normalize(input: String): String? {
        var s = input.trim()
        if (s.isEmpty() || s.any { it.isWhitespace() }) return null
        if (!s.contains("://")) s = "http://$s"
        val uri = runCatching { URI(s) }.getOrNull() ?: return null
        val scheme = uri.scheme?.lowercase()
        if (scheme != "http" && scheme != "https") return null
        val host = uri.host?.lowercase() ?: return null
        if (uri.port > 65535) return null
        val port = if (uri.port == -1) "" else ":${uri.port}"
        val h = if (host.contains(':') && !host.startsWith("[")) "[$host]" else host
        return "$scheme://$h$port/"
    }

    /** scheme://host[:port] of a URL, for same-origin checks; null if unparseable. */
    fun origin(url: String?): String? {
        val uri = runCatching { URI(url ?: return null) }.getOrNull() ?: return null
        val scheme = uri.scheme?.lowercase() ?: return null
        val host = uri.host?.lowercase() ?: return null
        val port = when {
            uri.port != -1 -> uri.port
            scheme == "http" -> 80
            scheme == "https" -> 443
            else -> return null
        }
        return "$scheme://$host:$port"
    }

    fun sameOrigin(a: String?, b: String?): Boolean {
        val oa = origin(a) ?: return false
        return oa == origin(b)
    }
}
