package io.stepes.couchverse.catalog.tv

import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Carousel
import androidx.tv.material3.CarouselDefaults
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import androidx.tv.material3.rememberCarouselState
import io.stepes.couchverse.catalog.Empty
import io.stepes.couchverse.catalog.HomeActions
import io.stepes.couchverse.catalog.StaleNotice
import io.stepes.couchverse.catalog.caption
import io.stepes.couchverse.catalog.primaryAction
import io.stepes.couchverse.catalog.progressLabel
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
import io.stepes.couchverse.design.text.displayLocale
import io.stepes.couchverse.design.text.formatRuntime
import io.stepes.couchverse.design.theme.LocalReducedMotion
import io.stepes.couchverse.design.theme.Motion
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvBackdropCard
import io.stepes.couchverse.design.tv.TvPosterCard
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.TvShelf
import io.stepes.couchverse.design.tv.rememberScreenFocus
import io.stepes.couchverse.design.tv.screenFocus

private val HeroHeight = 340.dp

@Composable
internal fun HomeTv(view: HomeView, actions: HomeActions) {
    if (view.featured.isEmpty() && view.rows.isEmpty()) {
        Empty(stringResource(R.string.catalog_library_empty_title), stringResource(R.string.catalog_library_empty_message))
        return
    }
    val navigation = actions.navigation
    val focus = rememberScreenFocus()
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(bottom = TvSafe.vertical * 2),
        verticalArrangement = Arrangement.spacedBy(20.dp),
    ) {
        if (view.featured.isNotEmpty()) {
            item(key = "hero") { HeroTv(view.featured, actions, Modifier.screenFocus(focus, "hero", start = true)) }
        }
        if (view.problem != null) {
            item(key = "stale") { StaleNotice(view.status, view.problem, Modifier.padding(horizontal = TvSafe.horizontal)) }
        }
        itemsIndexed(view.rows, key = { _, row -> row.id }) { rowIndex, row ->
            val watching = row.continueWatching.map { "${row.id}/play/${it.play.id}" }
            val titles = row.cards.map { "${row.id}/${it.titleId}" }
            // without a hero, focus starts on the first card of the first row
            val first = (watching + titles).firstOrNull().takeIf { rowIndex == 0 && view.featured.isEmpty() }
            TvShelf(rowLabel(row)) {
                itemsIndexed(row.continueWatching, key = { _, card -> "${card.play.kind}-${card.play.id}" }) { index, card ->
                    TvBackdropCard(
                        name = card.name,
                        imageUrl = card.backdrop?.url,
                        accent = card.backdrop?.accent,
                        caption = card.episodeLabel,
                        progress = card.progress.toFloat(),
                        progressLabel = card.progressLabel(),
                        onClick = { navigation.play(card.play) },
                        modifier = Modifier.screenFocus(focus, watching[index], start = watching[index] == first),
                    )
                }
                itemsIndexed(row.cards, key = { _, card -> card.titleId }) { index, card ->
                    TvPosterCard(
                        name = card.name,
                        posterUrl = card.poster?.url,
                        accent = card.poster?.accent,
                        caption = card.caption(),
                        onClick = { navigation.openTitle(card.slug) },
                        modifier = Modifier.screenFocus(focus, titles[index], start = titles[index] == first),
                    )
                }
            }
        }
    }
}

/** The cinematic hero: it advances every 8 seconds and holds while focus is inside it. */
@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
private fun HeroTv(featured: List<FeaturedCard>, actions: HomeActions, modifier: Modifier) {
    val state = rememberCarouselState()
    val fade = Motion.ambient<Float>(600)
    Carousel(
        itemCount = featured.size,
        carouselState = state,
        autoScrollDurationMillis = if (LocalReducedMotion.current) Long.MAX_VALUE else Tokens.Motion.heroIntervalMillis,
        contentTransformStartToEnd = fadeIn(fade) togetherWith fadeOut(fade),
        contentTransformEndToStart = fadeIn(fade) togetherWith fadeOut(fade),
        carouselIndicator = {
            CarouselDefaults.IndicatorRow(
                itemCount = featured.size,
                activeItemIndex = state.activeItemIndex,
                modifier = Modifier.align(Alignment.BottomEnd).padding(horizontal = TvSafe.horizontal, vertical = 16.dp),
            )
        },
        modifier = modifier.fillMaxWidth().height(HeroHeight),
    ) { index ->
        HeroSlideTv(featured[index], actions)
    }
}

