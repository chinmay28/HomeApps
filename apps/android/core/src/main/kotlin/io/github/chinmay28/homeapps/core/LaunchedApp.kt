package io.github.chinmay28.homeapps.core

import java.net.URI

/**
 * One app the launcher asked the shell to open. Built only through [from],
 * which validates everything the web page sent: the bridge is reachable from
 * JavaScript, so its arguments are input, not trusted data.
 */
data class LaunchedApp(
    val id: String,
    val name: String,
    val url: String,
    /** ARGB, or null when the app declared no usable theme colour. */
    val color: Int?,
    /** Absolute http(s) URL of the app's icon, or null. */
    val iconUrl: String?,
) {
    /** Two letters for when there is no icon; mirrors the web client. */
    val monogram: String get() = monogramOf(name)

    companion object {
        private const val MAX_FIELD = 2048

        fun from(id: String?, name: String?, url: String?, color: String?, icon: String?): LaunchedApp? {
            val cleanId = id?.trim().orEmpty()
            if (cleanId.isEmpty() || cleanId.length > 128) return null
            val cleanUrl = url?.trim().orEmpty()
            if (!isHttpUrl(cleanUrl)) return null
            val cleanName = name?.trim().orEmpty().take(128).ifEmpty { cleanId }
            val cleanIcon = icon?.trim().orEmpty().takeIf { isHttpUrl(it) }
            return LaunchedApp(cleanId, cleanName, cleanUrl, parseColor(color), cleanIcon)
        }

        fun isHttpUrl(s: String): Boolean {
            if (s.isEmpty() || s.length > MAX_FIELD) return false
            val uri = runCatching { URI(s) }.getOrNull() ?: return false
            return (uri.scheme == "http" || uri.scheme == "https") && !uri.host.isNullOrEmpty()
        }

        /** "#rgb" or "#rrggbb" to opaque ARGB; anything else is null. */
        fun parseColor(s: String?): Int? {
            val hex = s?.trim()?.removePrefix("#") ?: return null
            if (!hex.all { it.isDigit() || it.lowercaseChar() in 'a'..'f' }) return null
            val full = when (hex.length) {
                3 -> hex.map { "$it$it" }.joinToString("")
                6 -> hex
                else -> return null
            }
            return (0xFF000000 or full.toLong(16)).toInt()
        }

        fun monogramOf(name: String): String {
            var words = name.trim().split(Regex("[\\s_-]+")).filter { it.isNotEmpty() }
            if (words.size == 1) words = words[0].split(Regex("(?<=[a-z0-9])(?=[A-Z])"))
            if (words.size >= 2) return "${words[0][0]}${words[1][0]}".uppercase()
            val w = words.firstOrNull() ?: return "?"
            return w.take(1).uppercase() + w.drop(1).take(1).lowercase()
        }
    }
}
