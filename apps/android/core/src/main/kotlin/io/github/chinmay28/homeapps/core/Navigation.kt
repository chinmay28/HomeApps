package io.github.chinmay28.homeapps.core

/**
 * What an app's WebView does with a link.
 *
 * Every http(s) page stays inside: an app's sign-in or cloud-connect flow
 * goes out to a provider and must come back to the same WebView to finish.
 * Schemes that belong to other apps (mailto:, tel:, intent:, market:) go to the
 * system; ones that could act on the WebView itself are refused.
 */
object Navigation {
    enum class Decision { STAY, SYSTEM, BLOCK }

    private val blocked = setOf("javascript", "file", "content", "data")

    fun decide(url: String): Decision {
        val scheme = url.substringBefore(':', "").lowercase()
        return when {
            scheme == "http" || scheme == "https" || scheme == "about" || scheme == "blob" -> Decision.STAY
            scheme.isEmpty() || scheme in blocked -> Decision.BLOCK
            else -> Decision.SYSTEM
        }
    }
}
