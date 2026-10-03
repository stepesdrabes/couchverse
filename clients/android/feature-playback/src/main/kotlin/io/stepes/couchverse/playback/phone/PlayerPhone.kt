package io.stepes.couchverse.playback.phone

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.SecondaryScrollableTabRow
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.NextUp
import io.stepes.couchverse.core.PlayerView
import io.stepes.couchverse.couch.CouchMembers
import io.stepes.couchverse.couch.CouchPanel
import io.stepes.couchverse.couch.CouchStatusPill
import io.stepes.couchverse.couch.active
import io.stepes.couchverse.couch.couchStatusText
import io.stepes.couchverse.couch.reactionChoices
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Artwork
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.components.StatusMessage
import io.stepes.couchverse.design.phone.inkButtonColors
import io.stepes.couchverse.design.text.formatClock
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.design.theme.LocalAccent
import io.stepes.couchverse.playback.CONTROLS_TIMEOUT_MS
import io.stepes.couchverse.playback.PlayerActions
import io.stepes.couchverse.playback.PlayerState
import io.stepes.couchverse.playback.SKIP_SECONDS
import io.stepes.couchverse.playback.qualityLabel
import io.stepes.couchverse.playback.rememberProgress
import kotlinx.coroutines.delay

private enum class Sheet { Settings, Episodes, Couch, Reactions }

/** The phone's controls: a tap shows them, and they fade after a few seconds of playing. */
@Composable
internal fun PlayerPhone(state: PlayerState, actions: PlayerActions) {
    val view = state.view
    val progress by rememberProgress(state.player)
    var visible by remember { mutableStateOf(true) }
    var touches by remember { mutableIntStateOf(0) }
    var sheet by remember { mutableStateOf<Sheet?>(null) }
    LaunchedEffect(visible, progress.playing, touches, sheet) {
        if (visible && progress.playing && sheet == null) {
            delay(CONTROLS_TIMEOUT_MS)
            visible = false
        }
    }
    val touched: () -> Unit = { touches++ }
    Box(
        Modifier
            .fillMaxSize()
            .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) {
                visible = !visible
                touched()
            },
    ) {
        PlayerStatus(view, buffering = progress.buffering, actions = actions, modifier = Modifier.align(Alignment.Center))
        AnimatedVisibility(visible || !progress.playing, enter = fadeIn(), exit = fadeOut()) {
            Controls(state, actions, progress, onTouched = touched, onSheet = { sheet = it })
        }
        view?.nextUp?.let { next ->
            // above the timeline, which stays reachable while it counts down
            NextUpCard(next, actions, Modifier.align(Alignment.BottomEnd).safeDrawingPadding().padding(end = 16.dp, bottom = 120.dp))
        }
        state.couch?.takeIf { it.active }?.let { couch ->
            CouchStatusPill(couch, Modifier.align(Alignment.TopCenter).safeDrawingPadding().padding(top = 56.dp))
        }
    }
    when (sheet) {
        Sheet.Settings -> view?.let { SettingsSheet(it, actions, onDismiss = { sheet = null }) }
        Sheet.Episodes -> view?.let { EpisodesSheet(it, actions, onDismiss = { sheet = null }) }
        Sheet.Couch -> state.couch?.let { CouchSheet(it, actions, onDismiss = { sheet = null }) }
        Sheet.Reactions -> state.couch?.let { ReactionsSheet(it, actions, onDismiss = { sheet = null }) }
        null -> {}
    }
}

/** What covers the video while there is nothing to show: loading, preparing or a failure. */
@Composable
internal fun PlayerStatus(view: PlayerView?, buffering: Boolean, actions: PlayerActions, modifier: Modifier = Modifier) {
    val status = view?.status
    val preparing = view?.preparing
    when {
        preparing != null -> Column(
            modifier.padding(32.dp).width(360.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(stringResource(R.string.player_preparing_title), style = MaterialTheme.typography.titleLarge, color = Tokens.Palette.text)
            Text(stringResource(R.string.player_preparing_description), style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.muted)
            LinearProgressIndicator(progress = { preparing.toFloat() / 100f }, modifier = Modifier.fillMaxWidth())
        }
        status == LoadStatus.Failed || status == LoadStatus.NotFound -> StatusMessage(
            title = stringResource(R.string.error_page_title),
            message = problemMessage(view.problem),
            modifier = modifier,
            action = {
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    OutlinedButton(onClick = actions.onBack) { Text(stringResource(R.string.common_back)) }
                    Button(onClick = actions.onRetry) { Text(stringResource(R.string.common_retry)) }
                }
            },
        )
        status == null || status == LoadStatus.Loading || status == LoadStatus.Stale || buffering ->
            CircularProgressIndicator(modifier.size(48.dp), color = Tokens.Palette.text)
        else -> {}
    }
}

