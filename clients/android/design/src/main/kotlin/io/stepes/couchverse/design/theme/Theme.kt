package io.stepes.couchverse.design.theme

import androidx.compose.animation.animateColorAsState
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import io.stepes.couchverse.design.Tokens
import androidx.compose.material3.darkColorScheme as phoneDarkColorScheme
import androidx.tv.material3.MaterialTheme as TvMaterialTheme
import androidx.tv.material3.darkColorScheme as tvDarkColorScheme

/** Whether the TV UI is showing; chosen once at launch from the UI mode. */
val LocalIsTv = staticCompositionLocalOf { false }

/**
 * The Couchverse look for both idioms: the dark canvas from the design tokens, tinted by
 * [accent] (the server's palette, or a title's). Phones get Material 3; the TV gets Compose for
 * TV's theme as well, since its focusable components read their own colour scheme.
 */
@Composable
fun CouchverseTheme(
    accent: AccentColors = AccentColors.Default,
    tv: Boolean = LocalIsTv.current,
    reducedMotion: Boolean? = null,
    content: @Composable () -> Unit,
) {
    val context = LocalContext.current
    val prefersReducedMotion = reducedMotion ?: remember(context) { systemPrefersReducedMotion(context) }
    CompositionLocalProvider(LocalReducedMotion provides prefersReducedMotion, LocalIsTv provides tv) {
        val animated = AccentColors(
            accent = animateAccent(accent.accent),
            strong = animateAccent(accent.strong),
            soft = animateAccent(accent.soft),
            onAccent = animateAccent(accent.onAccent),
        )
        val phoneTypography = remember { phoneTypography() }
        CompositionLocalProvider(LocalAccent provides animated) {
            MaterialTheme(colorScheme = phoneColors(animated), typography = phoneTypography) {
                if (tv) {
                    val tvTypography = remember { tvTypography() }
                    TvMaterialTheme(colorScheme = tvColors(animated), typography = tvTypography, content = content)
                } else {
                    content()
                }
            }
        }
    }
}

/** Re-themes part of a screen with another accent, e.g. a title page with its artwork's colour. */
@Composable
fun AccentScope(accent: AccentColors?, content: @Composable () -> Unit) {
    if (accent == null) {
        content()
    } else {
        CouchverseTheme(accent = accent, content = content)
    }
}

@Composable
private fun animateAccent(target: Color): Color {
    val color by animateColorAsState(target, Motion.ambient(), label = "accent")
    return color
}

private fun phoneColors(accent: AccentColors) = with(Tokens.Palette) {
    phoneDarkColorScheme(
        primary = accent.accent,
        onPrimary = accent.onAccent,
        primaryContainer = accent.strong,
        onPrimaryContainer = accent.onAccent,
        inversePrimary = accent.strong,
        secondary = accent.accent,
        onSecondary = accent.onAccent,
        secondaryContainer = surface2,
        onSecondaryContainer = text,
        tertiary = glowTeal,
        onTertiary = bg,
        background = bg,
        onBackground = text,
        surface = bg,
        onSurface = text,
        surfaceVariant = surface2,
        onSurfaceVariant = muted,
        // a tonal tint in the accent would turn every raised surface red; elevation stays neutral
        surfaceTint = surface,
        inverseSurface = text,
        inverseOnSurface = bg,
        error = danger,
        onError = bg,
        outline = edge,
        outlineVariant = edge,
        scrim = Color.Black,
        surfaceBright = surface2,
        surfaceDim = bg,
        surfaceContainerLowest = bg,
        surfaceContainerLow = surface,
        surfaceContainer = surface,
        surfaceContainerHigh = surface2,
        surfaceContainerHighest = edge,
    )
}

private fun tvColors(accent: AccentColors) = with(Tokens.Palette) {
    tvDarkColorScheme(
        primary = accent.accent,
        onPrimary = accent.onAccent,
        primaryContainer = accent.strong,
        onPrimaryContainer = accent.onAccent,
        inversePrimary = accent.strong,
        secondary = accent.accent,
        onSecondary = accent.onAccent,
        secondaryContainer = surface2,
        onSecondaryContainer = text,
        tertiary = glowTeal,
        onTertiary = bg,
        background = bg,
        onBackground = text,
        surface = surface,
        onSurface = text,
        surfaceVariant = surface2,
        onSurfaceVariant = muted,
        surfaceTint = surface,
        // focused TV components invert onto these: a light chip with dark text
        inverseSurface = text,
        inverseOnSurface = bg,
        error = danger,
        onError = bg,
        border = accent.accent,
        borderVariant = edge,
        scrim = Color.Black,
    )
}
