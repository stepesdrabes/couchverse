package io.stepes.couchverse.design.runtime

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.State
import androidx.compose.runtime.remember
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.core.runtime.CoreRuntime
import kotlinx.serialization.serializer

/** The app's one core runtime; screens read view models from it and send it events. */
val LocalCoreRuntime = staticCompositionLocalOf<CoreRuntime> { error("no CoreRuntime provided") }

/**
 * The view model of [surface], `null` until the runtime first read it. With [open] the screen
 * also tells the core it is showing (`ScreenOpened` while composed, `ScreenClosed` after), which
 * is what makes the catalog load the surface and keep it fresh.
 */
@Composable
inline fun <reified T> rememberSurface(surface: Surface, open: Boolean = false): State<T?> {
    val runtime = LocalCoreRuntime.current
    val flow = remember(runtime, surface) { runtime.view(surface, serializer<T>()) }
    if (open) {
        DisposableEffect(runtime, surface) {
            runtime.send(Event.ScreenOpened(surface))
            onDispose { runtime.send(Event.ScreenClosed(surface)) }
        }
    }
    return flow.collectAsStateWithLifecycle()
}

/** Sends events to the core from a composable. */
@Composable
fun rememberSend(): (Event) -> Unit {
    val runtime = LocalCoreRuntime.current
    return remember(runtime) { { event: Event -> runtime.send(event) } }
}