@Composable
private fun Controls(
    state: PlayerState,
    actions: PlayerActions,
    progress: io.stepes.couchverse.playback.Progress,
    onTouched: () -> Unit,
    onSheet: (Sheet) -> Unit,
) {
    val view = state.view
    val linear = view?.linear == true
    Box(
        Modifier
            .fillMaxSize()
            .background(Brush.verticalGradient(0f to Color.Black.copy(alpha = 0.7f), 0.3f to Color.Transparent, 0.65f to Color.Transparent, 1f to Color.Black.copy(alpha = 0.8f))),
    ) {
        Row(
            Modifier.fillMaxWidth().safeDrawingPadding().padding(horizontal = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(onClick = actions.onBack) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.player_back_to_title))
            }
            Column(Modifier.weight(1f).padding(horizontal = 4.dp)) {
                Text(view?.title.orEmpty(), style = MaterialTheme.typography.titleMedium, color = Tokens.Palette.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (!view?.subtitle.isNullOrEmpty()) {
                    Text(view.subtitle, style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
            val couch = state.couch
            if (couch != null && couch.active) {
                CouchMembers(couch.members, size = 28.dp, modifier = Modifier.padding(end = 4.dp))
                IconButton(onClick = { onSheet(Sheet.Reactions) }) {
                    Icon(CouchverseIcons.Smile, contentDescription = stringResource(R.string.couch_react))
                }
            }
            if (state.couchEnabled) {
                IconButton(
                    onClick = {
                        if (couch?.active != true) actions.onStartCouch()
                        onSheet(Sheet.Couch)
                    },
                ) {
                    Icon(CouchverseIcons.Couch, contentDescription = stringResource(if (couch?.active == true) R.string.couch_open else R.string.couch_start_session))
                }
            }
            actions.onPictureInPicture?.let { pip ->
                IconButton(onClick = pip) {
                    Icon(CouchverseIcons.PictureInPicture, contentDescription = stringResource(R.string.player_picture_in_picture))
                }
            }
            if (view != null && (view.qualities.size > 1 || view.audio.size > 1 || view.subtitles.isNotEmpty())) {
                IconButton(onClick = { onSheet(Sheet.Settings) }) {
                    Icon(Icons.Filled.Settings, contentDescription = stringResource(R.string.nav_settings))
                }
            }
        }

        Row(
            Modifier.align(Alignment.Center),
            horizontalArrangement = Arrangement.spacedBy(40.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (!linear) {
                SkipButton(forward = false) {
                    actions.onSeek((progress.positionSeconds - SKIP_SECONDS).coerceAtLeast(0.0))
                    onTouched()
                }
            }
            IconButton(
                onClick = {
                    actions.onTogglePlay()
                    onTouched()
                },
                modifier = Modifier.size(72.dp).clip(RoundedCornerShape(36.dp)).background(Color.Black.copy(alpha = 0.35f)),
            ) {
                Icon(
                    if (progress.playing) CouchverseIcons.Pause else Icons.Filled.PlayArrow,
                    contentDescription = stringResource(if (progress.playing) R.string.common_pause else R.string.common_play),
                    modifier = Modifier.size(48.dp),
                    tint = Tokens.Palette.text,
                )
            }
            if (!linear) {
                SkipButton(forward = true) {
                    actions.onSeek(progress.positionSeconds + SKIP_SECONDS)
                    onTouched()
                }
            }
        }

        Column(Modifier.align(Alignment.BottomCenter).fillMaxWidth().safeDrawingPadding().padding(horizontal = 16.dp, vertical = 8.dp)) {
            SeekBar(progress, enabled = !linear, onSeek = actions.onSeek, onTouched = onTouched)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "${formatClock(progress.positionSeconds.toLong())} / ${formatClock(progress.durationSeconds.toLong())}",
                    style = MaterialTheme.typography.labelMedium,
                    color = Tokens.Palette.text,
                )
                Spacer(Modifier.weight(1f))
                if (view != null && view.seasons.isNotEmpty() && !linear) {
                    TextButton(onClick = { onSheet(Sheet.Episodes) }, colors = inkButtonColors()) {
                        Text(stringResource(R.string.player_episodes), color = Tokens.Palette.text)
                    }
                }
                if (view?.shuffleAvailable == true && !linear) {
                    FilterChip(
                        selected = view.shuffle,
                        onClick = actions.onShuffle,
                        label = { Text(stringResource(R.string.player_shuffle)) },
                        leadingIcon = { Icon(CouchverseIcons.Shuffle, contentDescription = null, modifier = Modifier.size(18.dp)) },
                    )
                }
            }
        }
    }
}

/** Back or forward ten seconds: a circular arrow with the number in it. */
@Composable
internal fun SkipButton(forward: Boolean, onClick: () -> Unit) {
    val label = stringResource(if (forward) R.string.player_forward_10_seconds else R.string.player_back_10_seconds)
    IconButton(onClick = onClick, modifier = Modifier.size(56.dp).semantics { contentDescription = label }) {
        Box(contentAlignment = Alignment.Center) {
            Icon(
                CouchverseIcons.Replay,
                contentDescription = null,
                tint = Tokens.Palette.text,
                modifier = Modifier.size(40.dp).graphicsLayer { if (forward) scaleX = -1f },
            )
            Text("10", fontSize = 10.sp, color = Tokens.Palette.text, modifier = Modifier.padding(top = 4.dp))
        }
    }
}

@Composable
private fun SeekBar(progress: io.stepes.couchverse.playback.Progress, enabled: Boolean, onSeek: (Double) -> Unit, onTouched: () -> Unit) {
    val duration = progress.durationSeconds.coerceAtLeast(1.0)
    var dragging by remember { mutableStateOf(false) }
    var dragged by remember { mutableFloatStateOf(0f) }
    val value = if (dragging) dragged else (progress.positionSeconds / duration).toFloat().coerceIn(0f, 1f)
    val seekLabel = stringResource(R.string.player_seek)
    Slider(
        value = value,
        enabled = enabled,
        onValueChange = {
            dragging = true
            dragged = it
            onTouched()
        },
        onValueChangeFinished = {
            onSeek(dragged * duration)
            dragging = false
        },
        colors = SliderDefaults.colors(
            thumbColor = LocalAccent.current.accent,
            activeTrackColor = LocalAccent.current.accent,
            inactiveTrackColor = Tokens.Palette.text.copy(alpha = 0.3f),
            disabledActiveTrackColor = LocalAccent.current.accent,
            disabledThumbColor = Color.Transparent,
        ),
        modifier = Modifier.fillMaxWidth().semantics { contentDescription = seekLabel },
    )
}

/** The next episode, counting down in the last stretch; it plays at the end unless cancelled. */
@Composable
internal fun NextUpCard(next: NextUp, actions: PlayerActions, modifier: Modifier = Modifier) {
    Column(
        modifier
            .width(300.dp)
            .clip(RoundedCornerShape(Tokens.Radius.card))
            .background(Tokens.Palette.surface.copy(alpha = 0.92f))
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(
            stringResource(R.string.player_up_next, next.countdownSeconds.toString()),
            style = MaterialTheme.typography.labelLarge,
            color = LocalAccent.current.ink,
        )
        Text(
            "${stringResource(R.string.catalog_episode_short, next.season.toString(), next.episode.toString())} · ${next.name}",
            style = MaterialTheme.typography.titleSmall,
            color = Tokens.Palette.text,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = actions.onNextEpisode) { Text(stringResource(R.string.player_play_now)) }
            TextButton(onClick = actions.onCancelNext, colors = inkButtonColors()) { Text(stringResource(R.string.common_cancel)) }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun SettingsSheet(view: PlayerView, actions: PlayerActions, onDismiss: () -> Unit) {
    ModalBottomSheet(onDismissRequest = onDismiss, containerColor = Tokens.Palette.surface) {
        LazyColumn(Modifier.padding(bottom = 24.dp)) {
            if (view.qualities.size > 1) {
                item { SheetHeading(R.string.player_quality) }
                items(view.qualities, key = { "q-${it.key}" }) { option ->
                    Choice(qualityLabel(option), selected = option.key == view.quality) { actions.onQuality(option.key) }
                }
            }
            if (view.audio.size > 1) {
                item { SheetHeading(R.string.player_audio) }
                items(view.audio, key = { "a-${it.id}" }) { track ->
                    Choice(track.label, selected = track.id == view.audioSelected) { actions.onAudio(track.id) }
                }
            }
            if (view.subtitles.isNotEmpty()) {
                item { SheetHeading(R.string.player_subtitles) }
                item(key = "s-off") {
                    Choice(stringResource(R.string.player_subtitle_off), selected = view.subtitleSelected == null) { actions.onSubtitles(null) }
                }
                items(view.subtitles, key = { "s-${it.id}" }) { track ->
                    Choice(track.label, selected = track.id == view.subtitleSelected) { actions.onSubtitles(track.id) }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun CouchSheet(couch: CouchView, actions: PlayerActions, onDismiss: () -> Unit) {
    ModalBottomSheet(onDismissRequest = onDismiss, containerColor = Tokens.Palette.surface) {
        if (!couch.active) {
            Box(Modifier.fillMaxWidth().padding(32.dp), contentAlignment = Alignment.Center) {
                Text(couchStatusText(couch) ?: stringResource(R.string.couch_starting), color = Tokens.Palette.muted)
            }
        } else {
            CouchPanel(
                couch,
                onEnd = {
                    onDismiss()
                    actions.onEndCouch()
                },
                onLeave = {
                    onDismiss()
                    actions.onLeaveCouch()
                },
                qrSize = 140.dp,
                modifier = Modifier.padding(start = 24.dp, end = 24.dp, bottom = 32.dp),
            ) { label, onClick -> OutlinedButton(onClick = onClick, modifier = Modifier.fillMaxWidth()) { Text(label) } }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ReactionsSheet(couch: CouchView, actions: PlayerActions, onDismiss: () -> Unit) {
    ModalBottomSheet(onDismissRequest = onDismiss, containerColor = Tokens.Palette.surface) {
        SheetHeading(R.string.couch_react)
        androidx.compose.foundation.lazy.LazyRow(contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 16.dp, vertical = 8.dp)) {
            items(reactionChoices(couch.recentEmojis)) { emoji ->
                TextButton(onClick = { actions.onReact(emoji) }) { Text(emoji, fontSize = 30.sp) }
            }
        }
        Spacer(Modifier.padding(bottom = 24.dp))
    }
}

@Composable
private fun SheetHeading(label: Int) {
    Text(
        stringResource(label),
        style = MaterialTheme.typography.titleSmall,
        color = LocalAccent.current.ink,
        modifier = Modifier.padding(start = 24.dp, end = 24.dp, top = 16.dp, bottom = 4.dp),
    )
}

@Composable
private fun Choice(label: String, selected: Boolean, onClick: () -> Unit) {
    ListItem(
        colors = ListItemDefaults.colors(containerColor = Color.Transparent),
        headlineContent = { Text(label) },
        trailingContent = if (selected) {
            { Icon(Icons.Filled.Check, contentDescription = null, tint = LocalAccent.current.ink) }
        } else {
            null
        },
        modifier = Modifier.clickable(onClick = onClick),
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun EpisodesSheet(view: PlayerView, actions: PlayerActions, onDismiss: () -> Unit) {
    val current = view.seasons.indexOfFirst { season -> season.episodes.any { it.current } }.coerceAtLeast(0)
    var season by rememberSaveable { mutableIntStateOf(current) }
    ModalBottomSheet(onDismissRequest = onDismiss, containerColor = Tokens.Palette.surface) {
        if (view.seasons.size > 1) {
            SecondaryScrollableTabRow(selectedTabIndex = season, containerColor = Color.Transparent, edgePadding = 16.dp) {
                view.seasons.forEachIndexed { index, s ->
                    Tab(selected = index == season, onClick = { season = index }, text = { Text(stringResource(R.string.catalog_season_number, s.number.toString())) })
                }
            }
        }
        LazyColumn(Modifier.padding(bottom = 24.dp)) {
            items(view.seasons.getOrNull(season)?.episodes.orEmpty(), key = { it.id }) { episode ->
                ListItem(
                    colors = ListItemDefaults.colors(containerColor = if (episode.current) Tokens.Palette.surface2 else Color.Transparent),
                    leadingContent = {
                        Artwork(episode.still?.url, accent = episode.still?.accent, modifier = Modifier.width(120.dp).aspectRatio(16f / 9f).clip(RoundedCornerShape(8.dp)))
                    },
                    overlineContent = { Text(stringResource(R.string.player_episode_number, episode.number.toString())) },
                    headlineContent = { Text(episode.name, maxLines = 2, overflow = TextOverflow.Ellipsis) },
                    modifier = Modifier.clickable(enabled = !episode.current) {
                        onDismiss()
                        actions.onEpisode(io.stepes.couchverse.core.PlayTarget(io.stepes.couchverse.core.PlayKind.Episode, episode.id))
                    },
                )
            }
        }
    }
}
