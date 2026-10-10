package io.github.chinmay28.homeapps.core

/**
 * The HomeApps servers this phone knows, and which one the launcher shows.
 * Several deployments (home, a parent's house, a lab) each keep their own
 * apps; switching is picking another entry, not typing an address again.
 *
 * Immutable: every change returns a new list. Entries are normalised with
 * [ServerAddress.normalize], unique, in the order they were first used, and
 * at most [MAX] of them. [active] is always one of [servers], or null.
 */
data class ServerList(val servers: List<String>, val active: String?) {
    init {
        require(active == null || active in servers) { "active server $active is not in the list" }
    }

    /** Makes [server] the active one, adding it to the end if it is new. Null if it is not an address. */
    fun use(server: String): ServerList? {
        val s = ServerAddress.normalize(server) ?: return null
        if (s in servers) return copy(active = s)
        // Full: the oldest one not in use makes room.
        val kept = if (servers.size < MAX) servers else servers - servers.first { it != active }
        return ServerList(kept + s, s)
    }

    /** Drops [server]. Forgetting the active one leaves none active: the person picks the next. */
    fun forget(server: String): ServerList {
        val s = ServerAddress.normalize(server) ?: return this
        return ServerList(servers - s, active.takeUnless { it == s })
    }

    /** One address per line, the active one marked with a leading "*". */
    fun encode(): String = servers.joinToString("\n") { if (it == active) "*$it" else it }

    companion object {
        const val MAX = 16
        val EMPTY = ServerList(emptyList(), null)

        /**
         * Reads [encode]'s output, dropping anything that is no longer a valid
         * address. [legacy] is the single address older builds saved; it joins
         * the list as the active server so an upgrade keeps working.
         */
        fun decode(encoded: String?, legacy: String? = null): ServerList {
            val servers = mutableListOf<String>()
            var active: String? = null
            for (line in encoded.orEmpty().lines()) {
                val marked = line.startsWith("*")
                val s = ServerAddress.normalize(line.removePrefix("*")) ?: continue
                if (s !in servers && servers.size < MAX) servers += s
                if (marked && s in servers) active = s
            }
            val list = ServerList(servers, active)
            return legacy?.let { old -> if (list.active == null) list.use(old) else null } ?: list
        }
    }
}
