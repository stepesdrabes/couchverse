package io.stepes.couchverse.catalog.phone

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.Button
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.IconButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.catalog.StaleNotice
import io.stepes.couchverse.catalog.TitleActions
import io.stepes.couchverse.catalog.label
import io.stepes.couchverse.catalog.metaLine
import io.stepes.couchverse.catalog.playLabel
import io.stepes.couchverse.core.EpisodeView
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.PlayKind
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.core.SeasonView
import io.stepes.couchverse.core.TitleDetailView
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Artwork
import io.stepes.couchverse.design.components.Pill
import io.stepes.couchverse.design.components.SkeletonBox
import io.stepes.couchverse.design.components.TitleLogo
import io.stepes.couchverse.design.components.WatchProgress
import io.stepes.couchverse.design.components.loadingSemantics
import io.stepes.couchverse.design.components.scrim
import io.stepes.couchverse.design.phone.PhoneGutter
import io.stepes.couchverse.design.text.displayLocale
import io.stepes.couchverse.design.text.formatRuntime
import io.stepes.couchverse.design.theme.LocalAccent

@Composable
internal fun TitlePhone(detail: TitleDetailView, status: LoadStatus, problem: Problem?, actions: TitleActions) {
    val seasons = detail.seasons
    // opens on the season the play button would continue in
    val playSeason = detail.play?.episode?.season
    var seasonIndex by rememberSaveable(detail.id) {
        mutableIntStateOf(seasons.indexOfFirst { it.number == playSeason }.coerceAtLeast(0))
    }
    val season = seasons.getOrNull(seasonIndex)
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = 32.dp)) {
        item(key = "header") { Header(detail, actions.onBack) }
        item(key = "summary") { Summary(detail, status, problem, actions) }
        if (seasons.isNotEmpty()) {
            item(key = "seasons") { SeasonPicker(seasons, seasonIndex) { seasonIndex = it } }
            if (season != null) {
                items(season.episodes, key = { it.id }) { episode ->
                    EpisodeRow(episode) { actions.navigation.play(PlayTarget(PlayKind.Episode, episode.id)) }
                }
            }
        }
    }
}

@Composable
private fun Header(detail: TitleDetailView, onBack: (() -> Unit)?) {
    Box(Modifier.fillMaxWidth().aspectRatio(4f / 3.4f)) {
        Artwork(
            detail.backdrop?.url,
            accent = detail.backdrop?.accent,
            fallbackName = null,
            modifier = Modifier.matchParentSize(),
        )
        Box(Modifier.matchParentSize().background(scrim()))
        if (onBack != null) {
            IconButton(
                onClick = onBack,
                colors = IconButtonDefaults.iconButtonColors(containerColor = Tokens.Palette.bg.copy(alpha = 0.55f)),
                modifier = Modifier.statusBarsPadding().padding(8.dp),
            ) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back))
            }
        }
        TitleLogo(
            detail.name,
            detail.logo?.url,
            detail.logo?.aspect,
            MaterialTheme.typography.displaySmall,
            maxWidth = 260.dp,
            maxHeight = 100.dp,
            modifier = Modifier.align(Alignment.BottomStart).padding(horizontal = PhoneGutter, vertical = 8.dp),
        )
    }
}

@Composable
private fun Summary(detail: TitleDetailView, status: LoadStatus, problem: Problem?, actions: TitleActions) {
    Column(Modifier.padding(horizontal = PhoneGutter, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            val meta = detail.metaLine()
            if (meta.isNotEmpty()) {
                Text(meta, style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.text, modifier = Modifier.align(Alignment.CenterVertically))
            }
            detail.contentRating?.let { Pill(it) }
            detail.quality?.let { Pill(it.label()) }
            if (detail.hdr) Pill("HDR")
        }
        StaleNotice(status, problem)
        detail.play?.let { play ->
            Button(
                onClick = { actions.navigation.play(play.target) },
                modifier = Modifier.fillMaxWidth().heightIn(min = 52.dp),
            ) {
                Icon(Icons.Filled.PlayArrow, contentDescription = null, modifier = Modifier.size(22.dp))
                Spacer(Modifier.width(8.dp))
                Text(playLabel(play))
            }
        }
        ListButton(detail.inList) { actions.onListChange(!detail.inList) }
        if (detail.overview.isNotBlank()) {
            Text(detail.overview, style = MaterialTheme.typography.bodyLarge, color = Tokens.Palette.text.copy(alpha = 0.9f))
        }
        if (detail.genres.isNotEmpty()) {
            Text(detail.genres.joinToString(", "), style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.muted)
        }
    }
}

