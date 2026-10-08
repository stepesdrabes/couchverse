// Media3 marks most of ExoPlayer's construction and its view's styling unstable; both are used as documented
@file:OptIn(UnstableApi::class)

package io.stepes.couchverse.playback

import android.content.ComponentName
import android.graphics.Color
import androidx.annotation.OptIn
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.State
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.viewinterop.AndroidView
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.session.MediaController
import androidx.media3.session.SessionToken
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.PlayerView as MediaPlayerView
import io.stepes.couchverse.core.CouchPause
import io.stepes.couchverse.core.CouchReaction
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.DownloadRef
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.core.PlayerView
import io.stepes.couchverse.core.QualityChoice
import io.stepes.couchverse.core.QualityKind
import io.stepes.couchverse.core.QualityOption
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.core.TrackChoice
import io.stepes.couchverse.couch.ReactionsOverlay
import io.stepes.couchverse.couch.active
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.playback.phone.PlayerPhone
import io.stepes.couchverse.playback.tv.PlayerTv
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.suspendCancellableCoroutine
import com.google.common.util.concurrent.ListenableFuture
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/** What opened the player screen. */
sealed interface PlayerStart {
    data class Title(val target: PlayTarget) : PlayerStart

    /** A finished download, played from the device. */
    data class Download(val id: String) : PlayerStart

    /** A couch follower's player: the core loads whatever the host plays. */
    data object Couch : PlayerStart
}

/** The app's player engine; absent in previews and screenshots, which show no video. */
val LocalPlaybackEngine = staticCompositionLocalOf<PlaybackEngine?> { null }

/** What the player screen shows: the core's view and the player it drives. */
class PlayerState(
    val view: PlayerView?,
    val player: Player?,
    /** Inside a picture-in-picture window: only the video. */
    val pictureInPicture: Boolean = false,
    /** The couch session, when the account's server has couch sessions on or one is live. */
    val couch: CouchView? = null,
    val couchEnabled: Boolean = false,
)

class PlayerActions(
    val onBack: () -> Unit,
    val onTogglePlay: () -> Unit,
    val onSeek: (seconds: Double) -> Unit,
    val onQuality: (String) -> Unit,
    val onAudio: (String) -> Unit,
    val onSubtitles: (String?) -> Unit,
    val onEpisode: (PlayTarget) -> Unit,
    val onNextEpisode: () -> Unit,
    val onCancelNext: () -> Unit,
    val onShuffle: () -> Unit,
    val onRetry: () -> Unit,
    /** Shrinks the player into a window over other apps; phones only. */
    val onPictureInPicture: (() -> Unit)? = null,
    val onStartCouch: () -> Unit = {},
    val onEndCouch: () -> Unit = {},
    val onLeaveCouch: () -> Unit = {},
    val onReact: (String) -> Unit = {},
)

/** The full-screen player: the video, and the phone's or the TV's controls over it. */
@Composable
fun PlayerScreen(state: PlayerState, actions: PlayerActions) {
    Box(Modifier.fillMaxSize().background(androidx.compose.ui.graphics.Color.Black)) {
        VideoSurface(state.player, Modifier.fillMaxSize())
        state.couch?.takeIf { it.active }?.let { ReactionsOverlay(it.reactions) }
        if (!state.pictureInPicture) {
            if (LocalIsTv.current) PlayerTv(state, actions) else PlayerPhone(state, actions)
        }
    }
}

