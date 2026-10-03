package io.stepes.couchverse.design.theme

import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.em
import io.stepes.couchverse.design.Tokens
import androidx.compose.material3.Typography as PhoneTypography
import androidx.tv.material3.Typography as TvTypography

/** Applies a token role's weight and tracking on top of the platform's scalable size. */
private fun TextStyle.with(role: Tokens.TypeRole) =
    copy(fontWeight = FontWeight(role.weight), letterSpacing = role.trackingEm.em)

/** The Material 3 phone type scale with the Couchverse roles (sizes stay the system's, in sp). */
internal fun phoneTypography(): PhoneTypography {
    val base = PhoneTypography()
    val ramp = Tokens.TypeRamp
    return base.copy(
        displaySmall = base.displaySmall.with(ramp.hero),
        headlineMedium = base.headlineMedium.with(ramp.title),
        titleLarge = base.titleLarge.with(ramp.section),
        titleSmall = base.titleSmall.with(ramp.card),
        bodyLarge = base.bodyLarge.with(ramp.body),
        bodySmall = base.bodySmall.with(ramp.caption),
        labelSmall = base.labelSmall.with(ramp.eyebrow),
    )
}

/** The Compose for TV type scale with the same roles. */
internal fun tvTypography(): TvTypography {
    val base = TvTypography()
    val ramp = Tokens.TypeRamp
    return base.copy(
        displaySmall = base.displaySmall.with(ramp.hero),
        headlineMedium = base.headlineMedium.with(ramp.title),
        titleLarge = base.titleLarge.with(ramp.section),
        titleSmall = base.titleSmall.with(ramp.card),
        bodyLarge = base.bodyLarge.with(ramp.body),
        bodySmall = base.bodySmall.with(ramp.caption),
        labelSmall = base.labelSmall.with(ramp.eyebrow),
    )
}
