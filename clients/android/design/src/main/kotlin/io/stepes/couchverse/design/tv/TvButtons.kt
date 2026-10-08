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
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.FilterChip
import androidx.tv.material3.FilterChipDefaults
import androidx.tv.material3.Icon
import androidx.tv.material3.MaterialTheme
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

/**
 * A TV choice among a few (a sort, a genre, a board). Focused, a chosen chip turns white like
 * any focused one: Compose for TV would fill it with `onPrimaryContainer` and write on it in
 * `onPrimary`, which are both the accent's text colour here.
 */
@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun TvFilterChip(label: String, selected: Boolean, onClick: () -> Unit, modifier: Modifier = Modifier) {
    FilterChip(
        selected = selected,
        onClick = onClick,
        modifier = modifier,
        colors = FilterChipDefaults.colors(
            focusedSelectedContainerColor = MaterialTheme.colorScheme.onSurface,
            focusedSelectedContentColor = MaterialTheme.colorScheme.inverseOnSurface,
        ),
    ) { Text(label) }
}
