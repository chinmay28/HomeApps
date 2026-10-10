package io.github.chinmay28.homeapps.core

import io.github.chinmay28.homeapps.core.Navigation.Decision
import kotlin.test.Test
import kotlin.test.assertEquals

class NavigationTest {
    @Test
    fun decides() {
        val cases = mapOf(
            "http://pi5.local:8123/notes" to Decision.STAY,
            "https://www.dropbox.com/oauth2/authorize" to Decision.STAY,
            "about:blank" to Decision.STAY,
            "mailto:me@example.com" to Decision.SYSTEM,
            "tel:+15551234" to Decision.SYSTEM,
            "intent://scan#Intent;scheme=zxing;end" to Decision.SYSTEM,
            "javascript:alert(1)" to Decision.BLOCK,
            "JavaScript:alert(1)" to Decision.BLOCK,
            "file:///data/data/x" to Decision.BLOCK,
            "content://contacts" to Decision.BLOCK,
            "no-scheme-at-all" to Decision.BLOCK,
        )
        for ((url, want) in cases) assertEquals(want, Navigation.decide(url), url)
    }
}
