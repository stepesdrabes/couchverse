package io.stepes.couchverse.playback.tv

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Icon
import androidx.tv.material3.ListItem
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.PlayKind
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.core.PlayerView
import io.stepes.couchverse.couch.CouchPanel
import io.stepes.couchverse.couch.CouchStatusPill
import io.stepes.couchverse.couch.active
import io.stepes.couchverse.couch.couchStatusText
import io.stepes.couchverse.couch.reactionChoices
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.text.formatClock
import io.stepes.couchverse.design.theme.LocalAccent
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.playback.CONTROLS_TIMEOUT_MS
import io.stepes.couchverse.playback.PlayerActions
import io.stepes.couchverse.playback.PlayerState
import io.stepes.couchverse.playback.Progress
import io.stepes.couchverse.playback.SKIP_SECONDS
import io.stepes.couchverse.playback.phone.PlayerStatus
import io.stepes.couchverse.playback.qualityLabel
import io.stepes.couchverse.playback.rememberProgress
import kotlinx.coroutines.delay

private enum class Panel { Settings, Episodes, Couch, Reactions }

/**
 * The TV's controls. Hidden, the remote still works: select plays and pauses, left and right
 * skip ten seconds, up and down bring the controls back. Back closes a panel, then hides the
 * controls, then leaves.
 */
