package io.stepes.couchverse.settings.phone

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.selection.selectable
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ExitToApp
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Info
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.AccountCard
import io.stepes.couchverse.core.Server
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.components.InsecureBadge
import io.stepes.couchverse.design.phone.PhoneGutter
import io.stepes.couchverse.settings.ConfirmDialog
import io.stepes.couchverse.settings.DisplayLanguages
import io.stepes.couchverse.settings.SettingsActions
import io.stepes.couchverse.settings.SettingsState
import io.stepes.couchverse.settings.languageName

@Composable
internal fun SettingsPhone(state: SettingsState, actions: SettingsActions) {
    var signingOut by remember { mutableStateOf<AccountCard?>(null) }
    var removing by remember { mutableStateOf<Server?>(null) }
    LazyColumn(Modifier.fillMaxSize().statusBarsPadding(), contentPadding = PaddingValues(bottom = 24.dp)) {
        item(key = "title") {
            Text(
                stringResource(R.string.nav_settings),
                style = MaterialTheme.typography.headlineMedium,
                color = Tokens.Palette.text,
                modifier = Modifier.padding(horizontal = PhoneGutter, vertical = 12.dp).semantics { heading() },
            )
        }
        state.current?.let { current ->
            item(key = "current") { CurrentAccount(current, actions.onSwitchAccount) }
        }

        section("accounts", R.string.app_settings_accounts)
        items(state.accounts?.accounts.orEmpty(), key = { "account-${it.id}" }) { account ->
            SettingRow(
                headline = account.displayName,
                supporting = if (account.signedIn) account.serverName else stringResource(R.string.accounts_sign_in_again),
                leading = { Avatar(account.avatarUrl, seed = account.username, size = 40.dp) },
                trailing = {
                    IconButton(onClick = { signingOut = account }) {
                        Icon(Icons.AutoMirrored.Filled.ExitToApp, contentDescription = stringResource(R.string.devices_sign_out_device, account.displayName))
                    }
                },
            )
        }
        item(key = "add-account") { ActionRow(Icons.Filled.Add, stringResource(R.string.accounts_add), actions.onAddAccount) }

        section("servers", R.string.app_settings_servers)
        items(state.servers?.servers.orEmpty(), key = { "server-${it.id}" }) { server ->
            SettingRow(
                headline = server.name,
                supporting = server.url,
                leading = { Icon(CouchverseIcons.Server, contentDescription = null) },
                below = {
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            stringResource(R.string.servers_version, server.version),
                            style = MaterialTheme.typography.bodySmall,
                            color = Tokens.Palette.faint,
                        )
                        if (server.insecure) InsecureBadge()
                    }
                },
                trailing = {
                    IconButton(onClick = { removing = server }) {
                        Icon(Icons.Filled.Delete, contentDescription = stringResource(R.string.servers_remove))
                    }
                },
            )
        }
        item(key = "add-server") { ActionRow(Icons.Filled.Add, stringResource(R.string.servers_add), actions.onAddServer) }

        section("language", R.string.language_label)
        items(DisplayLanguages, key = { "lang-$it" }) { code ->
            val selected = state.session?.language == code
            ListItem(
                colors = ListItemDefaults.colors(containerColor = Color.Transparent),
                modifier = Modifier.selectable(selected, role = Role.RadioButton) { actions.onLanguage(code) },
                leadingContent = { RadioButton(selected = selected, onClick = null) },
                headlineContent = { Text(languageName(code)) },
            )
        }

        section("devices", R.string.devices_heading)
        item(key = "devices") { ActionRow(CouchverseIcons.Devices, stringResource(R.string.devices_heading), actions.onDevices, chevron = true) }
        item(key = "approve") { ActionRow(CouchverseIcons.ScanCode, stringResource(R.string.pair_heading), actions.onApprove, chevron = true) }

        section("about", R.string.app_settings_about)
        item(key = "version") {
            ListItem(
                colors = ListItemDefaults.colors(containerColor = Color.Transparent),
                leadingContent = { Icon(Icons.Filled.Info, contentDescription = null) },
                headlineContent = { Text(stringResource(R.string.app_settings_app_version, state.version)) },
            )
        }
    }
    signingOut?.let { account ->
        ConfirmDialog(
            title = stringResource(R.string.accounts_sign_out_title, account.displayName),
            message = stringResource(R.string.accounts_sign_out_message),
            confirm = stringResource(R.string.devices_sign_out),
            onConfirm = { actions.onSignOut(account) },
            onDismiss = { signingOut = null },
        )
    }
    removing?.let { server ->
        ConfirmDialog(
            title = stringResource(R.string.servers_remove_title, server.name),
            message = stringResource(R.string.servers_remove_message),
            confirm = stringResource(R.string.servers_remove),
            onConfirm = { actions.onRemoveServer(server) },
            onDismiss = { removing = null },
        )
    }
}

@Composable
private fun CurrentAccount(account: AccountCard, onSwitch: () -> Unit) {
    Column(Modifier.fillMaxWidth().padding(horizontal = PhoneGutter, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Row(horizontalArrangement = Arrangement.spacedBy(16.dp), verticalAlignment = Alignment.CenterVertically) {
            Avatar(account.avatarUrl, seed = account.username, size = 64.dp)
            Column(Modifier.weight(1f)) {
                Text(account.displayName, style = MaterialTheme.typography.titleLarge, color = Tokens.Palette.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(account.serverName, style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.muted)
                if (account.insecure) InsecureBadge(Modifier.padding(top = 6.dp))
            }
        }
        FilledTonalButton(onClick = onSwitch, modifier = Modifier.fillMaxWidth()) { Text(stringResource(R.string.accounts_switch)) }
    }
}

private fun LazyListScope.section(key: String, title: Int) {
    item(key = "section-$key") {
        Column {
            HorizontalDivider(color = Tokens.Palette.edge, modifier = Modifier.padding(top = 12.dp))
            Text(
                stringResource(title),
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(start = PhoneGutter, end = PhoneGutter, top = 16.dp, bottom = 4.dp).semantics { heading() },
            )
        }
    }
}

@Composable
private fun SettingRow(
    headline: String,
    supporting: String?,
    leading: @Composable () -> Unit,
    trailing: @Composable () -> Unit,
    below: (@Composable () -> Unit)? = null,
) {
    ListItem(
        colors = ListItemDefaults.colors(containerColor = Color.Transparent),
        leadingContent = leading,
        headlineContent = { Text(headline, maxLines = 1, overflow = TextOverflow.Ellipsis) },
        supportingContent = {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                if (supporting != null) Text(supporting, maxLines = 1, overflow = TextOverflow.Ellipsis)
                below?.invoke()
            }
        },
        trailingContent = trailing,
    )
}

@Composable
private fun ActionRow(icon: ImageVector, label: String, onClick: () -> Unit, chevron: Boolean = false) {
    ListItem(
        colors = ListItemDefaults.colors(containerColor = Color.Transparent),
        modifier = Modifier.clickable(role = Role.Button, onClick = onClick),
        leadingContent = { Icon(icon, contentDescription = null) },
        headlineContent = { Text(label) },
        trailingContent = if (chevron) {
            { Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, contentDescription = null) }
        } else {
            null
        },
    )
}
