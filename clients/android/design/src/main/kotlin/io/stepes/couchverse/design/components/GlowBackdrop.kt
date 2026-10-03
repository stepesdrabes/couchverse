package io.stepes.couchverse.design.components

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.State
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.theme.LocalAccent
import io.stepes.couchverse.design.theme.LocalReducedMotion
import kotlin.math.cos
import kotlin.math.sin

/**
 * The Couchverse canvas: soft orbs of the accent and the two glow colours over the background,
 * drifting slowly. Radial gradients instead of a blur, which TV GPUs struggle with.
 */
@Composable
fun GlowBackdrop(modifier: Modifier = Modifier, accent: Color = LocalAccent.current.accent, intensity: Float = 1f) {
    val phase = drift()
    Canvas(modifier.fillMaxSize()) {
        drawRect(Tokens.Palette.bg)
        val t = phase.value * 2 * Math.PI
        val reach = size.maxDimension
        fun orb(color: Color, x: Float, y: Float, radius: Float, alpha: Float) {
            val center = Offset(size.width * x, size.height * y)
            drawCircle(
                Brush.radialGradient(
                    listOf(color.copy(alpha = alpha * intensity), Color.Transparent),
                    center = center,
                    radius = reach * radius,
                ),
                radius = reach * radius,
                center = center,
            )
        }
        orb(accent, 0.18f + 0.06f * cos(t).toFloat(), 0.2f + 0.05f * sin(t).toFloat(), 0.55f, 0.42f)
        orb(Tokens.Palette.glowViolet, 0.85f + 0.05f * sin(t).toFloat(), 0.3f, 0.45f, 0.22f)
        orb(Tokens.Palette.glowTeal, 0.6f, 0.95f + 0.04f * cos(t).toFloat(), 0.5f, 0.14f)
    }
}

/** A slow loop from 0 to 1, or a still 0 when the user prefers reduced motion. */
@Composable
private fun drift(): State<Float> {
    if (LocalReducedMotion.current) return remember { mutableFloatStateOf(0f) }
    val transition = rememberInfiniteTransition(label = "glow")
    return transition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(24_000, easing = LinearEasing), RepeatMode.Restart),
        label = "glow",
    )
}