@Composable
internal fun PlayerTv(state: PlayerState, actions: PlayerActions) {
    val view = state.view
    val progress by rememberProgress(state.player)
    val linear = view?.linear == true
    var visible by remember { mutableStateOf(true) }
    var touches by remember { mutableIntStateOf(0) }
    var panel by remember { mutableStateOf<Panel?>(null) }
    val root = remember { FocusRequester() }
    val play = remember { FocusRequester() }
    LaunchedEffect(visible, progress.playing, touches, panel) {
        if (visible && progress.playing && panel == null) {
            delay(CONTROLS_TIMEOUT_MS)
            visible = false
        }
    }
    LaunchedEffect(visible, panel) {
        // the controls take focus when they appear; hidden, the screen itself listens to the remote
        if (panel == null) runCatching { if (visible) play.requestFocus() else root.requestFocus() }
    }
    BackHandler(enabled = panel != null) { panel = null }
    BackHandler(enabled = panel == null && visible && progress.playing) { visible = false }
    val skip = { by: Double -> if (!linear) actions.onSeek((progress.positionSeconds + by).coerceAtLeast(0.0)) }

    Box(
        Modifier
            .fillMaxSize()
            .onPreviewKeyEvent {
                if (it.type == KeyEventType.KeyDown) touches++
                false
            }
            .focusRequester(root)
            .onKeyEvent { event ->
                if (visible || event.type != KeyEventType.KeyDown) return@onKeyEvent false
                when (event.key) {
                    Key.DirectionCenter, Key.Enter, Key.MediaPlayPause -> actions.onTogglePlay()
                    Key.DirectionLeft, Key.MediaRewind -> skip(-SKIP_SECONDS)
                    Key.DirectionRight, Key.MediaFastForward -> skip(SKIP_SECONDS)
                    Key.DirectionUp, Key.DirectionDown -> {}
                    else -> return@onKeyEvent false
                }
                visible = true
                true
            }
            .focusable(),
    ) {
        PlayerStatus(view, buffering = progress.buffering, actions = actions, modifier = Modifier.align(Alignment.Center))
        AnimatedVisibility(visible, enter = fadeIn(), exit = fadeOut()) {
            Controls(state, progress, linear, actions, play, skip, onPanel = { panel = it })
        }
        view?.nextUp?.let { next ->
            Column(
                Modifier
                    .align(Alignment.TopEnd)
                    .padding(horizontal = TvSafe.horizontal, vertical = TvSafe.vertical)
                    .width(340.dp)
                    .clip(RoundedCornerShape(Tokens.Radius.card))
                    .background(Tokens.Palette.surface.copy(alpha = 0.92f))
                    .padding(20.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                Text(stringResource(R.string.player_up_next, next.countdownSeconds.toString()), style = MaterialTheme.typography.labelLarge, color = LocalAccent.current.ink)
                Text(
                    "${stringResource(R.string.catalog_episode_short, next.season.toString(), next.episode.toString())} · ${next.name}",
                    style = MaterialTheme.typography.titleMedium,
                    color = Tokens.Palette.text,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    TvActionButton(stringResource(R.string.player_play_now), onClick = actions.onNextEpisode, primary = true)
                    TvActionButton(stringResource(R.string.common_cancel), onClick = actions.onCancelNext)
                }
            }
        }
        state.couch?.takeIf { it.active }?.let { couch ->
            CouchStatusPill(couch, Modifier.align(Alignment.TopCenter).padding(top = TvSafe.vertical))
        }
        AnimatedVisibility(
            panel != null,
            enter = slideInHorizontally { it } + fadeIn(),
            exit = slideOutHorizontally { it } + fadeOut(),
            modifier = Modifier.align(Alignment.CenterEnd),
        ) {
            if (view != null) {
                when (panel) {
                    Panel.Settings -> SettingsPanel(view, actions, onDone = { panel = null })
                    Panel.Episodes -> EpisodesPanel(view, actions, onDone = { panel = null })
                    Panel.Couch -> state.couch?.let { CouchSidePanel(it, actions, onDone = { panel = null }) }
                    Panel.Reactions -> state.couch?.let { ReactionsPanel(it, actions) }
                    null -> {}
                }
            }
        }
    }
}

@Composable
private fun Controls(
    state: PlayerState,
    progress: Progress,
    linear: Boolean,
    actions: PlayerActions,
    play: FocusRequester,
    skip: (Double) -> Unit,
    onPanel: (Panel) -> Unit,
) {
    val view = state.view
    val couch = state.couch
    Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.45f to Color.Transparent, 1f to Color.Black.copy(alpha = 0.85f)))) {
        Column(
            Modifier.align(Alignment.BottomStart).fillMaxWidth().padding(horizontal = TvSafe.horizontal, vertical = TvSafe.vertical),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            Row(verticalAlignment = Alignment.Bottom) {
                Column(Modifier.weight(1f)) {
                    Text(view?.title.orEmpty(), style = MaterialTheme.typography.headlineSmall, color = Tokens.Palette.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    if (!view?.subtitle.isNullOrEmpty()) {
                        Text(view.subtitle, style = MaterialTheme.typography.titleMedium, color = Tokens.Palette.muted, maxLines = 1)
                    }
                }
                Text(
                    "${formatClock(progress.positionSeconds.toLong())} / ${formatClock(progress.durationSeconds.toLong())}",
                    style = MaterialTheme.typography.titleSmall,
                    color = Tokens.Palette.text,
                )
            }
            SeekBar(progress, enabled = !linear, skip = skip)
            Row(horizontalArrangement = Arrangement.spacedBy(14.dp), verticalAlignment = Alignment.CenterVertically) {
                TvActionButton(
                    stringResource(if (progress.playing) R.string.common_pause else R.string.common_play),
                    onClick = actions.onTogglePlay,
                    icon = if (progress.playing) CouchverseIcons.Pause else Icons.Filled.PlayArrow,
                    primary = true,
                    modifier = Modifier.focusRequester(play),
                )
                if (view != null && view.seasons.isNotEmpty() && !linear) {
                    TvActionButton(stringResource(R.string.player_episodes), onClick = { onPanel(Panel.Episodes) })
                }
                if (view != null && (view.qualities.size > 1 || view.audio.size > 1 || view.subtitles.isNotEmpty())) {
                    TvActionButton(stringResource(R.string.player_subtitles), onClick = { onPanel(Panel.Settings) }, icon = CouchverseIcons.ClosedCaption)
                }
                if (view?.shuffleAvailable == true && !linear) {
                    TvActionButton(
                        stringResource(R.string.player_shuffle),
                        onClick = actions.onShuffle,
                        icon = if (view.shuffle) Icons.Filled.Check else CouchverseIcons.Shuffle,
                    )
                }
                if (state.couchEnabled) {
                    val active = couch?.active == true
                    TvActionButton(
                        if (active) stringResource(R.string.couch_on_couch_count, couch.members.size.toString()) else stringResource(R.string.couch_start_session),
                        onClick = {
                            if (!active) actions.onStartCouch()
                            onPanel(Panel.Couch)
                        },
                        icon = CouchverseIcons.Couch,
                    )
                    if (active) TvActionButton(stringResource(R.string.couch_react), onClick = { onPanel(Panel.Reactions) }, icon = CouchverseIcons.Smile)
                }
            }
        }
    }
}

