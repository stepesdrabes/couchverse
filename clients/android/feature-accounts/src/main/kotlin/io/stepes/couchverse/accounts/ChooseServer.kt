package io.stepes.couchverse.accounts

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.accounts.phone.OnboardingPage
import io.stepes.couchverse.accounts.tv.TvOnboardingPage
import io.stepes.couchverse.core.Server
import io.stepes.couchverse.core.ServersView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.components.InsecureBadge
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalAccent
import io.stepes.couchverse.design.theme.LocalIsTv
import androidx.tv.material3.Icon as TvIcon
import androidx.tv.material3.ListItem as TvListItem
import androidx.tv.material3.Text as TvText
import io.stepes.couchverse.design.tv.focusOnStart

/** Which server to sign in to, when this device knows several and none has an account yet. */
@Composable
fun ChooseServerScreen(view: ServersView?, onPick: (Server) -> Unit, onAddServer: () -> Unit) {
    val servers = view?.servers.orEmpty()
    if (LocalIsTv.current) {
        val first = remember { FocusRequester() }
        TvOnboardingPage {
            TvText(stringResource(R.string.accounts_choose_server), style = androidx.tv.material3.MaterialTheme.typography.headlineMedium)
            servers.forEachIndexed { index, server ->
                TvListItem(
                    selected = false,
                    onClick = { onPick(server) },
                    headlineContent = { TvText(server.name) },
                    supportingContent = { TvText(server.url) },
                    leadingContent = { TvIcon(CouchverseIcons.Server, contentDescription = null) },
                    trailingContent = { if (server.insecure) InsecureBadge() },
                    modifier = if (index == 0) Modifier.focusOnStart(first) else Modifier,
                )
            }
            TvListItem(selected = false, onClick = onAddServer, headlineContent = { TvText(stringResource(R.string.servers_add_title)) })
        }
    } else {
        OnboardingPage(onBack = null) {
            Text(
                stringResource(R.string.accounts_choose_server),
                style = MaterialTheme.typography.headlineMedium,
                color = Tokens.Palette.text,
                modifier = Modifier.semantics { heading() },
            )
            val colors = ListItemDefaults.colors(containerColor = Color.Transparent)
            servers.forEach { server ->
                ListItem(
                    colors = colors,
                    modifier = Modifier.clickable(role = Role.Button) { onPick(server) },
                    leadingContent = { androidx.compose.material3.Icon(CouchverseIcons.Server, contentDescription = null) },
                    headlineContent = { Text(server.name) },
                    supportingContent = {
                        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                            Text(server.url, color = Tokens.Palette.muted)
                            if (server.insecure) InsecureBadge()
                        }
                    },
                )
            }
            ListItem(
                colors = colors,
                modifier = Modifier.clickable(role = Role.Button, onClick = onAddServer),
                headlineContent = { Text(stringResource(R.string.servers_add_title), color = LocalAccent.current.ink) },
            )
        }
    }
}

@Composable
fun ChooseServerRoute(onPick: (Server) -> Unit, onAddServer: () -> Unit) {
    val view by rememberSurface<ServersView>(Surface.Servers)
    ChooseServerScreen(view, onPick, onAddServer)
}
