package io.stepes.couchverse.catalog

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.res.stringResource
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.components.GlowBackdrop
import io.stepes.couchverse.design.components.StatusMessage
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.focusOnStart

/** Where a play button leads until the player arrives with playback (Phase 12). */
@Composable
fun PlayPlaceholderScreen(onBack: () -> Unit) {
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        GlowBackdrop(intensity = 0.5f)
        StatusMessage(
            title = stringResource(R.string.catalog_play_unavailable_title),
            message = stringResource(R.string.catalog_play_unavailable_message),
            action = {
                if (LocalIsTv.current) {
                    val focus = remember { FocusRequester() }
                    TvActionButton(stringResource(R.string.common_back), onClick = onBack, primary = true, modifier = Modifier.focusOnStart(focus))
                } else {
                    Button(onClick = onBack) { Text(stringResource(R.string.common_back)) }
                }
            },
        )
    }
}
