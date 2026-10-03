package io.stepes.couchverse.catalog.phone

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.GenreView
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Identicon
import io.stepes.couchverse.design.phone.PhoneGutter

/** Genres as colour tiles; the colour is the genre's own, the same every time. */
@Composable
internal fun GenreGridPhone(genres: List<GenreView>, onOpen: (GenreView) -> Unit) {
    LazyVerticalGrid(
        columns = GridCells.Adaptive(160.dp),
        contentPadding = PaddingValues(horizontal = PhoneGutter, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        modifier = Modifier.fillMaxSize(),
    ) {
        items(genres, key = { it.name }) { genre ->
            val tint = Identicon.colorOf(genre.name)
            Box(
                Modifier
                    .fillMaxWidth()
                    .aspectRatio(16f / 9f)
                    .clip(RoundedCornerShape(Tokens.Radius.card))
                    .background(Brush.linearGradient(listOf(tint.copy(alpha = 0.85f), Tokens.Palette.surface2)))
                    .clickable(role = Role.Button) { onOpen(genre) },
                contentAlignment = Alignment.BottomStart,
            ) {
                Text(genre.label, style = MaterialTheme.typography.titleMedium, color = Tokens.Palette.text, modifier = Modifier.padding(14.dp))
            }
        }
    }
}
