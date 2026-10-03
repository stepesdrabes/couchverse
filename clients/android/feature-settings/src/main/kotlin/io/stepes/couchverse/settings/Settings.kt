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
    /** The viewer's own profile and the leaderboard, with rankings on. */
    val onProfile: (() -> Unit)? = null,
    val onLeaderboard: (() -> Unit)? = null,
    /** The downloads on this phone, with downloads on. */
    val onDownloads: (() -> Unit)? = null,
    /** Joining someone's couch session by its code, with couch sessions on. */
    val onJoinCouch: (() -> Unit)? = null,
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
    onProfile: ((username: String) -> Unit)? = null,
    onLeaderboard: (() -> Unit)? = null,
    onDownloads: (() -> Unit)? = null,
    onJoinCouch: (() -> Unit)? = null,
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
            onProfile = session?.user?.username?.let { me -> onProfile?.let { open -> { open(me) } } }
                ?.takeIf { session?.features?.rankings == true },
            onLeaderboard = onLeaderboard?.takeIf { session?.features?.rankings == true },
            onDownloads = onDownloads?.takeIf { session?.features?.downloads == true },
            onJoinCouch = onJoinCouch?.takeIf { session?.features?.couch == true },
        ),
    )
}
