package io.stepes.couchverse.accounts

import androidx.compose.ui.input.key.Key
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsFocused
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.performKeyInput
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.pressKey
import io.stepes.couchverse.core.AccountCard
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.PairingState
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.core.SignInView
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.Fixture
import androidx.test.platform.app.InstrumentationRegistry
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import kotlin.test.assertEquals
import kotlin.test.assertTrue

@Config(qualifiers = RobolectricDeviceQualifiers.Pixel7)
@RunWith(RobolectricTestRunner::class)
class AccountsUiTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun `an address is submitted trimmed`() {
        val submitted = mutableListOf<String>()
        compose.setContent { Fixture(Device.Phone) { AddServerScreen(null, onSubmit = { submitted += it }, onBack = null, onScan = {}) } }
        compose.onNodeWithText("Server address").performTextInput("  tv.home:8080 ")
        compose.onNodeWithText("Connect").performClick()
        assertEquals(listOf("tv.home:8080"), submitted)
    }

    @Test
    fun `a password sign-in sends the form and shows why it failed`() {
        var sent: Pair<String, String>? = null
        val actions = SignInActions({ user, password -> sent = user to password }, {}, {}, null)
        val failed = SignInView(serverId = "s1", status = LoadStatus.Failed, problem = Problem("invalid_credentials", ""))
        compose.setContent { Fixture(Device.Phone) { SignInScreen(SignInState(Fixtures.server, failed, 0, "nora"), actions) } }

        compose.onNodeWithText("Wrong username or password.").assertIsDisplayed()
        compose.onNodeWithText("Password").performTextInput("hunter2")
        compose.onNodeWithText("Sign in").performClick()
        assertEquals("nora" to "hunter2", sent)
    }

    @Config(qualifiers = RobolectricDeviceQualifiers.Television1080p)
    @Test
    fun `an expired pairing code offers a new one`() {
        var restarted = false
        val actions = SignInActions({ _, _ -> }, { restarted = true }, {}, null)
        compose.setContent { Fixture(Device.Tv) { SignInScreen(SignInState(Fixtures.server, Fixtures.pairing(PairingState.Expired), 0), actions) } }
        compose.onNodeWithText("The code has expired").assertIsDisplayed()
        // TV buttons answer the remote, not touch
        compose.onNodeWithText("Get a new code").performSemanticsAction(SemanticsActions.OnClick)
        assertTrue(restarted)
    }

    @Config(qualifiers = RobolectricDeviceQualifiers.Television1080p)
    @Test
    fun `a waiting pairing shows the code and the time left`() {
        val actions = SignInActions({ _, _ -> }, {}, {}, null)
        compose.setContent { Fixture(Device.Tv) { SignInScreen(SignInState(Fixtures.server, Fixtures.pairing(PairingState.Waiting), 125), actions) } }
        compose.onNodeWithText("WDJB-MJHT").assertIsDisplayed()
        compose.onNodeWithText("Expires in 2:05").assertIsDisplayed()
        compose.onNodeWithContentDescription("QR code for https://media.example.com/pair?code=WDJB-MJHT").assertIsDisplayed()
    }

    @OptIn(ExperimentalTestApi::class)
    @Config(qualifiers = RobolectricDeviceQualifiers.Television1080p)
    @Test
    fun `who's watching starts on the last account and moves with the remote`() {
        val picked = mutableListOf<AccountCard>()
        // a remote, not a finger: focus works as on a TV
        InstrumentationRegistry.getInstrumentation().setInTouchMode(false)
        compose.setContent { Fixture(Device.Tv) { WhosWatchingScreen(Fixtures.accounts, onPick = { picked += it }, onAdd = {}) } }
        compose.waitForIdle()
        compose.onNodeWithContentDescription("Nora, Home Media").assertIsFocused()

        compose.onRoot().performKeyInput { pressKey(Key.DirectionRight) }
        compose.onNodeWithContentDescription("Otto, Home Media, Sign in again").assertIsFocused()
        compose.onRoot().performKeyInput { pressKey(Key.DirectionCenter) }
        compose.waitForIdle()
        assertEquals(listOf("Otto"), picked.map { it.displayName })
    }

    @Test
    fun `scanned text is a Couchverse link or not`() {
        assertTrue(isCouchverseLink("couchverse://connect?server=https%3A%2F%2Ftv.home&code=abc"))
        assertTrue(isCouchverseLink("couchverse://pair?code=WDJB-MJHT"))
        assertTrue(isCouchverseLink("https://tv.home/pair?code=WDJB-MJHT"))
        assertTrue(!isCouchverseLink("https://tv.home/title/glass-harbor"))
        assertTrue(!isCouchverseLink("WIFI:S:home;T:WPA;P:secret;;"))
        assertTrue(isConnectLink("couchverse://connect?server=x&code=y"))
    }

    @Test
    fun `countdowns round up to whole seconds`() {
        assertEquals(600, secondsLeft(600_000, 0))
        assertEquals(1, secondsLeft(600_000, 599_001))
        assertEquals(0, secondsLeft(600_000, 700_000))
        assertEquals("9:41", formatCountdown(581))
        assertEquals("https://tv.home/pair", pairingPage("https://tv.home/pair?code=WDJB-MJHT"))
    }
}
