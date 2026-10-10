package io.github.chinmay28.homeapps.core

import io.github.chinmay28.homeapps.core.ServerCheck.Verdict
import kotlin.test.Test
import kotlin.test.assertEquals

class ServerCheckTest {
    @Test
    fun acceptsOnlyHomeApps() {
        assertEquals(Verdict.HOMEAPPS, ServerCheck.classify(200, """{"app":"homeapps","status":"ok","version":"v2026.10.9"}"""))
        assertEquals(Verdict.HOMEAPPS, ServerCheck.classify(200, """{ "app" : "homeapps" }"""))
    }

    @Test
    fun rejectsASiblingAppThatAnswersHealthTheSameWay() {
        // HostMan's /api/health, verbatim: the address that was once accepted.
        assertEquals(Verdict.OTHER_APP, ServerCheck.classify(200, """{"status":"ok","version":"v2026.9.68"}"""))
        assertEquals(Verdict.OTHER_APP, ServerCheck.classify(200, """{"app":"countroster","status":"ok"}"""))
        assertEquals(Verdict.OTHER_APP, ServerCheck.classify(200, "<html>a router login page</html>"))
        assertEquals(Verdict.OTHER_APP, ServerCheck.classify(404, null))
        assertEquals(Verdict.OTHER_APP, ServerCheck.classify(502, """{"app":"homeapps"}"""))
    }

    @Test
    fun nothingAnswering() {
        assertEquals(Verdict.UNREACHABLE, ServerCheck.classify(null, null))
    }
}
