package io.stepes.couchverse.catalog.tv

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.focusRestorer
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import io.stepes.couchverse.core.GenreView
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Identicon
import io.stepes.couchverse.design.theme.LocalReducedMotion
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.rememberScreenFocus
import io.stepes.couchverse.design.tv.screenFocus

/** A TV screen's title, inside the title-safe area. */
@Composable
internal fun TvHeader(title: String) {
    Text(
        title,
        style = MaterialTheme.typography.headlineMedium,
        color = Tokens.Palette.text,
        modifier = Modifier
            .padding(start = TvSafe.horizontal, end = TvSafe.horizontal, top = TvSafe.vertical, bottom = 8.dp)
            .semantics { heading() },
    )
}

@OptIn(ExperimentalComposeUiApi::class)
@Composable
internal fun GenreGridTv(genres: List<GenreView>, onOpen: (GenreView) -> Unit) {
    val focus = rememberScreenFocus()
    LazyVerticalGrid(
        columns = GridCells.Adaptive(200.dp),
        contentPadding = PaddingValues(horizontal = TvSafe.horizontal, vertical = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(20.dp),
        verticalArrangement = Arrangement.spacedBy(20.dp),
        modifier = Modifier.fillMaxSize().focusRestorer(),
    ) {
        itemsIndexed(genres, key = { _, genre -> genre.name }) { index, genre ->
            val tint = Identicon.colorOf(genre.name)
            val shape = RoundedCornerShape(12.dp)
            Surface(
                onClick = { onOpen(genre) },
                shape = ClickableSurfaceDefaults.shape(shape),
                scale = ClickableSurfaceDefaults.scale(focusedScale = if (LocalReducedMotion.current) 1f else 1.06f),
                modifier = Modifier.fillMaxWidth().aspectRatio(16f / 9f).screenFocus(focus, genre.name, start = index == 0),
            ) {
                Box(
                    Modifier.fillMaxSize().background(Brush.linearGradient(listOf(tint.copy(alpha = 0.85f), Tokens.Palette.surface2))),
                    contentAlignment = Alignment.BottomStart,
                ) {
                    Text(genre.label, style = MaterialTheme.typography.titleMedium, color = Tokens.Palette.text, modifier = Modifier.padding(14.dp))
                }
            }
        }
    }
}
