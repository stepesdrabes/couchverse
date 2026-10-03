// Media3 marks most of ExoPlayer's construction and its view's styling unstable; both are used as documented
@file:OptIn(UnstableApi::class)

package io.stepes.couchverse.playback

import android.content.Context
import android.os.Handler
import android.os.Looper
import androidx.annotation.OptIn
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.Tracks
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DefaultDataSource
import androidx.media3.datasource.HttpDataSource
import androidx.media3.datasource.okhttp.OkHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.exoplayer.source.MediaSource
import androidx.media3.exoplayer.upstream.DefaultLoadErrorHandlingPolicy
import androidx.media3.exoplayer.upstream.LoadErrorHandlingPolicy
import io.stepes.couchverse.core.PlayerCommand
import io.stepes.couchverse.core.PlayerLoad
import io.stepes.couchverse.core.PlayerReport
import io.stepes.couchverse.core.PlayerSubtitle
import io.stepes.couchverse.core.runtime.PlayerExecutor
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import okhttp3.OkHttpClient
import java.io.File

/**
 * The app's one video player, run by the core's `player` effects. Commands arrive on the core's
 * thread and are applied on the main thread, where ExoPlayer lives; what the player does goes
 * back to the core through [report] on every state change and about once a second while playing.
 * The player is created by the first load and released by `stop`; screens and the media session
 * follow [player].
 */
class PlaybackEngine(
    private val context: Context,
    private val downloads: File,
    private val report: (PlayerReport) -> Unit,
    private val build: (Context) -> ExoPlayer = { defaultPlayer(it, null) },
    looper: Looper = Looper.getMainLooper(),
) : PlayerExecutor {
    constructor(context: Context, client: OkHttpClient, downloads: File, report: (PlayerReport) -> Unit) :
        this(context, downloads, report, { defaultPlayer(it, client) })

    private val main = Handler(looper)
    private val current = MutableStateFlow<ExoPlayer?>(null)
    private val loaded = MutableStateFlow<PlayerLoad?>(null)

    /** The player while something is loaded, for the screen and the media session. */
    val player: StateFlow<ExoPlayer?> = current.asStateFlow()

    /** The last load: what is playing, and whether it is a couch follower's (linear) player. */
    val load: StateFlow<PlayerLoad?> = loaded.asStateFlow()

    /** The subtitle track to show once the tracks are known, `null` for none. */
    private var subtitle: PlayerSubtitle? = null
    private var subtitlePending = false
    private var lastReport: PlayerReport? = null

    private val listener = object : Player.Listener {
        override fun onEvents(player: Player, events: Player.Events) {
            if (events.containsAny(
                    Player.EVENT_IS_PLAYING_CHANGED,
                    Player.EVENT_PLAYBACK_STATE_CHANGED,
                    Player.EVENT_PLAYER_ERROR,
                    Player.EVENT_POSITION_DISCONTINUITY,
                    Player.EVENT_TIMELINE_CHANGED,
                )
            ) {
                send()
            }
            if (events.contains(Player.EVENT_IS_PLAYING_CHANGED)) ticking(player.isPlaying)
        }

        override fun onTracksChanged(tracks: Tracks) {
            if (subtitlePending) chooseSubtitles(tracks)
        }
    }

    private val tick = object : Runnable {
        override fun run() {
            send(always = true)
            main.postDelayed(this, TICK_MS)
        }
    }

    override fun perform(command: PlayerCommand) {
        main.post { apply(command) }
    }

    private fun apply(command: PlayerCommand) {
        when (command) {
            is PlayerCommand.Load -> load(command.content)
            PlayerCommand.Play -> current.value?.play()
            PlayerCommand.Pause -> current.value?.pause()
            is PlayerCommand.Seek -> current.value?.seekTo((command.content.seconds * 1000).toLong())
            is PlayerCommand.SelectAudio -> current.value?.let { player ->
                player.trackSelectionParameters = player.trackSelectionParameters.buildUpon()
                    .setPreferredAudioLanguage(command.content.lang)
                    .build()
            }
            is PlayerCommand.SelectSubtitles -> {
                subtitle = loaded.value?.subtitles?.firstOrNull { it.id == command.content.id }
                current.value?.let { chooseSubtitles(it.currentTracks) }
            }
            PlayerCommand.Stop -> stop()
        }
    }

    private fun load(load: PlayerLoad) {
        val player = current.value ?: build(context).also {
            it.addListener(listener)
            current.value = it
        }
        loaded.value = load
        lastReport = null
        subtitle = load.subtitles.firstOrNull { it.id == load.subtitle }
        player.trackSelectionParameters = player.trackSelectionParameters.buildUpon().applyLoad(load).build()
        player.setMediaItem(mediaItem(load, downloads), (load.startSeconds * 1000).toLong())
        player.playWhenReady = load.autoplay
        player.prepare()
        chooseSubtitles(player.currentTracks)
    }

    private fun chooseSubtitles(tracks: Tracks) {
        val player = current.value ?: return
        val builder = player.trackSelectionParameters.buildUpon()
        // a track inside the media only shows up once it is prepared
        subtitlePending = !builder.selectSubtitles(subtitle, tracks)
        if (!subtitlePending) player.trackSelectionParameters = builder.build()
    }

    private fun stop() {
        main.removeCallbacks(tick)
        current.value?.let {
            it.removeListener(listener)
            it.release()
        }
        current.value = null
        loaded.value = null
        lastReport = null
    }

    private fun ticking(playing: Boolean) {
        main.removeCallbacks(tick)
        if (playing) main.postDelayed(tick, TICK_MS)
    }

    private fun send(always: Boolean = false) {
        val player = current.value ?: return
        val report = reportOf(player, loaded.value)
        if (always || report != lastReport) {
            lastReport = report
            report(report)
        }
    }

    private companion object {
        const val TICK_MS = 1000L
    }
}

