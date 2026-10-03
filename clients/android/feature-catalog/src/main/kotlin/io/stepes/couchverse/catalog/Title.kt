package io.stepes.couchverse.catalog

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.catalog.phone.TitlePhone
import io.stepes.couchverse.catalog.phone.TitleSkeletonPhone
import io.stepes.couchverse.catalog.tv.TitleSkeletonTv
import io.stepes.couchverse.catalog.tv.TitleTv
import io.stepes.couchverse.core.DownloadAsk
import io.stepes.couchverse.core.DownloadItem
import io.stepes.couchverse.core.DownloadQuality
import io.stepes.couchverse.core.DownloadRef
import io.stepes.couchverse.core.DownloadsView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.PlayAction
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.core.TitleDetailView
import io.stepes.couchverse.core.TitleKind
import io.stepes.couchverse.core.TitleView
import io.stepes.couchverse.core.WatchlistChange
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.components.LoadState
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.text.displayLocale
import io.stepes.couchverse.design.text.formatClock
import io.stepes.couchverse.design.text.formatRuntime
import io.stepes.couchverse.design.theme.AccentColors
import io.stepes.couchverse.design.theme.AccentScope
import io.stepes.couchverse.design.theme.LocalIsTv

class TitleActions(
    val navigation: CatalogNavigation,
    val onListChange: (listed: Boolean) -> Unit,
    val onRefresh: () -> Unit,
    val onBack: (() -> Unit)?,
    /** Keeping the movie or its episodes on the phone; absent on TVs and where downloads are off. */
    val downloads: TitleDownloads? = null,
)

class TitleDownloads(
    val items: List<DownloadItem>,
    val onDownload: (PlayTarget, DownloadQuality) -> Unit,
    val onRetry: (DownloadItem) -> Unit,
)

/** A title page: backdrop and logo, the play or resume button, My List, seasons and episodes. */
@Composable
fun TitleScreen(view: TitleView?, actions: TitleActions) {
    val tv = LocalIsTv.current
    val detail = view?.detail
    // the page takes its colours from the backdrop, as on the web
    AccentScope(detail?.accent?.let(AccentColors::of)) {
        LoadState(
            status = view?.status,
            skeleton = { if (tv) TitleSkeletonTv() else TitleSkeletonPhone(actions.onBack) },
            failed = { WithBack(actions.onBack) { LoadFailed(view?.problem, actions.onRefresh) } },
            notFound = { WithBack(actions.onBack) { Empty(stringResource(R.string.error_not_found)) } },
        ) {
            if (detail != null) {
                if (tv) TitleTv(detail, view.status, view.problem, actions) else TitlePhone(detail, view.status, view.problem, actions)
            }
        }
    }
}

@Composable
fun TitleRoute(slug: String, navigation: CatalogNavigation, onBack: (() -> Unit)?) {
    val send = rememberSend()
    val view by rememberSurface<TitleView>(Surface.Title(slug), open = true)
    val session by rememberSurface<SessionView>(Surface.Session)
    val downloadable = !LocalIsTv.current && session?.features?.downloads == true
    val downloads = if (downloadable) rememberSurface<DownloadsView>(Surface.Downloads, open = true).value else null
    TitleScreen(
        view,
        TitleActions(
            navigation = navigation,
            onListChange = { listed ->
                view?.detail?.let { send(Event.WatchlistChanged(WatchlistChange(it.id, listed))) }
            },
            onRefresh = { send(Event.RefreshRequested(Surface.Title(slug))) },
            onBack = onBack,
            downloads = if (downloadable) {
                TitleDownloads(
                    items = downloads?.items.orEmpty(),
                    onDownload = { target, quality -> send(Event.DownloadRequested(DownloadAsk(target, quality))) },
                    onRetry = { send(Event.DownloadRetried(DownloadRef(it.id))) },
                )
            } else {
                null
            },
        ),
    )
}

/** The play button's words: resume where the viewer stopped, or which episode it starts. */
@Composable
@ReadOnlyComposable
fun playLabel(play: PlayAction): String {
    val episode = play.episode?.let { episodeLabel(it) }
    val resume = play.resumeSeconds
    return when {
        episode != null && resume != null -> stringResource(R.string.catalog_resume_episode, episode)
        episode != null -> stringResource(R.string.catalog_play_episode, episode)
        resume != null -> stringResource(R.string.catalog_resume_from, formatClock(resume.toLong()))
        else -> stringResource(R.string.common_play)
    }
}

/** "2024 · 1 h 52 min" or "2021 · 3 seasons". */
@Composable
@ReadOnlyComposable
fun TitleDetailView.metaLine(): String {
    val locale = displayLocale()
    val length = when (kind) {
        TitleKind.Series -> if (seasons.isEmpty()) null else pluralStringResource(R.plurals.catalog_season_count, seasons.size, seasons.size)
        TitleKind.Movie -> runtimeMinutes?.let { formatRuntime(it.toInt(), locale) }
    }
    return listOfNotNull(year?.toString(), length).joinToString("  ·  ")
}

/** A page with nothing to show still needs a way back on phones (the TV has its Back key). */
@Composable
private fun WithBack(onBack: (() -> Unit)?, content: @Composable () -> Unit) {
    Box(Modifier.fillMaxSize()) {
        content()
        if (onBack != null && !LocalIsTv.current) {
            IconButton(onClick = onBack, modifier = Modifier.statusBarsPadding().padding(8.dp)) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back))
            }
        }
    }
}