@Composable
private fun SeasonPicker(seasons: List<SeasonView>, selected: Int, onSelect: (Int) -> Unit) {
    Column(Modifier.padding(top = 20.dp, bottom = 8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Text(
            stringResource(R.string.catalog_episodes),
            style = MaterialTheme.typography.titleLarge,
            color = Tokens.Palette.text,
            modifier = Modifier.padding(horizontal = PhoneGutter).semantics { heading() },
        )
        if (seasons.size > 1) {
            LazyRow(contentPadding = PaddingValues(horizontal = PhoneGutter), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                itemsIndexed(seasons, key = { _, season -> season.id }) { index, season ->
                    FilterChip(
                        selected = index == selected,
                        onClick = { onSelect(index) },
                        label = { Text(seasonName(season)) },
                    )
                }
            }
        } else {
            Text(
                seasonName(seasons.first()),
                style = MaterialTheme.typography.bodyMedium,
                color = Tokens.Palette.muted,
                modifier = Modifier.padding(horizontal = PhoneGutter),
            )
        }
    }
}

@Composable
internal fun seasonName(season: SeasonView): String =
    season.name.ifBlank { stringResource(R.string.catalog_season_number, season.number.toString()) }

@Composable
private fun EpisodeRow(episode: EpisodeView, onClick: () -> Unit) {
    val locale = displayLocale()
    val watched = stringResource(R.string.catalog_watched)
    val label = stringResource(R.string.catalog_play_episode, "${episode.number}. ${episode.name}")
    Row(
        Modifier
            .fillMaxWidth()
            .clickable(role = Role.Button, onClick = onClick)
            .semantics(mergeDescendants = true) { contentDescription = label }
            .padding(horizontal = PhoneGutter, vertical = 10.dp),
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Box(Modifier.width(136.dp).aspectRatio(16f / 9f).clip(RoundedCornerShape(10.dp))) {
            Artwork(episode.still?.url, fallbackName = null, modifier = Modifier.matchParentSize())
            if (episode.completed) {
                Icon(
                    Icons.Filled.Check,
                    contentDescription = watched,
                    tint = LocalAccent.current.onAccent,
                    modifier = Modifier
                        .align(Alignment.TopEnd)
                        .padding(6.dp)
                        .background(LocalAccent.current.accent, CircleShape)
                        .padding(2.dp)
                        .size(14.dp),
                )
            } else if (episode.progress > 0) {
                WatchProgress(episode.progress.toFloat(), Modifier.align(Alignment.BottomCenter).padding(8.dp))
            }
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(
                "${episode.number}. ${episode.name}",
                style = MaterialTheme.typography.titleSmall,
                color = Tokens.Palette.text,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            episode.runtimeMinutes?.let {
                Text(formatRuntime(it.toInt(), locale), style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.muted)
            }
            if (episode.overview.isNotBlank()) {
                Text(
                    episode.overview,
                    style = MaterialTheme.typography.bodySmall,
                    color = Tokens.Palette.muted,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}

@Composable
internal fun TitleSkeletonPhone(onBack: (() -> Unit)?) {
    Column(Modifier.fillMaxSize().loadingSemantics(), verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Box(Modifier.fillMaxWidth().aspectRatio(4f / 3.4f).background(SkeletonArt)) {
            if (onBack != null) {
                IconButton(onClick = onBack, modifier = Modifier.statusBarsPadding().padding(8.dp)) {
                    Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back))
                }
            }
            SkeletonBox(Modifier.align(Alignment.BottomStart).padding(PhoneGutter).width(220.dp).height(56.dp))
        }
        Column(Modifier.padding(horizontal = PhoneGutter), verticalArrangement = Arrangement.spacedBy(14.dp)) {
            SkeletonBox(Modifier.width(180.dp).height(18.dp))
            SkeletonBox(Modifier.fillMaxWidth().height(52.dp))
            SkeletonBox(Modifier.width(150.dp).height(40.dp))
            repeat(3) { SkeletonBox(Modifier.fillMaxWidth().height(16.dp)) }
        }
    }
}
