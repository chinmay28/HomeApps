package io.github.chinmay28.homeapps.core

/**
 * What answered at a server address: HomeApps, some other app, or nothing.
 *
 * Every sibling app answers GET /api/health with {"status","version"}, so a
 * version alone proves nothing: HostMan's address passed for HomeApps and the
 * shell showed HostMan as its home screen. HomeApps names itself with
 * "app":"homeapps", and that is the only thing accepted.
 */
object ServerCheck {
    enum class Verdict { HOMEAPPS, OTHER_APP, UNREACHABLE }

    private val homeApps = Regex("\"app\"\\s*:\\s*\"homeapps\"")

    /** @param status the HTTP status, or null when nothing answered */
    fun classify(status: Int?, body: String?): Verdict = when {
        status == null -> Verdict.UNREACHABLE
        status == 200 && body != null && homeApps.containsMatchIn(body) -> Verdict.HOMEAPPS
        else -> Verdict.OTHER_APP
    }
}
