package io.stepes.couchverse.design

import androidx.compose.ui.graphics.Color
import io.stepes.couchverse.core.AccentPalette
import io.stepes.couchverse.design.components.Identicon
import io.stepes.couchverse.design.components.Showing
import io.stepes.couchverse.design.components.showing
import io.stepes.couchverse.design.text.formatClock
import io.stepes.couchverse.design.text.problemText
import io.stepes.couchverse.design.theme.AccentColors
import io.stepes.couchverse.design.theme.colorOf
import io.stepes.couchverse.core.LoadStatus
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class DesignTest {
    @Test
    fun `identicons hash like the web's minidenticons`() {
        // reference values from minidenticons' simpleHash in node
        val expected = mapOf(
            "nora" to 19258L,
            "admin" to 1073681699L,
            "otto" to 18518L,
            "Drama" to 1073690054L,
            "a" to 1073741699L,
            "Žluťoučký kůň" to 797457427L,
        )
        expected.forEach { (seed, hash) -> assertEquals(hash, Identicon.hash(seed), seed) }
        assertEquals(Identicon.colorOf("vera"), Identicon.colorOf("nora"), "both hash to hue 280")
    }

    @Test
    fun `colours parse as the core writes them`() {
        assertEquals(Color(0xFFE50914), colorOf("#e50914"))
        assertEquals(Color(0x29E50914), colorOf("#e5091429"))
        assertNull(colorOf("red"))
        assertNull(colorOf(null))
        val palette = AccentColors.of(AccentPalette("#3a6ea5", "#2d5681", "#3a6ea529", "#ffffff"))
        assertEquals(Color(0xFF3A6EA5), palette.accent)
        assertEquals(AccentColors.Default, AccentColors.of(null))
    }

    @Test
    fun `stale beats blank`() {
        assertEquals(Showing.Skeleton, showing(null))
        assertEquals(Showing.Skeleton, showing(LoadStatus.Loading))
        assertEquals(Showing.Content, showing(LoadStatus.Stale))
        assertEquals(Showing.Content, showing(LoadStatus.Loaded))
        assertEquals(Showing.NotFound, showing(LoadStatus.NotFound))
        assertEquals(Showing.Failed, showing(LoadStatus.Failed))
    }

    @Test
    fun `problem codes map to messages, unknown ones to a generic one`() {
        assertEquals(R.string.problem_offline, problemText("offline"))
        assertEquals(R.string.problem_rate_limited, problemText("slow_down"))
        assertEquals(R.string.problem_generic, problemText("http_502"))
        assertEquals(R.string.problem_generic, problemText(null))
    }

    @Test
    fun `positions read as clocks`() {
        assertEquals("0:09", formatClock(9))
        assertEquals("12:34", formatClock(754))
        assertEquals("1:02:03", formatClock(3723))
    }
}
