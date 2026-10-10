package io.github.chinmay28.homeapps.core

import kotlin.test.Test
import kotlin.test.assertEquals

class TileTest {
    @Test
    fun hueMatchesTheWebClient() {
        // Values printed by view.js hue() under node.
        val web = mapOf("SAND Vault" to 150, "Thought Mesh" to 260, "x" to 120, "Ünïcode 🙂" to 308)
        for ((name, want) in web) assertEquals(want, Tile.hue(name), name)
    }

    @Test
    fun hslMatchesCss() {
        // Reference values from Python's colorsys for the same hsl() inputs.
        assertEquals(0xFF86EAB8.toInt(), Tile.hslToArgb(150.0, 0.70, 0.72))
        assertEquals(0xFF170934.toInt(), Tile.hslToArgb(260.0, 0.70, 0.12))
        assertEquals(0xFFD92626.toInt(), Tile.hslToArgb(0.0, 0.70, 0.50))
    }
}
