package io.github.chinmay28.homeapps.core

import io.github.chinmay28.homeapps.core.SafeArea.Edges
import kotlin.test.Test
import kotlin.test.assertEquals

class SafeAreaTest {
    private val bars = Edges(top = 80, bottom = 48)
    private val cutout = Edges(top = 96)
    private val gestures = Edges(bottom = 48)
    private val keyboard = Edges(bottom = 900)
    private val none = Edges.NONE

    @Test
    fun launcher() {
        data class Case(val name: String, val bars: Edges, val cutout: Edges, val keyboard: Edges, val want: Edges)
        val cases = listOf(
            Case("nothing", none, none, none, none),
            Case("bars only", bars, none, none, bars),
            Case("cutout deeper than the status bar", bars, cutout, none, Edges(top = 96, bottom = 48)),
            Case("keyboard covers the nav bar", bars, none, keyboard, Edges(top = 80, bottom = 900)),
            Case("landscape cutout on the left", bars, Edges(left = 120), none, Edges(left = 120, top = 80, bottom = 48)),
        )
        for (c in cases) assertEquals(c.want, SafeArea.launcher(c.bars, c.cutout, c.keyboard), c.name)
    }

    @Test
    fun app() {
        data class Case(val name: String, val cutout: Edges, val keyboard: Edges, val want: Edges)
        val cases = listOf(
            Case("full screen", none, none, none),
            Case("camera only", cutout, none, cutout),
            Case("keyboard up", none, keyboard, keyboard),
            Case("camera and keyboard", cutout, keyboard, Edges(top = 96, bottom = 900)),
        )
        for (c in cases) assertEquals(c.want, SafeArea.app(c.cutout, c.keyboard), c.name)
    }

    @Test
    fun bubbleBottom() {
        assertEquals(0, SafeArea.bubbleBottom(none, none, none))
        assertEquals(48, SafeArea.bubbleBottom(none, gestures, none))
        assertEquals(900, SafeArea.bubbleBottom(none, gestures, keyboard))
        assertEquals(60, SafeArea.bubbleBottom(Edges(bottom = 60), gestures, none))
    }
}
