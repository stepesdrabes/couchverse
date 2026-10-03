package io.stepes.couchverse.design.components

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.theme.LocalAccent

/** A small rounded label: a content rating, a quality, "Not encrypted". */
@Composable
fun Pill(text: String, modifier: Modifier = Modifier, color: Color = Tokens.Palette.muted, filled: Boolean = false) {
    val shape = RoundedCornerShape(6.dp)
    Text(
        text = text,
        style = MaterialTheme.typography.labelMedium,
        color = if (filled) Tokens.Palette.bg else color,
        maxLines = 1,
        modifier = modifier
            .clip(shape)
            .then(if (filled) Modifier.background(color) else Modifier.border(1.dp, color.copy(alpha = 0.5f), shape))
            .padding(horizontal = 8.dp, vertical = 2.dp),
    )
}

/** Marks a server reached over plain http (D14). */
@Composable
fun InsecureBadge(modifier: Modifier = Modifier) {
    val label = stringResource(R.string.servers_not_encrypted)
    Row(
        modifier
            .clip(CircleShape)
            .background(Tokens.Palette.danger.copy(alpha = 0.14f))
            .padding(horizontal = 10.dp, vertical = 4.dp)
            .clearAndSetSemantics { contentDescription = label },
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(Icons.Filled.Warning, contentDescription = null, tint = Tokens.Palette.danger, modifier = Modifier.size(14.dp))
        Text(label, style = MaterialTheme.typography.labelMedium, color = Tokens.Palette.danger)
    }
}

/** How far into a title or episode the viewer is, as a thin accent bar. */
@Composable
fun WatchProgress(progress: Float, modifier: Modifier = Modifier, height: Dp = 3.dp) {
    Box(modifier.fillMaxWidth().height(height).clip(CircleShape).background(Color.White.copy(alpha = 0.22f))) {
        Box(
            Modifier
                .fillMaxWidth(progress.coerceIn(0f, 1f))
                .fillMaxHeight()
                .background(LocalAccent.current.accent),
        )
    }
}

/**
 * A title's wordmark, sized from its aspect ratio before it loads; the name in [nameStyle] stands
 * in when there is no logo or it fails to load.
 */
@Composable
fun TitleLogo(
    name: String,
    logoUrl: String?,
    aspect: Double?,
    nameStyle: TextStyle,
    modifier: Modifier = Modifier,
    maxWidth: Dp = 320.dp,
    maxHeight: Dp = 120.dp,
) {
    var failed by remember(logoUrl) { mutableStateOf(false) }
    if (logoUrl == null || failed) {
        Text(
            name,
            style = nameStyle,
            color = Tokens.Palette.text,
            maxLines = 3,
            overflow = TextOverflow.Ellipsis,
            modifier = modifier.semantics { heading() },
        )
        return
    }
    val ratio = (aspect ?: DEFAULT_LOGO_ASPECT).toFloat().coerceIn(0.5f, 8f)
    val width = minOf(maxWidth, maxHeight * ratio)
    AsyncImage(
        model = logoUrl,
        contentDescription = name,
        onError = { failed = true },
        alignment = Alignment.BottomStart,
        contentScale = ContentScale.Fit,
        modifier = modifier.size(width, width / ratio).semantics { heading() },
    )
}

/** The Couchverse mark: the logo in the contrast colour on an accent tile. */
@Composable
fun LogoMark(modifier: Modifier = Modifier, size: Dp = 40.dp) {
    val accent = LocalAccent.current
    Box(
        modifier.size(size).clip(RoundedCornerShape(size * 0.25f)).background(accent.accent),
        contentAlignment = Alignment.Center,
    ) {
        Image(
            painter = painterResource(R.drawable.couchverse_logo),
            contentDescription = null,
            colorFilter = ColorFilter.tint(accent.onAccent),
            modifier = Modifier.fillMaxWidth(0.8f),
        )
    }
}

/**
 * The shared shape of an empty or failed screen: a title, an explanation and an optional action
 * (the caller's idiom-appropriate button).
 */
@Composable
fun StatusMessage(
    title: String,
    modifier: Modifier = Modifier,
    message: String? = null,
    action: (@Composable () -> Unit)? = null,
) {
    Column(
        modifier.padding(24.dp).widthIn(max = 480.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text(
            title,
            style = MaterialTheme.typography.titleLarge,
            color = Tokens.Palette.text,
            textAlign = TextAlign.Center,
            modifier = Modifier.semantics { heading() },
        )
        if (message != null) {
            Text(message, style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.muted, textAlign = TextAlign.Center)
        }
        if (action != null) {
            Box(Modifier.padding(top = 8.dp)) { action() }
        }
    }
}

/** Logos without a known shape are assumed to be a wide wordmark. */
private const val DEFAULT_LOGO_ASPECT = 3.0
