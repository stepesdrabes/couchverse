package io.stepes.couchverse.accounts.tv

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.focusRestorer
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Border
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Glow
import androidx.tv.material3.Icon
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import io.stepes.couchverse.accounts.accountTint
import io.stepes.couchverse.core.AccountCard
import io.stepes.couchverse.core.AccountsView
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.components.GlowBackdrop
import io.stepes.couchverse.design.theme.LocalAccent
import io.stepes.couchverse.design.theme.LocalReducedMotion
import io.stepes.couchverse.design.theme.Motion
import io.stepes.couchverse.design.theme.sharedAvatar
import io.stepes.couchverse.design.tv.TvSafe
import kotlinx.coroutines.delay

private val TileSize = 148.dp

/**
 * The TV's showpiece (plan 12.3): profiles as large circular tiles over the drifting glow, which
 * takes the colour of the focused one. Focus lifts a tile, brightens its ring and reveals where
 * the account lives; picking one sends its avatar into the sidebar while the others drift apart.
 */
@OptIn(ExperimentalComposeUiApi::class)
@Composable
internal fun WhosWatchingTv(view: AccountsView?, onPick: (AccountCard) -> Unit, onAdd: () -> Unit) {
    val accounts = view?.accounts.orEmpty()
    var focusedId by remember { mutableStateOf<String?>(null) }
    var chosen by remember { mutableStateOf<String?>(null) }
    val defaultTint = LocalAccent.current.accent
    val tint by animateColorAsState(
        accounts.firstOrNull { it.id == focusedId }?.let(::accountTint) ?: defaultTint,
        Motion.ambient(900),
        label = "tint",
    )
    val departure by animateFloatAsState(if (chosen == null) 0f else 1f, Motion.smooth(), label = "departure")
    val chosenIndex = accounts.indexOfFirst { it.id == chosen }
    val start = remember { FocusRequester() }
    val startIndex = accounts.indexOfFirst { it.id == view?.active }.coerceAtLeast(0)
    LaunchedEffect(accounts.isEmpty()) { if (accounts.isNotEmpty()) start.requestFocus() }

    Box(Modifier.fillMaxSize()) {
        GlowBackdrop(accent = tint)
        Column(
            Modifier.fillMaxSize().padding(vertical = TvSafe.vertical),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
        ) {
            Text(
                stringResource(R.string.accounts_whos_watching),
                style = MaterialTheme.typography.displaySmall,
                color = Tokens.Palette.text,
                modifier = Modifier.alpha(1f - departure).semantics { heading() },
            )
            Spacer(Modifier.height(48.dp))
            LazyRow(
                modifier = Modifier.fillMaxWidth().focusRestorer(start),
                horizontalArrangement = Arrangement.spacedBy(40.dp, Alignment.CenterHorizontally),
                contentPadding = PaddingValues(horizontal = TvSafe.horizontal, vertical = 28.dp),
            ) {
                itemsIndexed(accounts, key = { _, account -> account.id }) { index, account ->
                    val leaving = if (chosenIndex >= 0 && index != chosenIndex) departure else 0f
                    ProfileTile(
                        account = account,
                        appearDelayMillis = index * 70L,
                        leaving = leaving,
                        direction = if (index < chosenIndex) -1f else 1f,
                        onFocused = { focusedId = account.id },
                        onClick = {
                            if (account.signedIn) chosen = account.id
                            onPick(account)
                        },
                        modifier = if (index == startIndex) Modifier.focusRequester(start) else Modifier,
                    )
                }
                item(key = "add") {
                    AddTile(
                        appearDelayMillis = accounts.size * 70L,
                        leaving = if (chosenIndex >= 0) departure else 0f,
                        onFocused = { focusedId = null },
                        onClick = onAdd,
                    )
                }
            }
        }
    }
}

