package io.github.chinmay28.homeapps.core

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

class ServerListTest {
    private val home = "http://pi5.local:8686/"
    private val lab = "http://192.168.2.10:8686/"

    @Test
    fun useAddsOnceAndSwitches() {
        val one = ServerList.EMPTY.use("pi5.local:8686")!!
        assertEquals(ServerList(listOf(home), home), one)
        val two = one.use("192.168.2.10:8686")!!
        assertEquals(ServerList(listOf(home, lab), lab), two)
        assertEquals(ServerList(listOf(home, lab), home), two.use("HTTP://PI5.local:8686/#/hosts"), "switching back keeps the order")
        assertNull(two.use("ftp://nope"))
    }

    @Test
    fun forgetTheActiveOneLeavesNoneActive() {
        val two = ServerList(listOf(home, lab), lab)
        assertEquals(ServerList(listOf(home), null), two.forget(lab))
        assertEquals(ServerList(listOf(lab), lab), two.forget(home))
        assertEquals(two, two.forget("http://unknown:1/"))
        assertEquals(two, two.forget("not an address"))
    }

    @Test
    fun fullListDropsTheOldestThatIsNotInUse() {
        var list = ServerList.EMPTY
        for (i in 1..ServerList.MAX) list = list.use("h$i.local")!!
        list = list.use("h1.local")!! // the oldest is now the active one
        list = list.use("new.local")!!
        assertEquals(ServerList.MAX, list.servers.size)
        assertEquals("http://h1.local/", list.servers.first(), "the active server is never the one dropped")
        assertEquals(false, "http://h2.local/" in list.servers)
        assertEquals("http://new.local/", list.active)
    }

    @Test
    fun encodeDecodeRoundTrips() {
        for (list in listOf(ServerList.EMPTY, ServerList(listOf(home, lab), lab), ServerList(listOf(home, lab), null))) {
            assertEquals(list, ServerList.decode(list.encode()))
        }
    }

    @Test
    fun decodeDropsWhatIsNoLongerValid() {
        val got = ServerList.decode("not a url\n*$lab\n$lab\n\nhttp://PI5.local:8686\n*ftp://x")
        assertEquals(ServerList(listOf(lab, home), lab), got)
    }

    @Test
    fun theOneServerAnOlderBuildSavedBecomesTheList() {
        assertEquals(ServerList(listOf(home), home), ServerList.decode(null, legacy = home))
        assertEquals(ServerList.EMPTY, ServerList.decode(null, legacy = "not a url"))
        val saved = ServerList(listOf(lab), lab)
        assertEquals(saved, ServerList.decode(saved.encode(), legacy = home), "a saved list wins over the legacy key")
    }

    @Test
    fun activeMustBeListed() {
        assertFailsWith<IllegalArgumentException> { ServerList(listOf(home), lab) }
    }
}
