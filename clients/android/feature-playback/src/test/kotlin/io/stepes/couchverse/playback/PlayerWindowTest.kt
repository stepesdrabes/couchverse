package io.stepes.couchverse.playback

import android.graphics.Rect
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import kotlin.test.assertEquals
import kotlin.test.assertNull

@RunWith(RobolectricTestRunner::class)
class PlayerWindowTest {
    @Test
    fun `picture-in-picture shrinks from where the picture is`() {
        assertEquals(Rect(240, 0, 2160, 1080), pictureIn(2400, 1080))
        assertEquals(Rect(0, 0, 1920, 1080), pictureIn(1920, 1080))
        assertEquals(Rect(0, 896, 1080, 1503), pictureIn(1080, 2400))
        assertNull(pictureIn(0, 0))
    }
}