@Composable
private fun ProfileTile(
    account: AccountCard,
    appearDelayMillis: Long,
    leaving: Float,
    direction: Float,
    onFocused: () -> Unit,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val tint = accountTint(account)
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val ring by animateColorAsState(if (focused) tint else tint.copy(alpha = 0.4f), Motion.standard(), label = "ring")
    val ringWidth by animateDpAsState(if (focused) 4.dp else 2.dp, Motion.snappy(), label = "ring")
    val reduced = LocalReducedMotion.current
    val label = listOfNotNull(
        account.displayName,
        account.serverName,
        stringResource(R.string.accounts_sign_in_again).takeIf { !account.signedIn },
    ).joinToString(", ")
    TileColumn(appearDelayMillis, leaving, direction, modifier) {
        Surface(
            onClick = onClick,
            interactionSource = interaction,
            shape = ClickableSurfaceDefaults.shape(CircleShape),
            scale = ClickableSurfaceDefaults.scale(focusedScale = if (reduced) 1f else 1.12f),
            glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(tint.copy(alpha = 0.55f), 28.dp)),
            border = ClickableSurfaceDefaults.border(
                border = Border(BorderStroke(ringWidth, ring), shape = CircleShape),
                focusedBorder = Border(BorderStroke(ringWidth, ring), shape = CircleShape),
            ),
            colors = ClickableSurfaceDefaults.colors(
                containerColor = Tokens.Palette.surface2.copy(alpha = 0.55f),
                focusedContainerColor = Tokens.Palette.surface2.copy(alpha = 0.85f),
            ),
            modifier = Modifier
                .size(TileSize)
                .onFocusChanged { if (it.isFocused) onFocused() }
                .semantics { contentDescription = label },
        ) {
            Avatar(
                account.avatarUrl,
                seed = account.username,
                size = TileSize - 18.dp,
                modifier = Modifier.align(Alignment.Center).sharedAvatar(account.id),
            )
        }
        Text(
            account.displayName,
            style = MaterialTheme.typography.titleMedium,
            color = if (focused) Tokens.Palette.text else Tokens.Palette.muted,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        AnimatedVisibility(
            visible = focused,
            enter = fadeIn(Motion.standard()) + expandVertically(Motion.snappy()),
            exit = fadeOut(Motion.standard()) + shrinkVertically(Motion.snappy()),
        ) {
            Text(
                if (account.signedIn) account.serverName else stringResource(R.string.accounts_sign_in_again),
                style = MaterialTheme.typography.bodySmall,
                color = if (account.signedIn) tint else Tokens.Palette.danger,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
private fun AddTile(appearDelayMillis: Long, leaving: Float, onFocused: () -> Unit, onClick: () -> Unit) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val label = stringResource(R.string.accounts_add)
    TileColumn(appearDelayMillis, leaving, direction = 1f) {
        Surface(
            onClick = onClick,
            interactionSource = interaction,
            shape = ClickableSurfaceDefaults.shape(CircleShape),
            scale = ClickableSurfaceDefaults.scale(focusedScale = if (LocalReducedMotion.current) 1f else 1.12f),
            border = ClickableSurfaceDefaults.border(
                border = Border(BorderStroke(2.dp, Tokens.Palette.edge), shape = CircleShape),
                focusedBorder = Border(BorderStroke(4.dp, Tokens.Palette.text), shape = CircleShape),
            ),
            colors = ClickableSurfaceDefaults.colors(
                containerColor = Color.Transparent,
                focusedContainerColor = Tokens.Palette.surface2.copy(alpha = 0.7f),
            ),
            modifier = Modifier
                .size(TileSize)
                .onFocusChanged { if (it.isFocused) onFocused() }
                .semantics { contentDescription = label },
        ) {
            Icon(
                Icons.Filled.Add,
                contentDescription = null,
                tint = if (focused) Tokens.Palette.text else Tokens.Palette.muted,
                modifier = Modifier.align(Alignment.Center).size(48.dp),
            )
        }
        Text(label, style = MaterialTheme.typography.titleMedium, color = if (focused) Tokens.Palette.text else Tokens.Palette.muted)
    }
}

/**
 * A tile's column: it rises into place a beat after the one before it, and when another tile
 * is picked it drifts away from it and dissolves.
 */
@Composable
private fun TileColumn(
    appearDelayMillis: Long,
    leaving: Float,
    direction: Float,
    modifier: Modifier = Modifier,
    content: @Composable androidx.compose.foundation.layout.ColumnScope.() -> Unit,
) {
    val reduced = LocalReducedMotion.current
    val appear = remember { Animatable(if (reduced) 1f else 0f) }
    val spec = Motion.smooth<Float>()
    LaunchedEffect(Unit) {
        delay(appearDelayMillis)
        appear.animateTo(1f, spec)
    }
    Column(
        modifier
            .width(TileSize + 28.dp)
            .graphicsLayer {
                val shown = appear.value
                alpha = shown * (1f - leaving)
                translationY = (1f - shown) * 24.dp.toPx()
                translationX = direction * leaving * 140.dp.toPx()
                val scale = (0.94f + 0.06f * shown) * (1f + 0.12f * leaving)
                scaleX = scale
                scaleY = scale
            },
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(14.dp),
        content = content,
    )
}
