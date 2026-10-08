package io.stepes.couchverse.settings

import androidx.compose.ui.test.hasScrollToIndexAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performScrollToNode
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import io.stepes.couchverse.core.AccentPalette
import io.stepes.couchverse.core.AccountCard
import io.stepes.couchverse.core.AccountsView
import io.stepes.couchverse.core.AddServerView
import io.stepes.couchverse.core.ApprovalOutcome
import io.stepes.couchverse.core.DeviceCard
import io.stepes.couchverse.core.DevicesView
import io.stepes.couchverse.core.Features
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.PairingApprovalView
import io.stepes.couchverse.core.RankBadge
import io.stepes.couchverse.core.Server
import io.stepes.couchverse.core.ServersView
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.Tier
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.Fixture
import io.stepes.couchverse.testing.Languages
import io.stepes.couchverse.testing.screenshot
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import org.junit.Rule
import java.time.Instant
import java.time.temporal.ChronoUnit
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import kotlin.test.assertEquals

@Config(qualifiers = RobolectricDeviceQualifiers.Pixel7)
@RunWith(RobolectricTestRunner::class)
class SettingsScreenshotTest {
    @get:Rule
    val compose = createComposeRule()

    private val accounts = AccountsView(
        listOf(
            AccountCard("s1/2", "s1", "Home Media", false, "nora", "Nora", signedIn = true),
            AccountCard("s2/1", "s2", "Cabin", true, "admin", "Admin", signedIn = false),
        ),
        active = "s1/2",
    )
    private val servers = ServersView(
        listOf(
            Server("s1", "https://media.example.com", "Home Media", "1.4.0", 2u, "#e50914", false),
            Server("s2", "http://192.168.1.20:8080", "Cabin", "1.3.2", 2u, "#3a6ea5", true),
        ),
        AddServerView(LoadStatus.Idle, ""),
    )
    private val session = SessionView(
        status = LoadStatus.Loaded,
        accountId = "s1/2",
        features = Features(couch = true, rankings = true, downloads = true),
        language = "en",
        accent = AccentPalette("#e50914", "#b30710", "#e5091429", "#ffffff", "#ec474f"),
        offline = false,
    )
    private val rank = RankBadge(Tier("binger", 4u, "#34d399", 1500u), Tier("popcorn", 5u, "#fbbf24", 3000u), 2140u, 43u)
    private val state = SettingsState(session, accounts, servers, "1.4.0", rank)
    private val actions = SettingsActions({}, {}, {}, {}, {}, {}, {}, {}, onProfile = {}, onLeaderboard = {}, onDownloads = {}, onJoinCouch = {})
    private val approve = ApproveActions({}, { _, _ -> }, {}, {}, {}, {})

    @Test
    fun settings() = Device.entries.forEach { device ->
        Languages.forEach { language -> screenshot("settings", device, language) { SettingsScreen(state, actions) } }
    }

    @Test
    fun `devices and approving a pairing`() {
        val devices = DevicesView(
            LoadStatus.Loaded,
            listOf(
                DeviceCard("d1", "Pixel 9", "android", ago(0), current = true),
                DeviceCard("d2", "Living Room", "androidtv", ago(2), current = false),
                DeviceCard("d3", "Firefox on Mac", "web", ago(21), current = false),
            ),
        )
        screenshot("devices", Device.Phone) { DevicesScreen(devices, {}, {}, onBack = {}) }
        val request = PairingApprovalView(LoadStatus.Loaded, "WDJB-MJHT", "Living Room", "androidtv")
        screenshot("approve_request", Device.Phone) { ApproveScreen(request, "Nora", approve) }
        screenshot("approve_done", Device.Tv, "cs") {
            ApproveScreen(request.copy(outcome = ApprovalOutcome.Approved), "Nora", approve)
        }
    }

    // relative to now, so "2 days ago" reads the same on any day
    private fun ago(days: Long) = Instant.now().minus(days, ChronoUnit.DAYS).minusSeconds(60).toString()

    @Test
    fun `a typed code is looked up in capitals`() {
        val looked = mutableListOf<String>()
        compose.setContent {
            Fixture(Device.Phone) { ApproveScreen(null, "Nora", ApproveActions({ looked += it }, { _, _ -> }, {}, {}, null, {})) }
        }
        compose.onNodeWithText("Sign-in code").performTextInput(" wdjb-mjht ")
        compose.onNodeWithText("Continue").performClick()
        assertEquals(listOf("WDJB-MJHT"), looked)
    }

    @Test
    fun `picking a language sends it`() {
        val picked = mutableListOf<String>()
        compose.setContent {
            Fixture(Device.Phone) { SettingsScreen(state, SettingsActions({}, {}, {}, {}, {}, { picked += it }, {}, {})) }
        }
        compose.onAllNodes(hasScrollToIndexAction()).onFirst().performScrollToNode(hasText("Čeština"))
        compose.onNodeWithText("Čeština").performClick()
        assertEquals(listOf("cs"), picked)
    }
}
