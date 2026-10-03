package io.stepes.couchverse.playback

import android.app.PendingIntent
import android.content.Intent
import androidx.media3.common.Player
import androidx.media3.session.MediaSession
import androidx.media3.session.MediaSessionService
import kotlinx.coroutines.MainScope
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.drop
import kotlinx.coroutines.launch

/** The application holds the one [PlaybackEngine]; the service and the screens share it. */
interface PlaybackHost {
    val playback: PlaybackEngine
}

/**
 * The media session over the engine's player: lock screen, notification and Bluetooth
 * controls, picture-in-picture actions, and playing on while the screen is off. It lives while
 * something is loaded; a player screen starts it by connecting a controller.
 */
class PlaybackService : MediaSessionService() {
    private val scope = MainScope()
    private var session: MediaSession? = null

    override fun onCreate() {
        super.onCreate()
        val engine = (application as PlaybackHost).playback
        follow(engine.player.value)
        scope.launch { engine.player.drop(1).collect(::follow) }
    }

    private fun follow(player: Player?) {
        val current = session
        when {
            player == null -> {
                current?.release()
                session = null
                stopSelf()
            }
            current == null -> session = MediaSession.Builder(this, player).setSessionActivity(openApp()).build()
            current.player !== player -> current.player = player
        }
    }

    private fun openApp(): PendingIntent {
        val launch = packageManager.getLaunchIntentForPackage(packageName) ?: Intent()
        return PendingIntent.getActivity(this, 0, launch, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
    }

    override fun onGetSession(controllerInfo: MediaSession.ControllerInfo): MediaSession? = session

    override fun onTaskRemoved(rootIntent: Intent?) {
        // swiping the app away while paused ends playback; while playing it carries on
        val player = session?.player
        if (player == null || !player.playWhenReady) stopSelf()
    }

    override fun onDestroy() {
        scope.cancel()
        session?.release()
        session = null
        super.onDestroy()
    }
}