/** The timeline; focused, left and right skip ten seconds. */
@Composable
private fun SeekBar(progress: Progress, enabled: Boolean, skip: (Double) -> Unit) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val fraction = (progress.positionSeconds / progress.durationSeconds.coerceAtLeast(1.0)).toFloat().coerceIn(0f, 1f)
    val label = stringResource(R.string.player_seek)
    val accent = LocalAccent.current.accent
    Box(
        Modifier
            .fillMaxWidth()
            .height(if (focused) 10.dp else 6.dp)
            .clip(RoundedCornerShape(5.dp))
            .background(Tokens.Palette.text.copy(alpha = 0.25f))
            .then(if (focused) Modifier.border(2.dp, Tokens.Palette.text, RoundedCornerShape(5.dp)) else Modifier)
            .semantics { contentDescription = label }
            .onKeyEvent { event ->
                if (event.type != KeyEventType.KeyDown) return@onKeyEvent false
                when (event.key) {
                    Key.DirectionLeft -> skip(-SKIP_SECONDS)
                    Key.DirectionRight -> skip(SKIP_SECONDS)
                    else -> return@onKeyEvent false
                }
                true
            }
            .focusable(enabled, interaction),
    ) {
        Box(Modifier.fillMaxHeight().fillMaxWidth(fraction).background(accent))
    }
}

@Composable
private fun PanelFrame(content: @Composable () -> Unit) {
    Box(
        Modifier
            .fillMaxHeight()
            .width(420.dp)
            .background(Tokens.Palette.surface.copy(alpha = 0.96f))
            .padding(vertical = TvSafe.vertical, horizontal = 24.dp),
    ) { content() }
}

@Composable
private fun SettingsPanel(view: PlayerView, actions: PlayerActions, onDone: () -> Unit) {
    val first = remember { FocusRequester() }
    LaunchedEffect(Unit) { runCatching { first.requestFocus() } }
    PanelFrame {
        LazyColumn(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            if (view.subtitles.isNotEmpty()) {
                item { PanelHeading(R.string.player_subtitles) }
                item(key = "s-off") {
                    Choice(stringResource(R.string.player_subtitle_off), view.subtitleSelected == null, Modifier.focusRequester(first)) {
                        actions.onSubtitles(null)
                        onDone()
                    }
                }
                items(view.subtitles, key = { "s-${it.id}" }) { track ->
                    Choice(track.label, track.id == view.subtitleSelected) {
                        actions.onSubtitles(track.id)
                        onDone()
                    }
                }
            }
            if (view.audio.size > 1) {
                item { PanelHeading(R.string.player_audio) }
                items(view.audio, key = { "a-${it.id}" }, itemContent = { track ->
                    Choice(track.label, track.id == view.audioSelected, if (view.subtitles.isEmpty() && track == view.audio.first()) Modifier.focusRequester(first) else Modifier) {
                        actions.onAudio(track.id)
                        onDone()
                    }
                })
            }
            if (view.qualities.size > 1) {
                item { PanelHeading(R.string.player_quality) }
                items(view.qualities, key = { "q-${it.key}" }, itemContent = { option ->
                    val start = view.subtitles.isEmpty() && view.audio.size <= 1 && option == view.qualities.first()
                    Choice(qualityLabel(option), option.key == view.quality, if (start) Modifier.focusRequester(first) else Modifier) {
                        actions.onQuality(option.key)
                        onDone()
                    }
                })
            }
        }
    }
}

