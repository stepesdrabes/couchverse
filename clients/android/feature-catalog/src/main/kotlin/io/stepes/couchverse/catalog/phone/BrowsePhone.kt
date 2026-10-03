package io.stepes.couchverse.catalog.phone

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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.ArrowDropDown
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.catalog.BrowseActions
import io.stepes.couchverse.catalog.BrowseState
import io.stepes.couchverse.catalog.CatalogNavigation
import io.stepes.couchverse.catalog.LoadMoreWhenNearEnd
import io.stepes.couchverse.catalog.SortOptions
import io.stepes.couchverse.catalog.caption
import io.stepes.couchverse.catalog.label
import io.stepes.couchverse.core.Card
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.phone.inkButtonColors
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.SkeletonBox
import io.stepes.couchverse.design.components.loadingSemantics
import io.stepes.couchverse.design.phone.PhoneGutter
import io.stepes.couchverse.design.phone.PosterCard

private val PosterColumn = 108.dp

@Composable
internal fun BrowsePhone(state: BrowseState, actions: BrowseActions, content: @Composable () -> Unit) {
    Column(Modifier.fillMaxSize()) {
        if (state.showTitle) {
            PhoneHeader(state.title, actions.onBack) { SortMenu(state, actions) }
        } else {
            Row(Modifier.fillMaxWidth().padding(end = 4.dp), horizontalArrangement = Arrangement.End) { SortMenu(state, actions) }
        }
        val genres = state.genres?.genres.orEmpty()
        if (genres.isNotEmpty()) {
            LazyRow(
                contentPadding = PaddingValues(horizontal = PhoneGutter),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.padding(bottom = 8.dp),
            ) {
                item(key = "all") {
                    FilterChip(state.genre == null, onClick = { actions.onGenre(null) }, label = { Text(stringResource(R.string.catalog_filter_all)) })
                }
                items(genres, key = { it.name }) { genre ->
                    FilterChip(
                        selected = state.genre == genre.name,
                        onClick = { actions.onGenre(genre.name.takeIf { it != state.genre }) },
                        label = { Text(genre.label) },
                    )
                }
            }
        }
        Box(Modifier.weight(1f)) { content() }
    }
}

/** A screen title under the status bar, with an optional back arrow and trailing actions. */
@Composable
internal fun PhoneHeader(title: String, onBack: (() -> Unit)?, actions: @Composable () -> Unit = {}) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(start = if (onBack == null) PhoneGutter else 4.dp, end = 4.dp, top = 8.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (onBack != null) {
            IconButton(onClick = onBack) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back))
            }
        }
        Text(
            title,
            style = MaterialTheme.typography.headlineMedium,
            color = Tokens.Palette.text,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f).semantics { heading() },
        )
        actions()
    }
}

@Composable
private fun SortMenu(state: BrowseState, actions: BrowseActions) {
    var open by remember { mutableStateOf(false) }
    val description = listOf(stringResource(R.string.catalog_sort_label), state.sort.label()).joinToString(", ")
    Box {
        TextButton(
            onClick = { open = true },
            colors = inkButtonColors(),
            modifier = Modifier.semantics { contentDescription = description },
        ) {
            Text(state.sort.label())
            Icon(Icons.Filled.ArrowDropDown, contentDescription = null)
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            SortOptions.forEach { sort ->
                DropdownMenuItem(
                    text = { Text(sort.label()) },
                    onClick = {
                        open = false
                        actions.onSort(sort)
                    },
                )
            }
        }
    }
}

/** Posters filling the width; more load as the end comes into view. */
@Composable
internal fun PosterGridPhone(cards: List<Card>, navigation: CatalogNavigation, loadingMore: Boolean, onNearEnd: () -> Unit = {}) {
    val grid = rememberLazyGridState()
    LoadMoreWhenNearEnd(grid, cards.size, onNearEnd)
    LazyVerticalGrid(
        state = grid,
        columns = GridCells.Adaptive(PosterColumn),
        contentPadding = PaddingValues(horizontal = PhoneGutter, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
        modifier = Modifier.fillMaxSize(),
    ) {
        items(cards, key = { it.titleId }) { card ->
            PosterCard(
                name = card.name,
                posterUrl = card.poster?.url,
                accent = card.poster?.accent,
                caption = card.caption(),
                onClick = { navigation.openTitle(card.slug) },
                width = null,
            )
        }
        if (loadingMore) {
            item(span = { GridItemSpan(maxLineSpan) }, key = "more") {
                Box(Modifier.fillMaxWidth().padding(16.dp), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator(Modifier.size(28.dp))
                }
            }
        }
    }
}

@Composable
internal fun GridSkeletonPhone() {
    LazyVerticalGrid(
        columns = GridCells.Adaptive(PosterColumn),
        contentPadding = PaddingValues(horizontal = PhoneGutter, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
        userScrollEnabled = false,
        modifier = Modifier.fillMaxSize().loadingSemantics(),
    ) {
        items(15) { SkeletonBox(Modifier.fillMaxWidth().aspectRatio(2f / 3f)) }
    }
}
