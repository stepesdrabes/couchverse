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
    private val actions = JoinCouchActions(onJoin = {}, onRemote = {}, onScan = {}, onBack = {})

    @Test
    fun `couch links carry their code and the server they name`() {
        assertEquals(CouchInvite("123456"), couchInvite("couchverse://couch/123456"))
        assertEquals(
            CouchInvite("123456", "http://192.168.1.5:8080"),
            couchInvite("couchverse://couch/123456?server=http%3A%2F%2F192.168.1.5%3A8080"),
        )
        assertEquals(CouchInvite("123456"), couchInvite("couchverse://couch/123456?server="))
        // a join page's server is its origin
        assertEquals(CouchInvite("123456", "https://media.example.com"), couchInvite("https://media.example.com/couch/123456"))
        assertEquals(CouchInvite("654321", "http://10.0.2.2:8092"), couchInvite("http://10.0.2.2:8092/couch/654321/"))
        assertNull(couchInvite("https://media.example.com/pair?code=WDJB-MJHT"))
        assertNull(couchInvite("https://media.example.com/couch/12345"))
        assertNull(couchInvite("couchverse://pair?code=123456"))
        assertNull(couchInvite("not a link"))
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
                JoinCouchScreen(null, CouchInvite("123456"), guest = false, actions)
            }
            // without an account the server is asked for too, and there is no player to steer
            screenshot("couch_join_guest", device, "cs") {
                JoinCouchScreen(null, CouchInvite("123456"), guest = true, actions)
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
