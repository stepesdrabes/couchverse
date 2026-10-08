package io.stepes.couchverse.couch

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import io.stepes.couchverse.core.CouchCode
import io.stepes.couchverse.core.CouchStatus
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.Fixture
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import kotlin.test.assertEquals

@Config(qualifiers = RobolectricDeviceQualifiers.Pixel7)
@RunWith(RobolectricTestRunner::class)
class JoinCouchTest {
    @get:Rule
    val compose = createComposeRule()

    private val joined = mutableListOf<CouchCode>()
    private val remotes = mutableListOf<String>()
    private val actions = JoinCouchActions(onJoin = { joined += it }, onRemote = { remotes += it }, onScan = null, onBack = {})

    @Test
    fun `a guest joins at the server's address and hears why it failed`() {
        var view by mutableStateOf<CouchView?>(null)
        compose.setContent { Fixture(Device.Phone) { JoinCouchScreen(view, CouchInvite("123456"), guest = true, actions) } }

        compose.onNodeWithText("Use as a remote").assertDoesNotExist()
        compose.onNodeWithText("Join").assertIsNotEnabled()
        compose.onNodeWithText("Server address").performTextInput(" tv.local:8080 ")
        compose.onNodeWithText("Join").performClick()
        assertEquals(listOf(CouchCode("123456", "tv.local:8080")), joined)

        view = idle.copy(problem = Problem("invalid_address", "nothing to connect to"))
        compose.onNodeWithText("Enter an address like media.example.com or 192.168.1.5:8080.").assertIsDisplayed()
    }

    @Test
    fun `with an account a link's server goes along unasked`() {
        compose.setContent {
            Fixture(Device.Phone) { JoinCouchScreen(null, CouchInvite("123456", "https://friend.example.org"), guest = false, actions) }
        }

        compose.onNodeWithText("Server address").assertDoesNotExist()
        compose.onNodeWithText("Join").performClick()
        assertEquals(listOf(CouchCode("123456", "https://friend.example.org")), joined)
        // a remote steers the account's own player, on the account's own server
        compose.onNodeWithText("Use as a remote").performClick()
        assertEquals(listOf("123456"), remotes)
    }

    @Test
    fun `an old failure is not this form's`() {
        compose.setContent { Fixture(Device.Phone) { JoinCouchScreen(idle.copy(problem = Problem("no_session", "")), CouchInvite("123456"), guest = false, actions) } }
        compose.onNodeWithText("Something went wrong. Try again.").assertDoesNotExist()
    }

    private val idle = CouchView(
        status = CouchStatus.Idle,
        members = emptyList(),
        playing = false,
        positionSeconds = 0.0,
        positionAtMs = 0u,
        hostAway = false,
        waiting = false,
        localPaused = false,
        reactions = emptyList(),
        recentEmojis = emptyList(),
        resynced = false,
    )
}
