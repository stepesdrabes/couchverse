package io.stepes.couchverse.design.tv

import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.Icon
import androidx.tv.material3.Text
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.theme.LocalAccent
import io.stepes.couchverse.design.theme.LocalReducedMotion

/**
 * A TV button. The [primary] one (Play, Sign in) fills with the accent when focused; the others
 * turn white, as Compose for TV buttons do.
 */
@Composable
fun TvActionButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    icon: ImageVector? = null,
    primary: Boolean = false,
    enabled: Boolean = true,
) {
    val accent = LocalAccent.current
    val colors = if (primary) {
        ButtonDefaults.colors(
            containerColor = accent.accent.copy(alpha = 0.85f),
            contentColor = accent.onAccent,
            focusedContainerColor = accent.accent,
            focusedContentColor = accent.onAccent,
        )
    } else {
        ButtonDefaults.colors(
            containerColor = Tokens.Palette.surface2.copy(alpha = 0.85f),
            contentColor = Tokens.Palette.text,
        )
    }
    Button(
        onClick = onClick,
        modifier = modifier,
        enabled = enabled,
        colors = colors,
        scale = ButtonDefaults.scale(focusedScale = if (LocalReducedMotion.current) 1f else 1.08f),
    ) {
        if (icon != null) {
            Icon(icon, contentDescription = null, modifier = Modifier.size(20.dp))
            Spacer(Modifier.width(8.dp))
        }
        Text(text)
    }
}
