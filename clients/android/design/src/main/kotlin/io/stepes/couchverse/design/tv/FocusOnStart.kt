package io.stepes.couchverse.design.tv

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.Saver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.layout.onPlaced

/**
 * Where focus starts on a TV screen. Focus is requested once the element is placed: inside a
 * lazy list an item is only composed during layout, after the screen's own effects ran.
 */
@Composable
fun Modifier.focusOnStart(requester: FocusRequester = remember { FocusRequester() }, enabled: Boolean = true): Modifier {
    var placed by remember { mutableStateOf(false) }
    LaunchedEffect(placed, enabled) {
        if (placed && enabled) runCatching { requester.requestFocus() }
    }
    return focusRequester(requester).onPlaced { placed = true }
}

/**
 * Which element of a TV screen has focus, kept in the screen's saved state. Navigation disposes
 * the screen it leaves, focus restorers included, so without this Back from a title would land
 * on the screen's first element rather than on the poster the title was opened from.
 */
class ScreenFocus internal constructor(internal var key: String?) {
    /** Until focus first lands, the remembered element (or else the start) takes it once placed. */
    internal var landing = true
}

@Composable
fun rememberScreenFocus(): ScreenFocus = rememberSaveable(saver = ScreenFocusSaver) { ScreenFocus(null) }

private val ScreenFocusSaver = Saver<ScreenFocus, String>(save = { it.key }, restore = { ScreenFocus(it) })

/**
 * Makes this element [key] of [screen]: it takes focus when the screen shows if it had focus
 * when the screen was left, or, the first time, if it is the [start]. Only the landing moves
 * focus, so content that refreshes later never pulls focus back from where the viewer went.
 */
@Composable
fun Modifier.screenFocus(screen: ScreenFocus, key: String, start: Boolean = false): Modifier =
    onFocusChanged {
        if (it.hasFocus) {
            screen.key = key
            screen.landing = false
        }
    }.focusOnStart(enabled = screen.landing && (screen.key == key || (screen.key == null && start)))
