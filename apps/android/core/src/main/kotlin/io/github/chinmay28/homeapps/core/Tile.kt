package io.github.chinmay28.homeapps.core

/**
 * Colours for an app tile that has no icon, matching the web client exactly
 * (view.js: hue(), and the .icon rule's hsl(h 70% 72%) on hsl(h 70% 12%)), so
 * a monogram looks the same in the launcher, the switcher and the bubble.
 */
object Tile {
    /** view.js hue(): h = h*31 + codePoint, as unsigned 32-bit, mod 360. */
    fun hue(name: String): Int {
        var h = 0L
        var i = 0
        while (i < name.length) {
            val cp = name.codePointAt(i)
            h = (h * 31 + cp) and 0xFFFFFFFFL
            i += Character.charCount(cp)
        }
        return (h % 360).toInt()
    }

    fun background(name: String): Int = hslToArgb(hue(name).toDouble(), 0.70, 0.72)

    fun foreground(name: String): Int = hslToArgb(hue(name).toDouble(), 0.70, 0.12)

    /** CSS hsl() to opaque ARGB; h in degrees, s and l in 0..1. */
    fun hslToArgb(h: Double, s: Double, l: Double): Int {
        val c = (1 - kotlin.math.abs(2 * l - 1)) * s
        val hp = ((h % 360) + 360) % 360 / 60
        val x = c * (1 - kotlin.math.abs(hp % 2 - 1))
        val (r1, g1, b1) = when {
            hp < 1 -> Triple(c, x, 0.0)
            hp < 2 -> Triple(x, c, 0.0)
            hp < 3 -> Triple(0.0, c, x)
            hp < 4 -> Triple(0.0, x, c)
            hp < 5 -> Triple(x, 0.0, c)
            else -> Triple(c, 0.0, x)
        }
        val m = l - c / 2
        fun ch(v: Double) = Math.round((v + m) * 255).toInt().coerceIn(0, 255)
        return (0xFF shl 24) or (ch(r1) shl 16) or (ch(g1) shl 8) or ch(b1)
    }
}
