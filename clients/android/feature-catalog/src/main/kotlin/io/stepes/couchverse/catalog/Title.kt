package io.stepes.couchverse.catalog

import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.getValue
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import io.stepes.couchverse.catalog.phone.TitlePhone
import io.stepes.couchverse.catalog.phone.TitleSkeletonPhone
import io.stepes.couchverse.catalog.tv.TitleSkeletonTv
import io.stepes.couchverse.catalog.tv.TitleTv
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.PlayAction
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
            failed = { LoadFailed(view?.problem, actions.onRefresh) },
            notFound = { Empty(stringResource(R.string.error_not_found)) },
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
    TitleScreen(
        view,
        TitleActions(
            navigation = navigation,
            onListChange = { listed ->
                view?.detail?.let { send(Event.WatchlistChanged(WatchlistChange(it.id, listed))) }
            },
            onRefresh = { send(Event.RefreshRequested(Surface.Title(slug))) },
            onBack = onBack,
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
