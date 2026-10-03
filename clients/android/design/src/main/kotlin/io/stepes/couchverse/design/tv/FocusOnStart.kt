package io.stepes.couchverse.design.tv

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
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
