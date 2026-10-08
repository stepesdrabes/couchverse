package io.stepes.couchverse.couch

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.RemoteControl
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.couch.phone.CouchRemotePhone
import io.stepes.couchverse.design.runtime.LocalCoreRuntime
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import kotlinx.coroutines.delay

/**
 * A phone steering this account's player on another device (the TV): play and pause, the
 * timeline, and the previous and next episode. Nothing plays here, and only phones join as one.
 */
@Composable
fun CouchRemoteScreen(view: CouchView?, positionSeconds: Double, onCommand: (RemoteControl) -> Unit, onLeave: () -> Unit) {
    CouchRemotePhone(view, positionSeconds, onCommand, onLeave)
}

/** [CouchRemoteScreen] over the core; the host's position runs on while it plays. */
@Composable
fun CouchRemoteRoute(onLeft: () -> Unit) {
    val send = rememberSend()
    val runtime = LocalCoreRuntime.current
    val view by rememberSurface<CouchView>(Surface.Couch)
    var now by remember { mutableLongStateOf(runtime.nowMs()) }
    LaunchedEffect(view?.playing) {
        while (view?.playing == true) {
            now = runtime.nowMs()
            delay(500)
        }
    }
    val position = view?.let { couch ->
        val since = if (couch.playing) (now - couch.positionAtMs.toLong()).coerceAtLeast(0) / 1000.0 else 0.0
        couch.positionSeconds + since
    } ?: 0.0
    CouchRemoteScreen(
        view,
        position,
        onCommand = { send(Event.CouchRemoteCommanded(it)) },
        onLeave = {
            send(Event.CouchLeft)
            onLeft()
        },
    )
}
