package io.github.chinmay28.homeapps.core

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull

class LaunchedAppTest {
    @Test
    fun acceptsWhatTheLauncherSends() {
        val app = assertNotNull(
            LaunchedApp.from("hm-7", "SAND Vault", "http://pi5.local:8123/", "#e7d3a8", "http://pi5.local:8686/api/icons/ab"),
        )
        assertEquals(0xFFE7D3A8.toInt(), app.color)
        assertEquals("http://pi5.local:8686/api/icons/ab", app.iconUrl)
    }

    @Test
    fun rejectsWhatCannotBeOpened() {
        val bad = listOf("", "javascript:alert(1)", "file:///sdcard/x", "pi5.local:8123", "http://", "intent://x#Intent;end")
        for (url in bad) assertNull(LaunchedApp.from("hm-1", "X", url, "", ""), "accepted url $url")
        assertNull(LaunchedApp.from("", "X", "http://a/", "", ""), "an app needs an id the shell can key its WebView by")
        assertNull(LaunchedApp.from(null, "X", "http://a/", "", ""))
    }

    @Test
    fun dropsBadOptionalFieldsInsteadOfFailing() {
        val app = assertNotNull(LaunchedApp.from("hm-1", "  ", "https://a/", "red", "data:image/png;base64,xx"))
        assertNull(app.color, "only hex colours are accepted")
        assertNull(app.iconUrl, "only http(s) icons are fetched")
        assertEquals("hm-1", app.name, "a blank name falls back to the id")
    }

    @Test
    fun parsesShortAndLongHex() {
        assertEquals(0xFFAABBCC.toInt(), LaunchedApp.parseColor("#abc"))
        assertEquals(0xFF0C0C11.toInt(), LaunchedApp.parseColor("0c0c11"))
        assertNull(LaunchedApp.parseColor("#12345"))
        assertNull(LaunchedApp.parseColor("#ggg"))
    }

    @Test
    fun monogramMatchesTheWebClient() {
        val cases = mapOf("Thought Mesh" to "TM", "CountRoster" to "CR", "HostMan" to "HM", "BIP39 Explorer" to "BE", "tickers" to "Ti", "" to "?")
        for ((name, want) in cases) assertEquals(want, LaunchedApp.monogramOf(name), name)
    }
}