/** [PlayerScreen] over the core and the engine. */
@Composable
fun PlayerRoute(start: PlayerStart, onBack: () -> Unit) {
    val send = rememberSend()
    val view by rememberSurface<PlayerView>(Surface.Player)
    val session by rememberSurface<SessionView>(Surface.Session)
    val couchView by rememberSurface<CouchView>(Surface.Couch)
    // a live couch counts as couch sessions being on: a guest has no account to say so
    val couchEnabled = session?.accountId != null && session?.features?.couch == true || couchView?.active == true
    val couch = couchView.takeIf { couchEnabled }
    val engine = LocalPlaybackEngine.current
    val player by remember(engine) { engine?.player ?: MutableStateFlow(null) }.collectAsStateWithLifecycle()
    val requested = remember(start) {
        {
            when (start) {
                is PlayerStart.Title -> send(Event.PlayRequested(start.target))
                is PlayerStart.Download -> send(Event.DownloadPlayRequested(DownloadRef(start.id)))
                PlayerStart.Couch -> {}
            }
        }
    }
    DisposableEffect(start) {
        requested()
        onDispose { send(Event.PlayerClosed) }
    }
    MediaSessionConnection(active = player != null)
    val window = rememberPlayerWindow(player, playing = rememberProgress(player).value.playing)
    val linear = view?.linear == true
    PlayerScreen(
        PlayerState(view, player, window.pictureInPicture, couch, couchEnabled),
        PlayerActions(
            onBack = onBack,
            onTogglePlay = {
                player?.let {
                    // a follower pauses for themself; the core keeps them on the host's timeline
                    if (linear) send(Event.CouchLocalPauseChanged(CouchPause(paused = it.playWhenReady))) else it.playWhenReady = !it.playWhenReady
                }
            },
            onSeek = { seconds -> if (!linear) player?.seekTo((seconds * 1000).toLong()) },
            onQuality = { send(Event.QualityChosen(QualityChoice(it))) },
            onAudio = { send(Event.AudioChosen(TrackChoice(it))) },
            onSubtitles = { send(Event.SubtitlesChosen(TrackChoice(it))) },
            onEpisode = { send(Event.PlayRequested(it)) },
            onNextEpisode = { send(Event.NextEpisodeRequested) },
            onCancelNext = { send(Event.NextEpisodeCancelled) },
            onShuffle = { send(Event.ShuffleToggled) },
            onRetry = requested,
            onPictureInPicture = window.enterPictureInPicture,
            onStartCouch = { send(Event.CouchStartRequested) },
            onEndCouch = { send(Event.CouchEndRequested) },
            onLeaveCouch = { send(Event.CouchLeft) },
            onReact = { send(Event.CouchEmojiSent(CouchReaction(it))) },
        ),
    )
}

/**
 * Keeps a controller connected to [PlaybackService] while something is loaded, which starts
 * the service and with it the media session and its notification.
 */
@Composable
private fun MediaSessionConnection(active: Boolean) {
    if (!active) return
    val context = LocalContext.current
    LaunchedEffect(Unit) {
        val token = SessionToken(context, ComponentName(context, PlaybackService::class.java))
        val controller = runCatching { MediaController.Builder(context, token).buildAsync().awaitResult() }.getOrNull()
            ?: return@LaunchedEffect
        try {
            kotlinx.coroutines.awaitCancellation()
        } finally {
            controller.release()
        }
    }
}

/** The video and its subtitles; Media3's view draws both, the controls are Compose. */
@Composable
fun VideoSurface(player: Player?, modifier: Modifier = Modifier) {
    AndroidView(
        factory = { context ->
            MediaPlayerView(context).apply {
                useController = false
                keepScreenOn = true
                resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
                setShutterBackgroundColor(Color.BLACK)
                setKeepContentOnPlayerReset(true)
            }
        },
        update = { it.player = player },
        onRelease = { it.player = null },
        modifier = modifier,
    )
}

/** Where playback is, refreshed a few times a second while shown. */
class Progress(val positionSeconds: Double, val durationSeconds: Double, val playing: Boolean, val buffering: Boolean)

@Composable
fun rememberProgress(player: Player?, fallbackDuration: Double = 0.0): State<Progress> =
    produceState(Progress(0.0, fallbackDuration, false, false), player) {
        while (player != null) {
            value = Progress(
                positionSeconds = player.currentPosition.coerceAtLeast(0) / 1000.0,
                durationSeconds = player.duration.takeIf { it > 0 }?.div(1000.0) ?: fallbackDuration,
                playing = player.playWhenReady,
                buffering = player.playbackState == Player.STATE_BUFFERING,
            )
            delay(250)
        }
    }

/** "Original", "Auto" or "1080p". */
@Composable
fun qualityLabel(option: QualityOption): String = when (option.kind) {
    QualityKind.Original -> stringResource(R.string.player_quality_original)
    QualityKind.Auto -> stringResource(R.string.player_quality_auto)
    QualityKind.Rendition -> option.height?.let { "${it}p" } ?: option.key
}

/** The controls fade out after a few seconds of playing untouched. */
const val CONTROLS_TIMEOUT_MS = 4000L

/** Seconds a skip button jumps. */
const val SKIP_SECONDS = 10.0

private suspend fun <T> ListenableFuture<T>.awaitResult(): T = suspendCancellableCoroutine { continuation ->
    addListener({ runCatching { get() }.fold(continuation::resume, continuation::resumeWithException) }, Runnable::run)
    continuation.invokeOnCancellation { cancel(false) }
}
