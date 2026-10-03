package io.stepes.couchverse.settings.tv

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ExitToApp
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Person
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Icon
import androidx.tv.material3.ListItem
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.RadioButton
import androidx.tv.material3.Text
import io.stepes.couchverse.core.AccountCard
import io.stepes.couchverse.core.Server
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.components.InsecureBadge
import io.stepes.couchverse.design.theme.LocalAccent
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.ranks.rankLine
import io.stepes.couchverse.settings.ConfirmDialog
import io.stepes.couchverse.settings.DisplayLanguages
import io.stepes.couchverse.settings.SettingsActions
import io.stepes.couchverse.settings.SettingsState
import io.stepes.couchverse.settings.languageName
import io.stepes.couchverse.design.tv.focusOnStart

/** One focusable list, top to bottom; OK on an account or server asks to sign out or remove it. */
@Composable
internal fun SettingsTv(state: SettingsState, actions: SettingsActions) {
    var signingOut by remember { mutableStateOf<AccountCard?>(null) }
    var removing by remember { mutableStateOf<Server?>(null) }
    val first = remember { FocusRequester() }
    LazyColumn(
        Modifier.fillMaxSize().widthIn(max = 720.dp),
        contentPadding = PaddingValues(horizontal = TvSafe.horizontal, vertical = TvSafe.vertical),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        item(key = "title") {
            Text(
                stringResource(R.string.nav_settings),
                style = MaterialTheme.typography.headlineMedium,
                color = Tokens.Palette.text,
                modifier = Modifier.padding(bottom = 12.dp).semantics { heading() },
            )
        }
        item(key = "switch") {
            val current = state.current
            ListItem(
                selected = false,
                onClick = actions.onSwitchAccount,
                modifier = Modifier.focusOnStart(first),
                leadingContent = { if (current != null) Avatar(current.avatarUrl, seed = current.username, size = 44.dp) },
                headlineContent = { Text(current?.displayName ?: stringResource(R.string.accounts_switch)) },
                supportingContent = { Text(stringResource(R.string.accounts_switch)) },
                trailingContent = { Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, contentDescription = null) },
            )
        }
        actions.onProfile?.let { open ->
            item(key = "profile") { Action(Icons.Filled.Person, stringResource(R.string.nav_public_profile), open, supporting = state.rank?.let { rankLine(it) }) }
        }
        actions.onLeaderboard?.let { open ->
            item(key = "leaderboard") { Action(CouchverseIcons.Leaderboard, stringResource(R.string.nav_leaderboard), open) }
        }
        actions.onJoinCouch?.let { open ->
            item(key = "join-couch") { Action(CouchverseIcons.Couch, stringResource(R.string.couch_join_title), open) }
        }

        section("accounts", R.string.accounts_title)
        items(state.accounts?.accounts.orEmpty(), key = { "account-${it.id}" }) { account ->
            ListItem(
                selected = false,
                onClick = { signingOut = account },
                leadingContent = { Avatar(account.avatarUrl, seed = account.username, size = 40.dp) },
                headlineContent = { Text(account.displayName) },
                supportingContent = {
                    Text(if (account.signedIn) account.serverName else stringResource(R.string.accounts_sign_in_again))
                },
                trailingContent = { Icon(Icons.AutoMirrored.Filled.ExitToApp, contentDescription = stringResource(R.string.devices_sign_out)) },
            )
        }
        item(key = "add-account") { Action(Icons.Filled.Add, stringResource(R.string.accounts_add_account), actions.onAddAccount) }

        section("servers", R.string.servers_title)
        items(state.servers?.servers.orEmpty(), key = { "server-${it.id}" }) { server ->
            ListItem(
                selected = false,
                onClick = { removing = server },
                leadingContent = { Icon(CouchverseIcons.Server, contentDescription = null) },
                headlineContent = { Text(server.name) },
                supportingContent = {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text("${server.url}  ·  " + stringResource(R.string.servers_version, server.version))
                        if (server.insecure) InsecureBadge()
                    }
                },
                trailingContent = { Icon(Icons.Filled.Delete, contentDescription = stringResource(R.string.servers_remove)) },
            )
        }
        item(key = "add-server") { Action(Icons.Filled.Add, stringResource(R.string.servers_add_title), actions.onAddServer) }

        section("language", R.string.language_label)
        items(DisplayLanguages, key = { "lang-$it" }) { code ->
            val selected = state.session?.language == code
            ListItem(
                selected = selected,
                onClick = { actions.onLanguage(code) },
                leadingContent = { RadioButton(selected = selected, onClick = null) },
                headlineContent = { Text(languageName(code)) },
            )
        }

        section("devices", R.string.devices_heading)
        item(key = "devices") { Action(CouchverseIcons.Devices, stringResource(R.string.devices_heading), actions.onDevices) }
        item(key = "approve") { Action(CouchverseIcons.ScanCode, stringResource(R.string.pair_heading), actions.onApprove) }

        section("about", R.string.about_title)
        item(key = "version") {
            Row(Modifier.padding(16.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                Icon(Icons.Filled.Info, contentDescription = null, tint = Tokens.Palette.muted)
                Column {
                    Text(stringResource(R.string.about_app_version), color = Tokens.Palette.text)
                    Text(state.version, color = Tokens.Palette.muted)
                }
            }
        }
    }
    signingOut?.let { account ->
        ConfirmDialog(
            title = stringResource(R.string.accounts_sign_out_confirm_title, account.displayName),
            message = stringResource(R.string.accounts_sign_out_confirm_message),
            confirm = stringResource(R.string.devices_sign_out),
            onConfirm = { actions.onSignOut(account) },
            onDismiss = { signingOut = null },
        )
    }
    removing?.let { server ->
        ConfirmDialog(
            title = stringResource(R.string.servers_remove_confirm_title, server.name),
            message = stringResource(R.string.servers_remove_confirm_message),
            confirm = stringResource(R.string.servers_remove),
            onConfirm = { actions.onRemoveServer(server) },
            onDismiss = { removing = null },
        )
    }
}

private fun LazyListScope.section(key: String, title: Int) {
    item(key = "section-$key") {
        Text(
            stringResource(title),
            style = MaterialTheme.typography.titleSmall,
            color = LocalAccent.current.ink,
            modifier = Modifier.padding(top = 20.dp, bottom = 4.dp).semantics { heading() },
        )
    }
}

@Composable
private fun Action(icon: ImageVector, label: String, onClick: () -> Unit, supporting: String? = null) {
    ListItem(
        selected = false,
        onClick = onClick,
        leadingContent = { Icon(icon, contentDescription = null, modifier = Modifier.size(24.dp)) },
        headlineContent = { Text(label) },
        supportingContent = supporting?.let { { Text(it) } },
    )
}
