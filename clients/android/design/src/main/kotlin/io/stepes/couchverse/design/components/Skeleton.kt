package io.stepes.couchverse.design.components

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.res.stringResource
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.theme.LocalReducedMotion

/** A placeholder block in a skeleton screen, mirroring where content will appear. */
@Composable
fun SkeletonBox(modifier: Modifier = Modifier, shape: Shape = RoundedCornerShape(Tokens.Radius.card)) {
    Box(modifier.clip(shape).background(Tokens.Palette.surface).shimmer())
}

/** Marks a skeleton screen for accessibility services as loading, once, at its root. */
@Composable
fun Modifier.loadingSemantics(): Modifier {
    val loading = stringResource(R.string.common_loading)
    return semantics { contentDescription = loading }
}

/** A light band sweeping across, or a still surface when the user prefers reduced motion. */
fun Modifier.shimmer(): Modifier = composed {
    if (LocalReducedMotion.current) return@composed this
    val transition = rememberInfiniteTransition(label = "shimmer")
    val progress by transition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(1400, easing = LinearEasing), RepeatMode.Restart),
        label = "shimmer",
    )
    drawWithContent {
        drawContent()
        val band = size.width * 0.6f
        val start = -band + (size.width + band * 2) * progress
        drawRect(
            Brush.linearGradient(
                listOf(
                    Tokens.Palette.surface2.copy(alpha = 0f),
                    Tokens.Palette.surface2,
                    Tokens.Palette.surface2.copy(alpha = 0f),
                ),
                start = Offset(start, 0f),
                end = Offset(start + band, size.height),
            ),
        )
    }
}