@Composable
private fun HeroSlideTv(card: FeaturedCard, actions: HomeActions) {
    val locale = displayLocale()
    Box(Modifier.fillMaxSize()) {
        Artwork(
            card.backdrop?.url,
            accent = card.backdrop?.accent,
            modifier = Modifier.align(Alignment.TopEnd).fillMaxHeight().fillMaxWidth(0.72f),
        )
        Box(
            Modifier.fillMaxSize().background(
                Brush.horizontalGradient(
                    0f to Tokens.Palette.bg,
                    0.32f to Tokens.Palette.bg.copy(alpha = 0.92f),
                    0.62f to Color.Transparent,
                ),
            ),
        )
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.6f to Color.Transparent, 1f to Tokens.Palette.bg)))
        Column(
            Modifier
                .align(Alignment.CenterStart)
                .padding(start = TvSafe.horizontal, top = TvSafe.vertical)
                .fillMaxWidth(0.46f),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                (listOf(stringResource(R.string.catalog_featured)) + card.genres.take(2)).joinToString("  ·  ").uppercase(locale),
                style = MaterialTheme.typography.labelSmall,
                color = Tokens.Palette.text.copy(alpha = 0.75f),
            )
            TitleLogo(card.name, card.logo?.url, card.logo?.aspect, MaterialTheme.typography.displaySmall, maxWidth = 320.dp, maxHeight = 96.dp)
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
                val meta = listOfNotNull(
                    card.year?.toString(),
                    card.runtimeMinutes?.takeIf { card.kind == TitleKind.Movie }?.let { formatRuntime(it.toInt(), locale) },
                )
                if (meta.isNotEmpty()) {
                    Text(meta.joinToString("  ·  "), style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.text)
                }
                card.contentRating?.let { Pill(it) }
            }
            if (card.overview.isNotBlank()) {
                Text(
                    card.overview,
                    style = MaterialTheme.typography.bodyMedium,
                    color = Tokens.Palette.muted,
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(14.dp), modifier = Modifier.padding(top = 4.dp)) {
                val playLabel = stringResource(R.string.catalog_play_episode, card.name)
                TvActionButton(
                    stringResource(R.string.common_play),
                    onClick = { card.primaryAction(actions.navigation) },
                    icon = Icons.Filled.PlayArrow,
                    primary = true,
                    modifier = Modifier.semantics { contentDescription = playLabel },
                )
                TvListButton(card.inList, onClick = { actions.onListChange(card.titleId, !card.inList) })
            }
        }
    }
}

/** Adds to or removes from My List on the TV. */
@Composable
internal fun TvListButton(inList: Boolean, onClick: () -> Unit, modifier: Modifier = Modifier) {
    val label = stringResource(if (inList) R.string.catalog_remove_from_list else R.string.catalog_add_to_list)
    TvActionButton(
        stringResource(R.string.nav_my_list),
        onClick = onClick,
        icon = if (inList) Icons.Filled.Check else Icons.Filled.Add,
        modifier = modifier.semantics { contentDescription = label },
    )
}

@Composable
internal fun HomeSkeletonTv() {
    Column(Modifier.fillMaxSize().loadingSemantics(), verticalArrangement = Arrangement.spacedBy(24.dp)) {
        Column(
            Modifier.height(HeroHeight).padding(start = TvSafe.horizontal, top = TvSafe.vertical * 2),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            SkeletonBox(Modifier.width(120.dp).height(12.dp))
            SkeletonBox(Modifier.width(300.dp).height(72.dp))
            SkeletonBox(Modifier.width(200.dp).height(16.dp))
            SkeletonBox(Modifier.width(380.dp).height(48.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                SkeletonBox(Modifier.width(110.dp).height(40.dp))
                SkeletonBox(Modifier.width(130.dp).height(40.dp))
            }
        }
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            SkeletonBox(Modifier.padding(horizontal = TvSafe.horizontal).width(180.dp).height(22.dp))
            LazyRow(
                contentPadding = PaddingValues(horizontal = TvSafe.horizontal),
                horizontalArrangement = Arrangement.spacedBy(20.dp),
                userScrollEnabled = false,
            ) {
                items(7) { SkeletonBox(Modifier.width(136.dp).aspectRatio(2f / 3f)) }
            }
        }
    }
}
