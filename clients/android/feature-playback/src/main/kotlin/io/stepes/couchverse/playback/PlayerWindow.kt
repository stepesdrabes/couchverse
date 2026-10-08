package io.stepes.couchverse.playback

import android.app.PictureInPictureParams
import android.content.pm.ActivityInfo
import android.graphics.Rect
import android.util.Rational
import android.view.View
import androidx.activity.ComponentActivity
import androidx.activity.compose.LocalActivity
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.core.app.PictureInPictureModeChangedInfo
import androidx.core.util.Consumer
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.media3.common.Player
import io.stepes.couchverse.design.theme.LocalIsTv

/** The window around the player: what the activity does while it shows. */
class PlayerWindowState(val pictureInPicture: Boolean, val enterPictureInPicture: (() -> Unit)?)

/**
 * Full screen for the player: system bars hidden, landscape on phones, and picture-in-picture
 * on phones (entered from the controls, or automatically on leaving the app while playing).
 * Leaving the player's window any other way (the screen goes off, the TV's home button, a
 * closed picture-in-picture window) pauses.
 */
@Composable
fun rememberPlayerWindow(player: Player?, playing: Boolean): PlayerWindowState {
    val activity = LocalActivity.current as? ComponentActivity
    val tv = LocalIsTv.current
    var pictureInPicture by remember { mutableStateOf(activity?.isInPictureInPictureMode == true) }

    DisposableEffect(activity) {
        if (activity == null) return@DisposableEffect onDispose {}
        val window = activity.window
        val bars = WindowCompat.getInsetsController(window, window.decorView)
        bars.systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        bars.hide(WindowInsetsCompat.Type.systemBars())
        val orientation = activity.requestedOrientation
        if (!tv) activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
        val listener = Consumer<PictureInPictureModeChangedInfo> { pictureInPicture = it.isInPictureInPictureMode }
        activity.addOnPictureInPictureModeChangedListener(listener)
        onDispose {
            activity.removeOnPictureInPictureModeChangedListener(listener)
            bars.show(WindowInsetsCompat.Type.systemBars())
            if (!tv) {
                activity.requestedOrientation = orientation
                activity.setPictureInPictureParams(pipParams(autoEnter = false, window.decorView))
            }
        }
    }
    LaunchedEffect(activity, playing) {
        if (activity != null && !tv) activity.setPictureInPictureParams(pipParams(autoEnter = playing, activity.window.decorView))
    }
    LifecycleEventEffect(Lifecycle.Event.ON_STOP) {
        if (activity?.isInPictureInPictureMode != true) player?.pause()
    }
    val enter: (() -> Unit)? = if (activity != null && !tv) {
        { activity.enterPictureInPictureMode(pipParams(autoEnter = playing, activity.window.decorView)) }
    } else {
        null
    }
    return PlayerWindowState(pictureInPicture, enter)
}

/**
 * Entered on its own while [autoEnter] (playing). The window shrinks from where the picture is
 * on the full-screen player, [screen], rather than from the whole screen.
 */
private fun pipParams(autoEnter: Boolean, screen: View): PictureInPictureParams =
    PictureInPictureParams.Builder()
        .setAspectRatio(Rational(16, 9))
        .setAutoEnterEnabled(autoEnter)
        .setSeamlessResizeEnabled(true)
        .setSourceRectHint(pictureIn(screen.width, screen.height))
        .build()

/** A 16:9 picture fitted and centred in a [width] by [height] screen; `null` before layout. */
internal fun pictureIn(width: Int, height: Int): Rect? {
    if (width <= 0 || height <= 0) return null
    val wide = minOf(width, height * 16 / 9)
    val tall = wide * 9 / 16
    val left = (width - wide) / 2
    val top = (height - tall) / 2
    return Rect(left, top, left + wide, top + tall)
}
