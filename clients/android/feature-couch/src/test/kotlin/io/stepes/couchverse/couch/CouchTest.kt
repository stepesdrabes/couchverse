package io.stepes.couchverse.couch

import io.stepes.couchverse.core.CouchMember
import io.stepes.couchverse.core.CouchRole
import io.stepes.couchverse.core.CouchStatus
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.screenshot
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import kotlin.test.assertEquals
import kotlin.test.assertNull

@RunWith(RobolectricTestRunner::class)
class CouchTest {
    @Test
    fun `couch links carry their code`() {
        assertEquals("123456", couchCode("couchverse://couch/123456"))
        assertEquals("123456", couchCode("https://media.example.com/couch/123456"))
        assertEquals("654321", couchCode("http://10.0.2.2:8092/couch/654321/"))
        assertNull(couchCode("https://media.example.com/pair?code=WDJB-MJHT"))
        assertNull(couchCode("https://media.example.com/couch/12345"))
    }

    @Test
    fun `recent reactions come first, without repeats`() {
        val choices = reactionChoices(listOf("🦄", "🔥"))
        assertEquals(listOf("🦄", "🔥", "❤️"), choices.take(3))
        assertEquals(choices.distinct(), choices)
        assertEquals("123 456", spacedCode("123456"))
    }

    @Test
    fun `the host's panel, joining, and the phone as a remote`() {
        screenshot("couch_panel", Device.Phone) {
            CouchPanel(hosting, onEnd = {}, onLeave = {}, qrSize = 140.dp) { label, onClick ->
                androidx.compose.material3.OutlinedButton(onClick = onClick) { androidx.compose.material3.Text(label) }
            }
        }
        Device.entries.forEach { device ->
            screenshot("couch_join", device) {
                JoinCouchScreen(null, "123456", JoinCouchActions(onJoin = {}, onRemote = {}, onScan = {}, onBack = {}))
            }
        }
        screenshot("couch_remote", Device.Phone, "cs") {
            CouchRemoteScreen(hosting.copy(role = CouchRole.Remote), positionSeconds = 754.0, onCommand = {}, onLeave = {})
        }
    }

    private val hosting = CouchView(
        status = CouchStatus.Open,
        role = CouchRole.Host,
        code = "123456",
        shareUrl = "https://media.example.com/couch/123456",
        members = listOf(
            CouchMember("p1", "Nora", seed = "nora", host = true, anonymous = false, paused = false, me = true),
            CouchMember("p2", "Otto", seed = "otto", host = false, anonymous = false, paused = false, me = false),
            CouchMember("p3", "Guest", seed = "guest-3", host = false, anonymous = true, paused = true, me = false),
        ),
        playing = true,
        positionSeconds = 754.0,
        positionAtMs = 0u,
        hostAway = false,
        waiting = false,
        localPaused = false,
        reactions = emptyList(),
        recentEmojis = emptyList(),
        resynced = false,
    )

    private val Int.dp get() = androidx.compose.ui.unit.Dp(toFloat())
}
