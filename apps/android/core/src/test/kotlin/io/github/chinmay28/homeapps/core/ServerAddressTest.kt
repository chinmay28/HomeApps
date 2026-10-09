package io.github.chinmay28.homeapps.core

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ServerAddressTest {
    @Test
    fun normalizesWhatPeopleType() {
        val cases = mapOf(
            "pi5.local:8686" to "http://pi5.local:8686/",
            "  http://192.168.2.10:8686/some/path?x=1 " to "http://192.168.2.10:8686/",
            "HTTPS://Home.Example" to "https://home.example/",
            "http://[fd00::1]:8686" to "http://[fd00::1]:8686/",
        )
        for ((input, want) in cases) assertEquals(want, ServerAddress.normalize(input), input)
    }

    @Test
    fun rejectsWhatIsNotAServer() {
        for (bad in listOf("", "   ", "ftp://pi5", "http://", "pi 5.local", "javascript:alert(1)")) {
            assertNull(ServerAddress.normalize(bad), "accepted '$bad'")
        }
    }

    @Test
    fun sameOriginComparesSchemeHostAndEffectivePort() {
        assertTrue(ServerAddress.sameOrigin("http://pi5.local:8686/#/hosts", "http://PI5.local:8686/"))
        assertTrue(ServerAddress.sameOrigin("http://pi5.local/", "http://pi5.local:80/"))
        assertFalse(ServerAddress.sameOrigin("http://pi5.local:8686/", "http://pi5.local:8123/"))
        assertFalse(ServerAddress.sameOrigin("https://pi5.local:8686/", "http://pi5.local:8686/"))
        assertFalse(ServerAddress.sameOrigin(null, "http://pi5.local/"))
        assertFalse(ServerAddress.sameOrigin("about:blank", "about:blank"))
    }
}
