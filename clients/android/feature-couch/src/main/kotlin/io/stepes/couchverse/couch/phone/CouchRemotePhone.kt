package io.stepes.couchverse.couch.phone

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.FilledIconButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.RemoteAction
import io.stepes.couchverse.core.RemoteControl
import io.stepes.couchverse.couch.CouchMembers
import io.stepes.couchverse.couch.couchStatusText
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.text.formatClock

@Composable
internal fun CouchRemotePhone(view: CouchView?, positionSeconds: Double, onCommand: (RemoteControl) -> Unit, onLeave: () -> Unit) {
    Box(Modifier.fillMaxSize().safeDrawingPadding()) {
        IconButton(onClick = onLeave, modifier = Modifier.padding(8.dp)) {
            Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.couch_leave))
        }
        Column(
            Modifier.align(Alignment.Center).fillMaxWidth().padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(24.dp),
        ) {
            Text(
                stringResource(R.string.couch_remote_title),
                style = MaterialTheme.typography.headlineSmall,
                color = Tokens.Palette.text,
                modifier = Modifier.semantics { heading() },
            )
            view?.let { couchStatusText(it) }?.let { Text(it, color = Tokens.Palette.muted) }
            view?.let { CouchMembers(it.members) }
            Text(formatClock(positionSeconds.toLong()), style = MaterialTheme.typography.displaySmall, color = Tokens.Palette.text)
            Row(horizontalArrangement = Arrangement.spacedBy(24.dp), verticalAlignment = Alignment.CenterVertically) {
                val seek = { by: Double -> onCommand(RemoteControl(RemoteAction.Seek, (positionSeconds + by).coerceAtLeast(0.0))) }
                IconButton(onClick = { seek(-SKIP) }) {
                    Icon(CouchverseIcons.Replay, contentDescription = stringResource(R.string.player_back_10_seconds))
                }
                IconButton(onClick = { seek(SKIP) }) {
                    Icon(CouchverseIcons.Replay, contentDescription = stringResource(R.string.player_forward_10_seconds), modifier = Modifier.graphicsLayer { scaleX = -1f })
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(32.dp), verticalAlignment = Alignment.CenterVertically) {
                IconButton(onClick = { onCommand(RemoteControl(RemoteAction.Previous)) }) {
                    Icon(CouchverseIcons.SkipNext, contentDescription = stringResource(R.string.couch_remote_previous), modifier = Modifier.graphicsLayer { scaleX = -1f })
                }
                val playing = view?.playing == true
                FilledIconButton(
                    onClick = { onCommand(RemoteControl(if (playing) RemoteAction.Pause else RemoteAction.Play)) },
                    modifier = Modifier.size(80.dp),
                ) {
                    Icon(
                        if (playing) CouchverseIcons.Pause else Icons.Filled.PlayArrow,
                        contentDescription = stringResource(if (playing) R.string.common_pause else R.string.common_play),
                        modifier = Modifier.size(44.dp),
                    )
                }
                IconButton(onClick = { onCommand(RemoteControl(RemoteAction.Next)) }) {
                    Icon(CouchverseIcons.SkipNext, contentDescription = stringResource(R.string.couch_remote_next))
                }
            }
            OutlinedButton(onClick = onLeave) { Text(stringResource(R.string.couch_leave)) }
        }
    }
}

private const val SKIP = 10.0
