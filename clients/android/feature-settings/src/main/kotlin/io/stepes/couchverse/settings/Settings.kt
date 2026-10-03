package io.stepes.couchverse.settings

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import io.stepes.couchverse.core.AccountCard
import io.stepes.couchverse.core.AccountRef
import io.stepes.couchverse.core.AccountsView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.LanguageChoice
import io.stepes.couchverse.core.Server
import io.stepes.couchverse.core.ServerRef
import io.stepes.couchverse.core.ServersView
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.settings.phone.SettingsPhone
import io.stepes.couchverse.settings.tv.SettingsTv

class SettingsState(
    val session: SessionView?,
    val accounts: AccountsView?,
    val servers: ServersView?,
    /** The app's version, for About. */
    val version: String,
) {
    val current: AccountCard? get() = accounts?.accounts?.firstOrNull { it.id == accounts.active }
}

class SettingsActions(
    val onSwitchAccount: () -> Unit,
    val onAddAccount: () -> Unit,
    val onSignOut: (AccountCard) -> Unit,
    val onAddServer: () -> Unit,
    val onRemoveServer: (Server) -> Unit,
    val onLanguage: (String) -> Unit,
    val onDevices: () -> Unit,
    val onApprove: () -> Unit,
)

/** The display languages the app ships, as their own names call them. */
val DisplayLanguages = listOf("en", "cs")

/** Servers, accounts, the display language, devices and approving another device's sign-in. */
@Composable
fun SettingsScreen(state: SettingsState, actions: SettingsActions) {
    if (LocalIsTv.current) SettingsTv(state, actions) else SettingsPhone(state, actions)
}

@Composable
fun SettingsRoute(
    version: String,
    onSwitchAccount: () -> Unit,
    onAddAccount: () -> Unit,
    onAddServer: () -> Unit,
    onDevices: () -> Unit,
    onApprove: () -> Unit,
) {
    val send = rememberSend()
    val session by rememberSurface<SessionView>(Surface.Session)
    val accounts by rememberSurface<AccountsView>(Surface.Accounts)
    val servers by rememberSurface<ServersView>(Surface.Servers)
    SettingsScreen(
        SettingsState(session, accounts, servers, version),
        SettingsActions(
            onSwitchAccount = onSwitchAccount,
            onAddAccount = onAddAccount,
            onSignOut = { send(Event.SignOutRequested(AccountRef(it.id))) },
            onAddServer = onAddServer,
            onRemoveServer = { send(Event.ServerRemoved(ServerRef(it.id))) },
            onLanguage = { send(Event.DisplayLanguageChanged(LanguageChoice(it))) },
            onDevices = onDevices,
            onApprove = onApprove,
        ),
    )
}
