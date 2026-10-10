package io.github.chinmay28.homeapps.core

/**
 * Where the floating bubble rests. It can be dragged anywhere, and on release
 * it snaps to the nearer side edge, kept inside the safe area, so it never
 * sits over the middle of an app's content or under a system gesture strip.
 */
object BubblePosition {
    data class Point(val x: Int, val y: Int)

    /**
     * @param x,y where the bubble's top-left was released
     * @param width,height the area it lives in
     * @param size the bubble's diameter
     * @param margin gap to keep from the edges
     * @param insetTop,insetBottom space taken by a camera cutout or gesture bar
     */
    fun snap(x: Int, y: Int, width: Int, height: Int, size: Int, margin: Int, insetTop: Int = 0, insetBottom: Int = 0): Point {
        val maxX = (width - size - margin).coerceAtLeast(margin)
        val minY = insetTop + margin
        val maxY = (height - insetBottom - size - margin).coerceAtLeast(minY)
        val centre = x + size / 2
        val sx = if (centre < width / 2) margin else maxX
        return Point(sx, y.coerceIn(minY, maxY))
    }

    /** Height as a fraction, so a remembered position survives rotation. */
    fun toFraction(y: Int, height: Int, size: Int): Float {
        val span = (height - size).coerceAtLeast(1)
        return (y.toFloat() / span).coerceIn(0f, 1f)
    }

    fun fromFraction(f: Float, height: Int, size: Int): Int = (f.coerceIn(0f, 1f) * (height - size).coerceAtLeast(0)).toInt()
}
