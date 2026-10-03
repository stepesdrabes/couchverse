package io.stepes.couchverse.design.theme

import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import io.stepes.couchverse.core.AccentPalette
import io.stepes.couchverse.design.Tokens

/** The accent colours of whatever is in charge of the screen: the server, or a title's artwork. */
@Immutable
data class AccentColors(
    val accent: Color,
    val strong: Color,
    val soft: Color,
    val onAccent: Color,
) {
    companion object {
        val Default = AccentColors(
            accent = Tokens.Palette.accent,
            strong = Tokens.Palette.accentStrong,
            soft = Tokens.Palette.accentSoft,
            onAccent = Tokens.Palette.onAccent,
        )

        /** The core's palette; it already derived every colour from one accent. */
        fun of(palette: AccentPalette?): AccentColors = palette?.let {
            AccentColors(
                accent = colorOf(it.accent) ?: Default.accent,
                strong = colorOf(it.strong) ?: Default.strong,
                soft = colorOf(it.soft) ?: Default.soft,
                onAccent = colorOf(it.onAccent) ?: Default.onAccent,
            )
        } ?: Default
    }
}

val LocalAccent = staticCompositionLocalOf { AccentColors.Default }

/** `#rrggbb` or `#rrggbbaa`, as the core and the API write colours. */
fun colorOf(hex: String?): Color? {
    val digits = hex?.trim()?.removePrefix("#") ?: return null
    val value = digits.toLongOrNull(16) ?: return null
    return when (digits.length) {
        6 -> Color(0xFF000000 or value)
        8 -> Color(((value and 0xFF) shl 24) or (value ushr 8))
        else -> null
    }
}
