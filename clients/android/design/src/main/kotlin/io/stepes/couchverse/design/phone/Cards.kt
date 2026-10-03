package io.stepes.couchverse.design.phone

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Artwork
import io.stepes.couchverse.design.components.WatchProgress

private val CardShape = RoundedCornerShape(Tokens.Radius.card)

/** A title as a 2:3 poster with its name and year below; one tap target for all of it. */
@Composable
fun PosterCard(
    name: String,
    posterUrl: String?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    accent: String? = null,
    caption: String? = null,
    width: Dp? = 128.dp,
) {
    Column(
        modifier
            .then(if (width != null) Modifier.width(width) else Modifier.fillMaxWidth())
            .clip(CardShape)
            .clickable(role = Role.Button, onClick = onClick)
            .semantics(mergeDescendants = true) {},
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Artwork(
            url = posterUrl,
            accent = accent,
            fallbackName = name,
            modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(CardShape),
        )
        Column(Modifier.padding(horizontal = 2.dp, vertical = 2.dp)) {
            Text(
                name,
                style = MaterialTheme.typography.titleSmall,
                color = Tokens.Palette.text,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (caption != null) {
                Text(caption, style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.muted, maxLines = 1)
            }
        }
    }
}

/**
 * A landscape card (continue watching, an episode): the backdrop with the progress along its
 * bottom edge and the name below. [progressLabel] is what accessibility services read for the bar.
 */
@Composable
fun BackdropCard(
    name: String,
    imageUrl: String?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    accent: String? = null,
    caption: String? = null,
    progress: Float? = null,
    progressLabel: String? = null,
    width: Dp? = 260.dp,
) {
    Column(
        modifier
            .then(if (width != null) Modifier.width(width) else Modifier.fillMaxWidth())
            .clip(CardShape)
            .clickable(role = Role.Button, onClick = onClick)
            .semantics(mergeDescendants = true) {
                if (progressLabel != null) contentDescription = listOfNotNull(name, caption, progressLabel).joinToString(", ")
            },
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Box(Modifier.fillMaxWidth().aspectRatio(16f / 9f).clip(CardShape)) {
            Artwork(url = imageUrl, accent = accent, fallbackName = name, modifier = Modifier.matchParentSize())
            if (progress != null) {
                WatchProgress(
                    progress,
                    Modifier.align(Alignment.BottomCenter).padding(horizontal = 10.dp, vertical = 8.dp),
                )
            }
        }
        Column(Modifier.padding(horizontal = 2.dp, vertical = 2.dp)) {
            Text(name, style = MaterialTheme.typography.titleSmall, color = Tokens.Palette.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (caption != null) {
                Text(caption, style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.muted, maxLines = 1)
            }
        }
    }
}
