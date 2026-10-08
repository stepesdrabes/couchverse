package io.stepes.couchverse.accounts.phone

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.accounts.accountTint
import io.stepes.couchverse.core.AccountCard
import io.stepes.couchverse.core.AccountsView
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.components.GlowBackdrop
import io.stepes.couchverse.design.components.InsecureBadge
import io.stepes.couchverse.design.theme.LocalAccent
import io.stepes.couchverse.design.theme.Motion
import io.stepes.couchverse.design.theme.colorOf
import io.stepes.couchverse.ranks.RankRing
import io.stepes.couchverse.ranks.rankLine

/** The phone's "Who's watching?": a lighter take on the TV's, with a haptic tick on the pick. */
@Composable
internal fun AccountPickerPhone(view: AccountsView?, onPick: (AccountCard) -> Unit, onAdd: () -> Unit) {
    val haptics = LocalHapticFeedback.current
    var chosen by remember { mutableStateOf<String?>(null) }
    // the glow takes the colour of the account picked, or of the one used last
    val shown = view?.accounts?.firstOrNull { it.id == (chosen ?: view.active) }
    val tint by animateColorAsState(shown?.let(::accountTint) ?: LocalAccent.current.accent, Motion.ambient(900), label = "tint")
    Box(Modifier.fillMaxSize()) {
        GlowBackdrop(accent = tint)
        LazyVerticalGrid(
            columns = GridCells.Adaptive(132.dp),
            modifier = Modifier.fillMaxSize().safeDrawingPadding(),
            contentPadding = PaddingValues(24.dp),
            horizontalArrangement = Arrangement.spacedBy(16.dp),
            verticalArrangement = Arrangement.spacedBy(24.dp),
        ) {
            item(span = { GridItemSpan(maxLineSpan) }) {
                Text(
                    stringResource(R.string.accounts_whos_watching),
                    style = MaterialTheme.typography.displaySmall,
                    color = Tokens.Palette.text,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.padding(top = 56.dp, bottom = 16.dp).semantics { heading() },
                )
            }
            items(view?.accounts.orEmpty(), key = { it.id }) { account ->
                val faded by animateFloatAsState(
                    if (chosen == null || chosen == account.id) 1f else 0.3f,
                    Motion.standard(),
                    label = "pick",
                )
                ProfileTilePhone(
                    account,
                    onClick = {
                        haptics.performHapticFeedback(HapticFeedbackType.Confirm)
                        if (account.signedIn) chosen = account.id
                        onPick(account)
                    },
                    modifier = Modifier.graphicsLayer { alpha = faded },
                )
            }
            item(key = "add") { AddTilePhone(onAdd) }
        }
    }
}

