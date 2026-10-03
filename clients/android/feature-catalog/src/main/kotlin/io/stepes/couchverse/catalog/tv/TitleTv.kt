package io.stepes.couchverse.catalog.tv

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.gestures.BringIntoViewSpec
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.focusRestorer
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Tab
import androidx.tv.material3.TabRow
import androidx.tv.material3.Text
import io.stepes.couchverse.catalog.StaleNotice
import io.stepes.couchverse.catalog.TitleActions
import io.stepes.couchverse.catalog.label
import io.stepes.couchverse.catalog.metaLine
import io.stepes.couchverse.catalog.phone.seasonName
import io.stepes.couchverse.catalog.playLabel
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.PlayKind
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.core.TitleDetailView
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Artwork
import io.stepes.couchverse.design.components.Pill
import io.stepes.couchverse.design.components.SkeletonBox
import io.stepes.couchverse.design.components.TitleLogo
import io.stepes.couchverse.design.components.loadingSemantics
import io.stepes.couchverse.design.text.displayLocale
import io.stepes.couchverse.design.text.formatRuntime
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvBackdropCard
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.rememberScreenFocus
import io.stepes.couchverse.design.tv.screenFocus

/** A full-bleed backdrop behind the title's details, with a shelf of episodes per season. */
@OptIn(ExperimentalComposeUiApi::class, ExperimentalFoundationApi::class)
@Composable
internal fun TitleTv(detail: TitleDetailView, status: LoadStatus, problem: Problem?, actions: TitleActions) {
    val seasons = detail.seasons
    val playSeason = detail.play?.episode?.season
    var seasonIndex by rememberSaveable(detail.id) {
        mutableIntStateOf(seasons.indexOfFirst { it.number == playSeason }.coerceAtLeast(0))
    }
    val focus = rememberScreenFocus()
    val locale = displayLocale()

    Box(Modifier.fillMaxSize()) {
        Artwork(
            detail.backdrop?.url,
            accent = detail.backdrop?.accent,
            modifier = Modifier.align(Alignment.TopEnd).fillMaxWidth(0.78f).fillMaxHeight(0.82f),
        )
        Box(
            Modifier.fillMaxSize().background(
                Brush.horizontalGradient(0f to Tokens.Palette.bg, 0.3f to Tokens.Palette.bg.copy(alpha = 0.9f), 0.65f to Color.Transparent),
            ),
        )
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.45f to Color.Transparent, 0.82f to Tokens.Palette.bg)))
        // the TV's default scrolls focus a third down the screen, pushing the logo past the top edge
        CompositionLocalProvider(LocalBringIntoViewSpec provides ScrollIntoViewOnly) {
            LazyColumn(
                Modifier.fillMaxSize(),
                contentPadding = PaddingValues(top = TvSafe.vertical * 2, bottom = TvSafe.vertical * 2),
                verticalArrangement = Arrangement.spacedBy(24.dp),
            ) {
                item(key = "details") {
                    Column(
                        Modifier.padding(horizontal = TvSafe.horizontal).fillMaxWidth(0.5f),
                        verticalArrangement = Arrangement.spacedBy(14.dp),
                    ) {
                        TitleLogo(detail.name, detail.logo?.url, detail.logo?.aspect, MaterialTheme.typography.displaySmall, maxWidth = 360.dp, maxHeight = 110.dp)
                        Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
                            val meta = detail.metaLine()
                            if (meta.isNotEmpty()) Text(meta, style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.text)
                            detail.contentRating?.let { Pill(it) }
                            detail.quality?.let { Pill(it.label()) }
                            if (detail.hdr) Pill("HDR")
                        }
                        if (detail.overview.isNotBlank()) {
                            Text(
                                detail.overview,
                                style = MaterialTheme.typography.bodyMedium,
                                color = Tokens.Palette.text.copy(alpha = 0.85f),
                                maxLines = 4,
                                overflow = TextOverflow.Ellipsis,
                            )
                        }
                        if (detail.genres.isNotEmpty()) {
                            Text(detail.genres.joinToString(", "), style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.muted)
                        }
                        StaleNotice(status, problem)
                        Row(horizontalArrangement = Arrangement.spacedBy(14.dp), modifier = Modifier.padding(top = 6.dp)) {
                            val play = detail.play
                            if (play != null) {
                                TvActionButton(
                                    playLabel(play),
                                    onClick = { actions.navigation.play(play.target) },
                                    icon = Icons.Filled.PlayArrow,
                                    primary = true,
                                    modifier = Modifier.screenFocus(focus, "play", start = true),
                                )
                            }
                            TvListButton(
                                detail.inList,
                                onClick = { actions.onListChange(!detail.inList) },
                                modifier = Modifier.screenFocus(focus, "list", start = play == null),
                            )
                        }
                    }
                }
                if (seasons.isNotEmpty()) {
                    item(key = "seasons") {
                        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                            Text(
                                stringResource(R.string.catalog_episodes),
                                style = MaterialTheme.typography.titleLarge,
                                color = Tokens.Palette.text,
                                modifier = Modifier.padding(horizontal = TvSafe.horizontal).semantics { heading() },
                            )
                            if (seasons.size > 1) {
                                TabRow(selectedTabIndex = seasonIndex, modifier = Modifier.padding(horizontal = TvSafe.horizontal).focusRestorer()) {
                                    seasons.forEachIndexed { index, season ->
                                        Tab(selected = index == seasonIndex, onFocus = { seasonIndex = index }) {
                                            Text(
                                                seasonName(season),
                                                style = MaterialTheme.typography.labelLarge,
                                                modifier = Modifier.padding(horizontal = 14.dp, vertical = 6.dp),
                                            )
                                        }
                                    }
                                }
                            }
                            val episodes = seasons.getOrNull(seasonIndex)?.episodes.orEmpty()
                            LazyRow(
                                modifier = Modifier.focusRestorer(),
                                contentPadding = PaddingValues(horizontal = TvSafe.horizontal, vertical = 12.dp),
                                horizontalArrangement = Arrangement.spacedBy(20.dp),
                            ) {
                                items(episodes, key = { it.id }) { episode ->
                                    TvBackdropCard(
                                        name = "${episode.number}. ${episode.name}",
                                        imageUrl = episode.still?.url,
                                        caption = if (episode.completed) {
                                            stringResource(R.string.catalog_watched)
                                        } else {
                                            episode.runtimeMinutes?.let { formatRuntime(it.toInt(), locale) }
                                        },
                                        progress = episode.progress.toFloat().takeIf { it > 0f },
                                        onClick = { actions.navigation.play(PlayTarget(PlayKind.Episode, episode.id)) },
                                        width = 260.dp,
                                        modifier = Modifier.screenFocus(focus, "episode/${episode.id}"),
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

private val ScrollIntoViewOnly = object : BringIntoViewSpec {}

@Composable
internal fun TitleSkeletonTv() {
    Column(
        Modifier.fillMaxSize().loadingSemantics().padding(horizontal = TvSafe.horizontal, vertical = TvSafe.vertical * 2),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        SkeletonBox(Modifier.width(340.dp).height(90.dp))
        SkeletonBox(Modifier.width(220.dp).height(18.dp))
        repeat(3) { SkeletonBox(Modifier.width(440.dp).height(14.dp)) }
        Row(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
            SkeletonBox(Modifier.width(150.dp).height(40.dp))
            SkeletonBox(Modifier.width(130.dp).height(40.dp))
        }
    }
}
