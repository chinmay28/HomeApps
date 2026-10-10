package io.github.chinmay28.homeapps.core

import io.github.chinmay28.homeapps.core.BubblePosition.Point
import kotlin.test.Test
import kotlin.test.assertEquals

class BubblePositionTest {
    private val w = 1080
    private val h = 2400
    private val size = 144
    private val margin = 24

    @Test
    fun snapsToTheNearerSide() {
        assertEquals(Point(24, 1000), BubblePosition.snap(300, 1000, w, h, size, margin))
        assertEquals(Point(w - size - margin, 1000), BubblePosition.snap(700, 1000, w, h, size, margin))
    }

    @Test
    fun staysOnScreenVertically() {
        assertEquals(margin, BubblePosition.snap(0, -500, w, h, size, margin).y)
        assertEquals(h - size - margin, BubblePosition.snap(0, 99999, w, h, size, margin).y)
    }

    @Test
    fun survivesATinyArea() {
        // Before layout the area can be smaller than the bubble; never throw.
        assertEquals(Point(margin, margin), BubblePosition.snap(0, 0, 10, 10, size, margin))
    }

    @Test
    fun fractionRoundTrips() {
        val f = BubblePosition.toFraction(1128, h, size)
        assertEquals(1128, BubblePosition.fromFraction(f, h, size))
        assertEquals(0f, BubblePosition.toFraction(-5, h, size))
        assertEquals(1f, BubblePosition.toFraction(99999, h, size))
    }

    @Test
    fun keepsClearOfTheCutoutAndGestureBar() {
        assertEquals(100 + margin, BubblePosition.snap(0, 0, w, h, size, margin, insetTop = 100, insetBottom = 60).y)
        assertEquals(h - 60 - size - margin, BubblePosition.snap(0, h, w, h, size, margin, insetTop = 100, insetBottom = 60).y)
    }
}
