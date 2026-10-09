package io.github.chinmay28.homeapps.core

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class SessionsTest {
    private fun app(id: String, url: String = "http://h/$id") = LaunchedApp(id, id, url, null, null)

    @Test
    fun keepsAppsWarmAndReusesThem() {
        val s = Sessions(capacity = 3)
        assertFalse(s.open(app("a")).reused)
        s.open(app("b"))
        val again = s.open(app("a"))
        assertTrue(again.reused, "switching back to a running app must not reload it")
        assertEquals(listOf("a", "b"), s.running().map { it.id })
        assertEquals("a", s.current?.id)
    }

    @Test
    fun evictsTheLeastRecentlyUsed() {
        val s = Sessions(capacity = 2)
        s.open(app("a"))
        s.open(app("b"))
        s.open(app("a")) // a is now the most recent
        val opened = s.open(app("c"))
        assertEquals(listOf("b"), opened.evicted.map { it.id })
        assertEquals(listOf("c", "a"), s.running().map { it.id })
    }

    @Test
    fun homeKeepsEverythingRunning() {
        val s = Sessions()
        s.open(app("a"))
        s.home()
        assertNull(s.current)
        assertEquals(listOf("a"), s.running().map { it.id })
    }

    @Test
    fun closeStopsOneApp() {
        val s = Sessions()
        s.open(app("a"))
        s.open(app("b"))
        assertEquals("b", s.close("b")?.id)
        assertNull(s.current, "closing the app on screen leaves nothing in front")
        assertNull(s.close("zzz"))
        assertEquals(listOf("a"), s.running().map { it.id })
    }

    @Test
    fun reportsAMovedApp() {
        val s = Sessions()
        s.open(app("a", "http://old:1/"))
        val moved = s.open(app("a", "http://new:2/"))
        assertTrue(moved.reused && moved.moved, "HostMan redeployed it elsewhere; the WebView must load the new address")
        assertEquals("http://new:2/", s.current?.url)
    }

    @Test
    fun capacityMustBePositive() {
        assertFailsWith<IllegalArgumentException> { Sessions(0) }
    }
}
