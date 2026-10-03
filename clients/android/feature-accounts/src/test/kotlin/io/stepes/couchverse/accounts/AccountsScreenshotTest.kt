package io.stepes.couchverse.accounts

import io.stepes.couchverse.core.AccountCard
import io.stepes.couchverse.core.AccountsView
import io.stepes.couchverse.core.AddServerView
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.PairingState
import io.stepes.couchverse.core.PairingView
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.core.Server
import io.stepes.couchverse.core.ServersView
import io.stepes.couchverse.core.SignInView
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.Languages
import io.stepes.couchverse.testing.screenshot
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** Onboarding, sign-in and "Who's watching?" on a phone and a TV. */
@RunWith(RobolectricTestRunner::class)
class AccountsScreenshotTest {
    private val actions = SignInActions(onPassword = { _, _ -> }, onStartPairing = {}, onCancelPairing = {}, onBack = {})

    @Test
    fun welcome() = Device.entries.forEach { device ->
        Languages.forEach { language -> screenshot("welcome", device, language) { WelcomeScreen({}, {}) } }
    }

    @Test
    fun `adding a server that does not answer`() = Device.entries.forEach { device ->
        screenshot("add_server_failed", device) {
            AddServerScreen(
                ServersView(emptyList(), AddServerView(LoadStatus.Failed, "media.example.com", problem = Problem("not_a_server", ""))),
                onSubmit = {},
                onBack = {},
                onScan = {},
            )
        }
    }

    @Test
    fun `password sign-in on a phone`() = Languages.forEach { language ->
        screenshot("sign_in", Device.Phone, language) {
            SignInScreen(SignInState(Fixtures.insecureServer, SignInView(status = LoadStatus.Idle), 0, "nora"), actions)
        }
    }

    @Test
    fun `pairing on a TV`() {
        Languages.forEach { language ->
            screenshot("sign_in_pairing", Device.Tv, language) {
                SignInScreen(SignInState(Fixtures.server, Fixtures.pairing(PairingState.Waiting), 581), actions)
            }
        }
        screenshot("sign_in_pairing_expired", Device.Tv) {
            SignInScreen(SignInState(Fixtures.server, Fixtures.pairing(PairingState.Expired), 0), actions)
        }
        screenshot("sign_in_pairing", Device.Phone) {
            SignInScreen(SignInState(Fixtures.server, Fixtures.pairing(PairingState.Waiting), 581), actions)
        }
    }

    @Test
    fun `who's watching`() {
        Languages.forEach { language -> screenshot("whos_watching", Device.Tv, language) { WhosWatchingScreen(Fixtures.accounts, {}, {}) } }
        screenshot("whos_watching", Device.Phone) { WhosWatchingScreen(Fixtures.accounts, {}, {}) }
    }

    @Test
    fun connecting() = screenshot("connecting_failed", Device.Phone) {
        ConnectingScreen(SignInView(status = LoadStatus.Failed, problem = Problem("invalid_code", "")), onBack = {})
    }
}

internal object Fixtures {
    val server = Server("s1", "https://media.example.com", "Home Media", "1.4.0", 2u, "#3a6ea5", insecure = false)
    val insecureServer = server.copy(url = "http://192.168.1.20:8080", insecure = true)

    fun pairing(state: PairingState) = SignInView(
        serverId = "s1",
        status = LoadStatus.Idle,
        pairing = PairingView("WDJB-MJHT", "https://media.example.com/pair?code=WDJB-MJHT", 600_000u, state),
    )

    private fun account(id: String, name: String, signedIn: Boolean = true, insecure: Boolean = false) =
        AccountCard(id, "s1", "Home Media", insecure, name.lowercase(), name, avatarUrl = null, signedIn = signedIn)

    val accounts = AccountsView(
        accounts = listOf(account("s1/1", "Admin"), account("s1/2", "Nora"), account("s1/5", "Otto", signedIn = false), account("s1/6", "Vera")),
        active = "s1/2",
    )
}
