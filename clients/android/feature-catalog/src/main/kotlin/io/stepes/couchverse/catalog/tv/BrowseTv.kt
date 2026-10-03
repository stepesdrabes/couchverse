package io.stepes.couchverse.catalog.tv

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.focusRestorer
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.FilterChip
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import io.stepes.couchverse.catalog.BrowseActions
import io.stepes.couchverse.catalog.BrowseState
import io.stepes.couchverse.catalog.CatalogNavigation
import io.stepes.couchverse.catalog.LoadMoreWhenNearEnd
import io.stepes.couchverse.catalog.SortOptions
import io.stepes.couchverse.catalog.caption
import io.stepes.couchverse.catalog.label
import io.stepes.couchverse.core.Card
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.SkeletonBox
import io.stepes.couchverse.design.components.loadingSemantics
import io.stepes.couchverse.design.tv.TvPosterCard
import io.stepes.couchverse.design.tv.TvSafe

private val PosterColumn = 136.dp

@OptIn(ExperimentalComposeUiApi::class, ExperimentalTvMaterial3Api::class)
@Composable
internal fun BrowseTv(state: BrowseState, actions: BrowseActions, content: @Composable () -> Unit) {
    Column(Modifier.fillMaxSize().padding(top = TvSafe.vertical), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Row(
            Modifier.padding(horizontal = TvSafe.horizontal),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                state.title,
                style = MaterialTheme.typography.headlineMedium,
                color = Tokens.Palette.text,
                modifier = Modifier.padding(end = 16.dp).semantics { heading() },
            )
            SortOptions.forEach { sort ->
                FilterChip(selected = state.sort == sort, onClick = { actions.onSort(sort) }) { Text(sort.label()) }
            }
        }
        val genres = state.genres?.genres.orEmpty()
        if (genres.isNotEmpty()) {
            LazyRow(
                modifier = Modifier.focusRestorer(),
                contentPadding = PaddingValues(horizontal = TvSafe.horizontal, vertical = 4.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                item(key = "all") {
                    FilterChip(selected = state.genre == null, onClick = { actions.onGenre(null) }) {
                        Text(stringResource(R.string.catalog_filter_all))
                    }
                }
                items(genres, key = { it.name }) { genre ->
                    FilterChip(
                        selected = state.genre == genre.name,
                        onClick = { actions.onGenre(genre.name.takeIf { it != state.genre }) },
                    ) { Text(genre.label) }
                }
            }
        }
        Box(Modifier.weight(1f)) { content() }
    }
}

/** A focus grid of posters; coming back to it lands on the poster that was focused. */
@OptIn(ExperimentalComposeUiApi::class)
@Composable
internal fun PosterGridTv(cards: List<Card>, navigation: CatalogNavigation, loadingMore: Boolean, onNearEnd: () -> Unit = {}) {
    val grid = rememberLazyGridState()
    LoadMoreWhenNearEnd(grid, cards.size, onNearEnd)
    LazyVerticalGrid(
        state = grid,
        columns = GridCells.Adaptive(PosterColumn),
        contentPadding = PaddingValues(horizontal = TvSafe.horizontal, vertical = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(20.dp),
        verticalArrangement = Arrangement.spacedBy(20.dp),
        modifier = Modifier.fillMaxSize().focusRestorer(),
    ) {
        items(cards, key = { it.titleId }) { card ->
            TvPosterCard(
                name = card.name,
                posterUrl = card.poster?.url,
                accent = card.poster?.accent,
                caption = card.caption(),
                onClick = { navigation.openTitle(card.slug) },
                width = PosterColumn,
            )
        }
        if (loadingMore) {
            item(span = { GridItemSpan(maxLineSpan) }, key = "more") {
                Box(Modifier.fillMaxWidth().padding(16.dp), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator(Modifier.size(32.dp))
                }
            }
        }
    }
}

@Composable
internal fun GridSkeletonTv() {
    LazyVerticalGrid(
        columns = GridCells.Adaptive(PosterColumn),
        contentPadding = PaddingValues(horizontal = TvSafe.horizontal, vertical = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(20.dp),
        verticalArrangement = Arrangement.spacedBy(20.dp),
        userScrollEnabled = false,
        modifier = Modifier.fillMaxSize().loadingSemantics(),
    ) {
        items(12) { SkeletonBox(Modifier.fillMaxWidth().aspectRatio(2f / 3f)) }
    }
}
