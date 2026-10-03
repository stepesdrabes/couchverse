package io.stepes.couchverse.downloads

import android.text.format.Formatter
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.DownloadItem
import io.stepes.couchverse.core.DownloadRef
import io.stepes.couchverse.core.DownloadState
import io.stepes.couchverse.core.DownloadsView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Artwork
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.components.StatusMessage
import io.stepes.couchverse.design.phone.PhoneGutter
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import java.io.File

class DownloadsActions(
    val onPlay: (DownloadItem) -> Unit,
    val onRetry: (DownloadItem) -> Unit,
    val onRemove: (DownloadItem) -> Unit,
    val onBack: (() -> Unit)? = null,
)

/**
 * The downloads on this device: what is still being prepared or fetched, what plays offline,
 * and how much room they take. Offline it is the one screen that still works.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DownloadsScreen(view: DownloadsView?, offline: Boolean, directory: File?, actions: DownloadsActions) {
    Column(Modifier.fillMaxSize()) {
        TopAppBar(
            title = { Text(stringResource(R.string.downloads_title)) },
            navigationIcon = {
                actions.onBack?.let { back ->
                    IconButton(onClick = back) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back)) }
                }
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = Tokens.Palette.bg),
        )
        if (offline) OfflineBanner()
        val items = view?.items.orEmpty()
        when {
            view == null || view.status == LoadStatus.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            items.isEmpty() -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                StatusMessage(stringResource(R.string.downloads_empty), message = stringResource(R.string.downloads_empty_hint))
            }
            else -> LazyColumn(Modifier.fillMaxSize()) {
                if (view.usedBytes > 0u) {
                    item(key = "storage") {
                        Text(
                            stringResource(R.string.downloads_storage, Formatter.formatShortFileSize(LocalContext.current, view.usedBytes.toLong())),
                            style = MaterialTheme.typography.bodySmall,
                            color = Tokens.Palette.muted,
                            modifier = Modifier.padding(horizontal = PhoneGutter, vertical = 8.dp),
                        )
                    }
                }
                items(items, key = { it.id }) { item -> DownloadRow(item, offline, directory, actions) }
            }
        }
    }
}

@Composable
private fun OfflineBanner() {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = PhoneGutter, vertical = 8.dp)
            .clip(RoundedCornerShape(Tokens.Radius.card))
            .background(Tokens.Palette.surface2)
            .padding(12.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(CouchverseIcons.CloudOff, contentDescription = null, tint = Tokens.Palette.muted)
        Text(stringResource(R.string.downloads_offline_banner), style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.text)
    }
}

@Composable
private fun DownloadRow(item: DownloadItem, offline: Boolean, directory: File?, actions: DownloadsActions) {
    val ready = item.state == DownloadState.Ready
    // the artwork kept with the download, which also shows without a network
    val art = item.artwork?.let { name -> directory?.resolve(name)?.takeIf { it.exists() }?.toURI()?.toString() }
        ?: item.image?.url?.takeIf { !offline }
    ListItem(
        colors = ListItemDefaults.colors(containerColor = Tokens.Palette.bg),
        modifier = Modifier.clickable(enabled = ready) { actions.onPlay(item) },
        leadingContent = {
            Artwork(art, accent = item.image?.accent, fallbackName = item.title, modifier = Modifier.width(112.dp).aspectRatio(16f / 9f).clip(RoundedCornerShape(8.dp)))
        },
        overlineContent = item.episode?.let { episode ->
            { Text(stringResource(R.string.catalog_episode_short, episode.season.toString(), episode.episode.toString())) }
        },
        headlineContent = {
            Text(listOfNotNull(item.title, item.episodeName?.takeIf { it.isNotBlank() }).joinToString(" · "), maxLines = 2, overflow = TextOverflow.Ellipsis)
        },
        supportingContent = { Text(stateText(item), color = if (item.state == DownloadState.Failed) Tokens.Palette.danger else Tokens.Palette.muted) },
        trailingContent = {
            Row(verticalAlignment = Alignment.CenterVertically) {
                when (item.state) {
                    DownloadState.Ready -> IconButton(onClick = { actions.onPlay(item) }) {
                        Icon(Icons.Filled.PlayArrow, contentDescription = stringResource(R.string.common_play))
                    }
                    DownloadState.Failed -> IconButton(onClick = { actions.onRetry(item) }) {
                        Icon(Icons.Filled.Refresh, contentDescription = stringResource(R.string.download_retry))
                    }
                    else -> CircularProgressIndicator(progress = { item.progress.toFloat() }, modifier = Modifier.size(24.dp), strokeWidth = 3.dp)
                }
                IconButton(onClick = { actions.onRemove(item) }) {
                    Icon(Icons.Filled.Delete, contentDescription = stringResource(R.string.download_remove))
                }
            }
        },
    )
}

/** Where a download stands, in words. */
@Composable
fun stateText(item: DownloadItem): String {
    val percent = (item.progress * 100).toInt()
    return when (item.state) {
        DownloadState.Queued -> stringResource(R.string.download_state_queued)
        DownloadState.Preparing -> stringResource(R.string.download_state_preparing, percent.toString())
        DownloadState.Fetching -> stringResource(R.string.download_state_fetching, percent.toString())
        DownloadState.Ready -> listOfNotNull(
            stringResource(R.string.download_state_ready),
            item.sizeBytes.takeIf { it > 0u }?.let { Formatter.formatShortFileSize(LocalContext.current, it.toLong()) },
        ).joinToString(" · ")
        DownloadState.Failed -> stringResource(failureText(item.problem?.code))
    }
}

fun failureText(code: String?): Int = when (code) {
    "unsupported" -> R.string.download_failed_unsupported
    "expired" -> R.string.download_failed_expired
    "prepare_failed" -> R.string.download_failed_prepare
    "no_space" -> R.string.download_failed_no_space
    else -> R.string.download_failed_fetch
}

/** [DownloadsScreen] over the core; [onPlay] opens the player on a finished download. */
@Composable
fun DownloadsRoute(onPlay: (String) -> Unit, onBack: (() -> Unit)?) {
    val send = rememberSend()
    val view by rememberSurface<DownloadsView>(Surface.Downloads, open = true)
    val session by rememberSurface<SessionView>(Surface.Session)
    val context = LocalContext.current
    DownloadsScreen(
        view = view,
        offline = session?.offline == true,
        directory = downloadsDirectory(context),
        actions = DownloadsActions(
            onPlay = { onPlay(it.id) },
            onRetry = { send(Event.DownloadRetried(DownloadRef(it.id))) },
            onRemove = { send(Event.DownloadRemoved(DownloadRef(it.id))) },
            onBack = onBack,
        ),
    )
}