@Composable
private fun ProfileTilePhone(account: AccountCard, onClick: () -> Unit, modifier: Modifier = Modifier) {
    val tint = accountTint(account)
    Column(
        modifier.clickable(role = Role.Button, onClick = onClick).padding(8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        AccountAvatar(account, 104.dp, ring = 4.dp, unranked = tint.copy(alpha = 0.7f))
        Text(
            account.displayName,
            style = MaterialTheme.typography.titleMedium,
            color = Tokens.Palette.text,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        account.rank?.takeIf { account.signedIn }?.let { rank ->
            Text(
                rankLine(rank),
                style = MaterialTheme.typography.labelMedium,
                color = colorOf(rank.tier.colour) ?: tint,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        Text(
            if (account.signedIn) account.serverName else stringResource(R.string.accounts_sign_in_again),
            style = MaterialTheme.typography.bodySmall,
            color = if (account.signedIn) Tokens.Palette.muted else Tokens.Palette.danger,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun AddTilePhone(onAdd: () -> Unit) {
    Column(
        Modifier.clickable(role = Role.Button, onClick = onAdd).padding(8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Box(Modifier.size(104.dp).border(2.dp, Tokens.Palette.edge, CircleShape), contentAlignment = Alignment.Center) {
            Icon(Icons.Filled.Add, contentDescription = null, tint = Tokens.Palette.muted, modifier = Modifier.size(40.dp))
        }
        Text(stringResource(R.string.accounts_add_account), style = MaterialTheme.typography.titleMedium, color = Tokens.Palette.muted)
    }
}

/** Every account on this phone, the active one checked, plus adding another. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun AccountSwitcherSheet(
    view: AccountsView?,
    onPick: (AccountCard) -> Unit,
    onAdd: () -> Unit,
    onDismiss: () -> Unit,
) {
    val haptics = LocalHapticFeedback.current
    val active = view?.accounts?.firstOrNull { it.id == view.active }
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
        containerColor = Tokens.Palette.surface,
    ) {
        Box {
            // the sheet glows in the colour of the account watching now
            active?.let { GlowBackdrop(Modifier.matchParentSize(), accent = accountTint(it), intensity = 0.6f) }
            AccountList(
                view = view,
                onPick = { account ->
                    haptics.performHapticFeedback(HapticFeedbackType.Confirm)
                    onPick(account)
                },
                onAdd = onAdd,
                modifier = Modifier.navigationBarsPadding().padding(bottom = 16.dp),
            )
        }
    }
}

/** The accounts as rows; shared by the switcher sheet and Settings. */
@Composable
fun AccountList(
    view: AccountsView?,
    onPick: (AccountCard) -> Unit,
    onAdd: () -> Unit,
    modifier: Modifier = Modifier,
    trailing: (@Composable (AccountCard) -> Unit)? = null,
) {
    Column(modifier.fillMaxWidth()) {
        Text(
            stringResource(R.string.accounts_switch),
            style = MaterialTheme.typography.titleLarge,
            color = Tokens.Palette.text,
            modifier = Modifier.padding(horizontal = 24.dp, vertical = 8.dp).semantics { heading() },
        )
        val colors = ListItemDefaults.colors(containerColor = Color.Transparent)
        view?.accounts.orEmpty().forEach { account ->
            val active = account.id == view?.active
            val end: (@Composable () -> Unit)? = when {
                trailing != null -> { { trailing(account) } }
                active -> { { Icon(Icons.Filled.Check, contentDescription = null, tint = MaterialTheme.colorScheme.primary) } }
                else -> null
            }
            ListItem(
                colors = colors,
                modifier = Modifier.clickable(role = Role.Button) { onPick(account) },
                leadingContent = { AccountAvatar(account, 44.dp, ring = 3.dp, unranked = Tokens.Palette.edge) },
                headlineContent = { Text(account.displayName, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                supportingContent = {
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            if (account.signedIn) account.serverName else stringResource(R.string.accounts_sign_in_again),
                            color = if (account.signedIn) Tokens.Palette.muted else Tokens.Palette.danger,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f, fill = false),
                        )
                        if (account.insecure) InsecureBadge()
                    }
                },
                trailingContent = end,
            )
        }
        ListItem(
            colors = colors,
            modifier = Modifier.clickable(role = Role.Button, onClick = onAdd),
            leadingContent = {
                Box(Modifier.size(44.dp).border(1.dp, Tokens.Palette.edge, CircleShape), contentAlignment = Alignment.Center) {
                    Icon(Icons.Filled.Add, contentDescription = null)
                }
            },
            headlineContent = { Text(stringResource(R.string.accounts_add_account)) },
        )
    }
}

/**
 * An account's picture inside its ring: the rank's once a rank is known (the tier's colour,
 * filled to the progress through it), else a plain ring in [unranked].
 */
@Composable
private fun AccountAvatar(account: AccountCard, size: Dp, ring: Dp, unranked: Color) {
    Box(Modifier.size(size), contentAlignment = Alignment.Center) {
        Avatar(account.avatarUrl, seed = account.username, size = size - ring * 4)
        val rank = account.rank
        if (rank != null) {
            RankRing(rank, Modifier.fillMaxSize(), stroke = ring)
        } else {
            Box(Modifier.fillMaxSize().border(ring, unranked, CircleShape))
        }
    }
}
