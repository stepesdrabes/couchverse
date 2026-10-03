package io.stepes.couchverse.navigation

import android.app.Application
import android.content.Intent
import android.net.Uri
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import kotlin.test.assertEquals
import kotlin.test.assertNull

// a plain application: the real one would start the native core, which the JVM tests do not load
@Config(application = Application::class)
@RunWith(RobolectricTestRunner::class)
class LinksTest {
    @Test
    fun `links say what they ask for`() {
        val connect = "couchverse://connect?server=https%3A%2F%2Ftv.home&code=abc"
        assertEquals(AppLink.Connect(connect), AppLink.parse(connect))
        assertEquals(AppLink.Pair("couchverse://pair?code=WDJB-MJHT"), AppLink.parse("couchverse://pair?code=WDJB-MJHT"))
        assertEquals(AppLink.OpenTitle("glass-harbor-2025"), AppLink.parse("couchverse://title/glass-harbor-2025"))
        assertNull(AppLink.parse("couchverse://title"))
        assertNull(AppLink.parse("https://tv.home/title/glass-harbor-2025"))
        assertNull(AppLink.parse("couchverse://admin"))
        assertEquals(AppLink.Couch("123456"), AppLink.parse("couchverse://couch/123456"))
        assertNull(AppLink.parse("couchverse://couch/12"))
        assertEquals(AppLink.Play("episode", "e2"), AppLink.parse("couchverse://play/episode/e2"))
        assertNull(AppLink.parse("couchverse://play/song/s1"))
    }

    @Test
    fun `a link waits until it is taken, once`() {
        val links = PendingLinks()
        links.offer(Intent(Intent.ACTION_VIEW, Uri.parse("couchverse://title/kestrel-2024")))
        links.offer(Intent(Intent.ACTION_MAIN))
        assertEquals(AppLink.OpenTitle("kestrel-2024"), links.consume())
        assertNull(links.consume())
    }
}
