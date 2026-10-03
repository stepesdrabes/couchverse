package io.stepes.couchverse.design.components

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.theme.colorOf

/**
 * A piece of artwork from a ready URL (the core signs it with the artwork grant). Until it loads,
 * and when there is none, the space shows the artwork's accent as a soft gradient; with
 * [fallbackName] the name is written on it, so a card without a poster still says what it is.
 */
@Composable
fun Artwork(
    url: String?,
    modifier: Modifier = Modifier,
    accent: String? = null,
    contentDescription: String? = null,
    fallbackName: String? = null,
    contentScale: ContentScale = ContentScale.Crop,
) {
    var loaded by remember(url) { mutableStateOf(false) }
    val tint = colorOf(accent) ?: Tokens.Palette.surface2
    Box(
        modifier.background(
            Brush.linearGradient(listOf(tint.copy(alpha = 0.55f), Tokens.Palette.surface)),
        ),
    ) {
        if (!loaded && fallbackName != null) {
            Text(
                text = fallbackName,
                style = MaterialTheme.typography.titleSmall,
                color = Tokens.Palette.text.copy(alpha = 0.8f),
                textAlign = TextAlign.Center,
                maxLines = 3,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.align(Alignment.Center).padding(12.dp),
            )
        }
        if (url != null) {
            AsyncImage(
                model = url,
                contentDescription = contentDescription,
                contentScale = contentScale,
                onSuccess = { loaded = true },
                modifier = Modifier.fillMaxSize(),
            )
        }
    }
}

/** A bottom-up fade into the canvas, so text stays readable over any artwork. */
fun scrim(from: Color = Tokens.Palette.bg, strength: Float = 1f): Brush =
    Brush.verticalGradient(
        0f to Color.Transparent,
        0.55f to from.copy(alpha = 0.45f * strength),
        1f to from.copy(alpha = strength),
    )
