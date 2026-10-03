package io.stepes.couchverse.design.tv

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalFocusManager
import io.stepes.couchverse.design.theme.LocalIsTv

/**
 * A text field keeps the arrow keys for its cursor, so once the keyboard is closed a remote
 * would be stuck in it. On TV, up and down move focus on, and so does left from an [empty] field.
 */
@Composable
fun Modifier.remoteLeavesField(empty: Boolean): Modifier {
    if (!LocalIsTv.current) return this
    val focus = LocalFocusManager.current
    return onPreviewKeyEvent { event ->
        val direction = when (event.key) {
            Key.DirectionDown -> FocusDirection.Down
            Key.DirectionUp -> FocusDirection.Up
            Key.DirectionLeft -> FocusDirection.Left.takeIf { empty }
            else -> null
        }
        direction != null && event.type == KeyEventType.KeyDown && focus.moveFocus(direction)
    }
}
