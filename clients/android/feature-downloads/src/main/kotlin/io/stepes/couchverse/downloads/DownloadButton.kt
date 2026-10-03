package io.stepes.couchverse.downloads

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.DownloadItem
import io.stepes.couchverse.core.DownloadQuality
import io.stepes.couchverse.core.DownloadState
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.phone.inkButtonColors

/**
 * Keeping a movie or an episode on the phone: a button that asks for a quality, then shows the
 * download's progress, that it is ready, or a retry.
 */
@Composable
fun DownloadButton(item: DownloadItem?, onDownload: (DownloadQuality) -> Unit, onRetry: (DownloadItem) -> Unit) {
    var choosing by remember { mutableStateOf(false) }
    when (item?.state) {
        null -> IconButton(onClick = { choosing = true }) {
            Icon(CouchverseIcons.Download, contentDescription = stringResource(R.string.download_action))
        }
        DownloadState.Ready -> Box(Modifier.size(48.dp), contentAlignment = Alignment.Center) {
            Icon(Icons.Filled.Check, contentDescription = stringResource(R.string.download_state_ready), tint = Tokens.Palette.success)
        }
        DownloadState.Failed -> IconButton(onClick = { onRetry(item) }) {
            Icon(Icons.Filled.Refresh, contentDescription = stringResource(R.string.download_retry), tint = Tokens.Palette.danger)
        }
        else -> Box(Modifier.size(48.dp), contentAlignment = Alignment.Center) {
            CircularProgressIndicator(progress = { item.progress.toFloat() }, modifier = Modifier.size(24.dp), strokeWidth = 3.dp)
        }
    }
    if (choosing) {
        QualityDialog(
            onPick = {
                choosing = false
                onDownload(it)
            },
            onDismiss = { choosing = false },
        )
    }
}

@Composable
private fun QualityDialog(onPick: (DownloadQuality) -> Unit, onDismiss: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = Tokens.Palette.surface,
        title = { Text(stringResource(R.string.download_choose_quality)) },
        text = {
            Column {
                DownloadQuality.entries.forEach { quality ->
                    ListItem(
                        colors = ListItemDefaults.colors(containerColor = Color.Transparent),
                        headlineContent = {
                            Text(if (quality == DownloadQuality.Original) stringResource(R.string.download_quality_original) else quality.string)
                        },
                        supportingContent = if (quality == DownloadQuality.Original) {
                            { Text(stringResource(R.string.download_quality_original_hint)) }
                        } else {
                            null
                        },
                        modifier = Modifier.clickable { onPick(quality) },
                    )
                }
            }
        },
        confirmButton = {},
        dismissButton = { TextButton(onClick = onDismiss, colors = inkButtonColors()) { Text(stringResource(R.string.common_cancel)) } },
    )
}
