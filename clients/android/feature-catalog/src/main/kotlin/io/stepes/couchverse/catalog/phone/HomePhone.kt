package io.stepes.couchverse.catalog.phone

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.catalog.Empty
import io.stepes.couchverse.catalog.HomeActions
import io.stepes.couchverse.catalog.StaleNotice
import io.stepes.couchverse.catalog.caption
import io.stepes.couchverse.catalog.primaryAction
import io.stepes.couchverse.catalog.progressLabel
import io.stepes.couchverse.catalog.refreshing
import io.stepes.couchverse.catalog.rowLabel
import io.stepes.couchverse.core.FeaturedCard
import io.stepes.couchverse.core.HomeView
import io.stepes.couchverse.core.TitleKind
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Artwork
import io.stepes.couchverse.design.components.Pill
import io.stepes.couchverse.design.components.SkeletonBox
import io.stepes.couchverse.design.components.TitleLogo
import io.stepes.couchverse.design.components.loadingSemantics
import io.stepes.couchverse.design.components.scrim
import io.stepes.couchverse.design.phone.BackdropCard
import io.stepes.couchverse.design.phone.HeroPager
import io.stepes.couchverse.design.phone.PhoneGutter
import io.stepes.couchverse.design.phone.PosterCard
import io.stepes.couchverse.design.phone.Shelf
import io.stepes.couchverse.design.text.displayLocale
import io.stepes.couchverse.design.text.formatRuntime

private val HeroHeight = 540.dp

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun HomePhone(view: HomeView, actions: HomeActions) {
    if (view.featured.isEmpty() && view.rows.isEmpty()) {
        Empty(stringResource(R.string.catalog_library_empty_title), stringResource(R.string.catalog_library_empty_message))
        return
    }
    val navigation = actions.navigation
    PullToRefreshBox(isRefreshing = refreshing(view.status, view.problem), onRefresh = actions.onRefresh) {
        LazyColumn(
            Modifier.fillMaxSize(),
            contentPadding = PaddingValues(bottom = 32.dp),
            verticalArrangement = Arrangement.spacedBy(28.dp),
        ) {
            if (view.featured.isNotEmpty()) {
                item(key = "hero") {
                    HeroPager(view.featured.size, Modifier.fillMaxWidth().height(HeroHeight)) { index ->
                        HeroSlidePhone(view.featured[index], actions)
                    }
                }
            } else {
                item(key = "top") { Spacer(Modifier.height(72.dp)) }
            }
            if (view.problem != null) {
                item(key = "stale") { StaleNotice(view.status, view.problem, Modifier.padding(horizontal = PhoneGutter)) }
            }
            items(view.rows, key = { it.id }) { row ->
                Shelf(rowLabel(row)) {
                    items(row.continueWatching, key = { "${it.play.kind}-${it.play.id}" }) { card ->
                        BackdropCard(
                            name = card.name,
                            imageUrl = card.backdrop?.url,
                            accent = card.backdrop?.accent,
                            caption = card.episodeLabel,
                            progress = card.progress.toFloat(),
                            progressLabel = card.progressLabel(),
                            onClick = { navigation.play(card.play) },
                        )
                    }
                    items(row.cards, key = { it.titleId }) { card ->
                        PosterCard(
                            name = card.name,
                            posterUrl = card.poster?.url,
                            accent = card.poster?.accent,
                            caption = card.caption(),
                            onClick = { navigation.openTitle(card.slug) },
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun HeroSlidePhone(card: FeaturedCard, actions: HomeActions) {
    val locale = displayLocale()
    Box(Modifier.fillMaxSize()) {
        Artwork(card.backdrop?.url, accent = card.backdrop?.accent, modifier = Modifier.fillMaxSize())
        Box(Modifier.fillMaxSize().background(scrim()))
        // keeps the status bar icons readable over bright artwork
        Box(
            Modifier
                .fillMaxWidth()
                .height(120.dp)
                .background(Brush.verticalGradient(listOf(Tokens.Palette.bg.copy(alpha = 0.7f), Color.Transparent))),
        )
        Column(
            Modifier.align(Alignment.BottomStart).padding(start = PhoneGutter, end = PhoneGutter, bottom = 32.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                (listOf(stringResource(R.string.catalog_featured)) + card.genres.take(2)).joinToString("  ·  ").uppercase(locale),
                style = MaterialTheme.typography.labelSmall,
                color = Tokens.Palette.text.copy(alpha = 0.8f),
            )
            TitleLogo(card.name, card.logo?.url, card.logo?.aspect, MaterialTheme.typography.displaySmall, maxWidth = 280.dp, maxHeight = 110.dp)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                val meta = listOfNotNull(
                    card.year?.toString(),
                    card.runtimeMinutes?.takeIf { card.kind == TitleKind.Movie }?.let { formatRuntime(it.toInt(), locale) },
                )
                if (meta.isNotEmpty()) {
                    Text(meta.joinToString("  ·  "), style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.text)
                }
                card.contentRating?.let { Pill(it) }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                val playLabel = stringResource(R.string.catalog_play_episode, card.name)
                Button(onClick = { card.primaryAction(actions.navigation) }, modifier = Modifier.semantics { contentDescription = playLabel }) {
                    Icon(Icons.Filled.PlayArrow, contentDescription = null, modifier = Modifier.size(20.dp))
                    Spacer(Modifier.width(6.dp))
                    Text(stringResource(R.string.common_play))
                }
                ListButton(card.inList) { actions.onListChange(card.titleId, !card.inList) }
            }
        }
    }
}

/** Adds to or removes from My List; the icon says which way it is now. */
@Composable
internal fun ListButton(inList: Boolean, onClick: () -> Unit) {
    val label = stringResource(if (inList) R.string.catalog_remove_from_list else R.string.catalog_add_to_list)
    FilledTonalButton(onClick = onClick, modifier = Modifier.semantics { contentDescription = label }) {
        Icon(if (inList) Icons.Filled.Check else Icons.Filled.Add, contentDescription = null, modifier = Modifier.size(20.dp))
        Spacer(Modifier.width(6.dp))
        Text(stringResource(R.string.nav_my_list))
    }
}

@Composable
internal fun HomeSkeletonPhone() {
    Column(Modifier.fillMaxSize().loadingSemantics(), verticalArrangement = Arrangement.spacedBy(28.dp)) {
        Box(Modifier.fillMaxWidth().height(HeroHeight).background(Tokens.Palette.surface)) {
            Column(Modifier.align(Alignment.BottomStart).padding(PhoneGutter), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                SkeletonBox(Modifier.width(220.dp).height(64.dp))
                SkeletonBox(Modifier.width(140.dp).height(18.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    SkeletonBox(Modifier.width(110.dp).height(40.dp))
                    SkeletonBox(Modifier.width(120.dp).height(40.dp))
                }
            }
        }
        repeat(2) {
            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                SkeletonBox(Modifier.padding(horizontal = PhoneGutter).width(160.dp).height(22.dp))
                LazyRow(
                    contentPadding = PaddingValues(horizontal = PhoneGutter),
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    userScrollEnabled = false,
                ) {
                    items(5) { SkeletonBox(Modifier.width(128.dp).aspectRatio(2f / 3f)) }
                }
            }
        }
    }
}