@Composable
private fun EpisodesPanel(view: PlayerView, actions: PlayerActions, onDone: () -> Unit) {
    val current = remember { FocusRequester() }
    LaunchedEffect(Unit) { runCatching { current.requestFocus() } }
    PanelFrame {
        LazyColumn(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            view.seasons.forEach { season ->
                if (view.seasons.size > 1) {
                    item(key = "season-${season.number}") {
                        Text(
                            stringResource(R.string.catalog_season_number, season.number.toString()),
                            style = MaterialTheme.typography.titleSmall,
                            color = LocalAccent.current.ink,
                            modifier = Modifier.padding(top = 12.dp, bottom = 4.dp),
                        )
                    }
                }
                items(season.episodes, key = { it.id }) { episode ->
                    Choice(
                        "${episode.number}. ${episode.name}",
                        selected = episode.current,
                        modifier = if (episode.current) Modifier.focusRequester(current) else Modifier,
                    ) {
                        onDone()
                        if (!episode.current) actions.onEpisode(PlayTarget(PlayKind.Episode, episode.id))
                    }
                }
            }
        }
    }
}

@Composable
private fun CouchSidePanel(couch: CouchView, actions: PlayerActions, onDone: () -> Unit) {
    val first = remember { FocusRequester() }
    LaunchedEffect(couch.active) { runCatching { first.requestFocus() } }
    PanelFrame {
        if (!couch.active) {
            Text(couchStatusText(couch) ?: stringResource(R.string.couch_starting), color = Tokens.Palette.muted)
        } else {
            CouchPanel(
                couch,
                onEnd = {
                    onDone()
                    actions.onEndCouch()
                },
                onLeave = {
                    onDone()
                    actions.onLeaveCouch()
                },
                qrSize = 160.dp,
            ) { label, onClick -> TvActionButton(label, onClick = onClick, modifier = Modifier.focusRequester(first)) }
        }
    }
}

/** The reactions in a column, focus on the first; each one floats up for everyone. */
@Composable
private fun ReactionsPanel(couch: CouchView, actions: PlayerActions) {
    val first = remember { FocusRequester() }
    LaunchedEffect(Unit) { runCatching { first.requestFocus() } }
    PanelFrame {
        LazyColumn(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            item { PanelHeading(R.string.couch_react) }
            items(reactionChoices(couch.recentEmojis)) { emoji ->
                ListItem(
                    selected = false,
                    onClick = { actions.onReact(emoji) },
                    headlineContent = { Text(emoji, style = MaterialTheme.typography.headlineMedium) },
                    modifier = if (emoji == reactionChoices(couch.recentEmojis).first()) Modifier.focusRequester(first) else Modifier,
                )
            }
        }
    }
}

@Composable
private fun PanelHeading(label: Int) {
    Text(
        stringResource(label),
        style = MaterialTheme.typography.titleSmall,
        color = LocalAccent.current.ink,
        modifier = Modifier.padding(top = 12.dp, bottom = 4.dp),
    )
}

@Composable
private fun Choice(label: String, selected: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    ListItem(
        selected = selected,
        onClick = onClick,
        headlineContent = { Text(label, maxLines = 1, overflow = TextOverflow.Ellipsis) },
        trailingContent = if (selected) {
            { Icon(Icons.Filled.Check, contentDescription = null) }
        } else {
            null
        },
        modifier = modifier,
    )
}
