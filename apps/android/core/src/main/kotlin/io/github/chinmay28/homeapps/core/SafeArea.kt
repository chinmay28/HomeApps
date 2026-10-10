package io.github.chinmay28.homeapps.core

/**
 * How far each layer keeps its content in from the window's edges.
 *
 * The window is drawn edge to edge, so Android no longer shrinks it for the
 * keyboard (adjustResize only works on a window that fits the system bars):
 * the shell pads for the keyboard itself. A layer that leaves room for the
 * keyboard gets shorter, its WebView with it, and the page scrolls the field
 * being typed in back into view, as it does in a browser.
 *
 * All values are pixels from the matching edge of the window. The keyboard's
 * inset is measured from the bottom of the screen, so it already covers the
 * navigation bar beneath it; each side is the largest inset that applies.
 */
object SafeArea {
    data class Edges(val left: Int = 0, val top: Int = 0, val right: Int = 0, val bottom: Int = 0) {
        /** The larger of the two on each side. */
        fun max(other: Edges) = Edges(
            maxOf(left, other.left),
            maxOf(top, other.top),
            maxOf(right, other.right),
            maxOf(bottom, other.bottom),
        )

        companion object {
            val NONE = Edges()
        }
    }

    /** The launcher and the setup screen sit inside the bars and above the keyboard. */
    fun launcher(bars: Edges, cutout: Edges, keyboard: Edges): Edges = bars.max(cutout).max(keyboard)

    /** A running app has the whole screen: it steps around only the camera and the keyboard. */
    fun app(cutout: Edges, keyboard: Edges): Edges = cutout.max(keyboard)

    /**
     * What the bubble keeps clear of at the bottom: the camera, the gesture
     * strip (there even while the bars are hidden) and the keyboard.
     */
    fun bubbleBottom(cutout: Edges, gestures: Edges, keyboard: Edges): Int =
        maxOf(cutout.bottom, gestures.bottom, keyboard.bottom)
}