/** What [player] is doing, as the core hears it. */
fun reportOf(player: Player, load: PlayerLoad?): PlayerReport {
    val duration = player.duration.takeIf { it != C.TIME_UNSET }?.div(1000.0)
        ?: load?.nowPlaying?.durationSeconds ?: 0.0
    return PlayerReport(
        positionSeconds = player.currentPosition.coerceAtLeast(0) / 1000.0,
        durationSeconds = duration,
        playing = player.isPlaying,
        buffering = player.playbackState == Player.STATE_BUFFERING,
        ended = player.playbackState == Player.STATE_ENDED,
        failed = player.playerError?.let(::reason),
    )
}

/** A short reason for a failure: the HTTP status when the server refused, else Media3's code. */
fun reason(error: PlaybackException): String {
    val http = generateSequence(error.cause) { it.cause }.filterIsInstance<HttpDataSource.InvalidResponseCodeException>().firstOrNull()
    return if (http != null) "http_${http.responseCode}" else error.errorCodeName.removePrefix("ERROR_CODE_").lowercase()
}

/** ExoPlayer over the app's OkHttp client (its trust settings and timeouts) for the network. */
fun defaultPlayer(context: Context, client: OkHttpClient?): ExoPlayer =
    ExoPlayer.Builder(context)
        .setMediaSourceFactory(mediaSources(context, client))
        .setAudioAttributes(
            AudioAttributes.Builder().setUsage(C.USAGE_MEDIA).setContentType(C.AUDIO_CONTENT_TYPE_MOVIE).build(),
            true,
        )
        .setHandleAudioBecomingNoisy(true)
        .build()

/** Where media comes from: OkHttp for the network, files for downloads. */
fun mediaSources(context: Context, client: OkHttpClient?): MediaSource.Factory {
    val network = client?.let { OkHttpDataSource.Factory(it) }
    val data = if (network != null) DefaultDataSource.Factory(context, network) else DefaultDataSource.Factory(context)
    return DefaultMediaSourceFactory(data).setLoadErrorHandlingPolicy(FailOnRefusal)
}

/**
 * A server that refuses a request (an expired grant, a title that went away) does not change
 * its mind on a retry: failing at once lets the core load again with a fresh grant.
 */
private object FailOnRefusal : DefaultLoadErrorHandlingPolicy() {
    override fun getRetryDelayMsFor(info: LoadErrorHandlingPolicy.LoadErrorInfo): Long {
        val http = info.exception as? HttpDataSource.InvalidResponseCodeException
        return if (http != null && http.responseCode in 400..499) C.TIME_UNSET else super.getRetryDelayMsFor(info)
    }
}
