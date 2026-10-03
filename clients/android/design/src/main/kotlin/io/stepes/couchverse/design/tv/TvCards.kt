package io.stepes.couchverse.design.tv

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Border
import androidx.tv.material3.Card
import androidx.tv.material3.CardBorder
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.CardGlow
import androidx.tv.material3.CardScale
import androidx.tv.material3.Glow
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.StandardCardContainer
import androidx.tv.material3.Text
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Artwork
import io.stepes.couchverse.design.components.WatchProgress
import io.stepes.couchverse.design.theme.LocalAccent
import io.stepes.couchverse.design.theme.LocalReducedMotion

private val TvCardShape = RoundedCornerShape(10.dp)

private class FocusLook(val scale: CardScale, val border: CardBorder, val glow: CardGlow)

/** Focus lifts a card, rings it and glows in the accent (plan 12.4): the TV's only pointer. */
@Composable
private fun focusLook(): FocusLook = FocusLook(
    scale = CardDefaults.scale(focusedScale = if (LocalReducedMotion.current) 1f else 1.08f),
    border = CardDefaults.border(focusedBorder = Border(BorderStroke(3.dp, Tokens.Palette.text), shape = TvCardShape)),
    glow = CardDefaults.glow(focusedGlow = Glow(LocalAccent.current.accent.copy(alpha = 0.5f), 18.dp)),
)

/** A title on the TV: a focusable 2:3 poster with the name below, bright while focused. */
@Composable
fun TvPosterCard(
    name: String,
    posterUrl: String?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    accent: String? = null,
    caption: String? = null,
    width: Dp = 136.dp,
) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val look = focusLook()
    StandardCardContainer(
        modifier = modifier.width(width),
        interactionSource = interaction,
        imageCard = { source ->
            Card(
                onClick = onClick,
                interactionSource = source,
                shape = CardDefaults.shape(TvCardShape),
                scale = look.scale,
                border = look.border,
                glow = look.glow,
                colors = CardDefaults.colors(containerColor = Tokens.Palette.surface),
                modifier = Modifier.semantics { contentDescription = listOfNotNull(name, caption).joinToString(", ") },
            ) {
                Artwork(posterUrl, accent = accent, fallbackName = name, modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f))
            }
        },
        title = {
            Text(
                name,
                style = MaterialTheme.typography.titleSmall,
                color = if (focused) Tokens.Palette.text else Tokens.Palette.muted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.padding(top = 8.dp),
            )
        },
        subtitle = {
            if (caption != null) {
                Text(caption, style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.faint, maxLines = 1)
            }
        },
    )
}

/** A landscape card on the TV (continue watching, an episode) with progress along its bottom. */
@Composable
fun TvBackdropCard(
    name: String,
    imageUrl: String?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    accent: String? = null,
    caption: String? = null,
    progress: Float? = null,
    progressLabel: String? = null,
    width: Dp = 248.dp,
) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val look = focusLook()
    StandardCardContainer(
        modifier = modifier.width(width),
        interactionSource = interaction,
        imageCard = { source ->
            Card(
                onClick = onClick,
                interactionSource = source,
                shape = CardDefaults.shape(TvCardShape),
                scale = look.scale,
                border = look.border,
                glow = look.glow,
                colors = CardDefaults.colors(containerColor = Tokens.Palette.surface),
                modifier = Modifier.semantics {
                    contentDescription = listOfNotNull(name, caption, progressLabel).joinToString(", ")
                },
            ) {
                Box(Modifier.fillMaxWidth().aspectRatio(16f / 9f)) {
                    Artwork(imageUrl, accent = accent, fallbackName = name, modifier = Modifier.matchParentSize())
                    if (progress != null) {
                        WatchProgress(progress, Modifier.align(Alignment.BottomCenter).padding(10.dp))
                    }
                }
            }
        },
        title = {
            Text(
                name,
                style = MaterialTheme.typography.titleSmall,
                color = if (focused) Tokens.Palette.text else Tokens.Palette.muted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.padding(top = 8.dp),
            )
        },
        subtitle = {
            if (caption != null) {
                Text(caption, style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.faint, maxLines = 1)
            }
        },
    )
}
