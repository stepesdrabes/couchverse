package io.stepes.couchverse.catalog

import androidx.compose.foundation.lazy.grid.LazyGridState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.res.stringResource
import io.stepes.couchverse.catalog.phone.BrowsePhone
import io.stepes.couchverse.catalog.phone.GridSkeletonPhone
import io.stepes.couchverse.catalog.phone.PosterGridPhone
import io.stepes.couchverse.catalog.tv.BrowseTv
import io.stepes.couchverse.catalog.tv.GridSkeletonTv
import io.stepes.couchverse.catalog.tv.PosterGridTv
import io.stepes.couchverse.core.BrowseKey
import io.stepes.couchverse.core.BrowseSort
import io.stepes.couchverse.core.BrowseView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.GenresView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.core.TitleKind
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.components.LoadState
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv

/** A listing's title and the filters it offers. */
class BrowseState(
    val title: String,
    val view: BrowseView?,
    val sort: BrowseSort,
    /** Genres to filter a movie or series listing by; empty for a genre's own listing. */
    val genres: GenresView? = null,
    val genre: String? = null,
    /** False inside the phone's Browse tabs, which name the listing already. */
    val showTitle: Boolean = true,
)

class BrowseActions(
    val navigation: CatalogNavigation,
    val onSort: (BrowseSort) -> Unit,
    val onGenre: (String?) -> Unit,
    val onMore: () -> Unit,
    val onRefresh: () -> Unit,
    val onBack: (() -> Unit)? = null,
)

/** Movies, series or one genre as a grid of posters that loads more as it scrolls. */
@Composable
fun BrowseScreen(state: BrowseState, actions: BrowseActions) {
    val tv = LocalIsTv.current
    val content: @Composable () -> Unit = {
        LoadState(
            status = state.view?.status,
            skeleton = { if (tv) GridSkeletonTv() else GridSkeletonPhone() },
            failed = { LoadFailed(state.view?.problem, actions.onRefresh) },
        ) {
            state.view?.let { view ->
                if (view.cards.isEmpty()) {
                    Empty(stringResource(R.string.catalog_browse_empty_title), stringResource(R.string.catalog_browse_empty_message))
                } else {
                    BrowseGrid(view, actions)
                }
            }
        }
    }
    if (tv) BrowseTv(state, actions, content) else BrowsePhone(state, actions, content)
}

@Composable
private fun BrowseGrid(view: BrowseView, actions: BrowseActions) {
    if (LocalIsTv.current) {
        PosterGridTv(view.cards, actions.navigation, view.loadingMore, onNearEnd = actions.onMore)
    } else {
        PosterGridPhone(view.cards, actions.navigation, view.loadingMore, onNearEnd = actions.onMore)
    }
}

/** Movies or series ([kind]), or one genre's titles ([genre], headed [label] until the core names it). */
@Composable
fun BrowseRoute(
    kind: TitleKind?,
    genre: String?,
    label: String,
    navigation: CatalogNavigation,
    onBack: (() -> Unit)? = null,
    showTitle: Boolean = true,
) {
    val send = rememberSend()
    var sort by rememberSaveable { mutableStateOf(BrowseSort.Added) }
    var filter by rememberSaveable { mutableStateOf<String?>(null) }
    val key = BrowseKey(kind = kind, genre = genre ?: filter, sort = sort)
    val view by rememberSurface<BrowseView>(Surface.Browse(key), open = true)
    // a kind's listing can be narrowed to a genre
    val genres = if (genre == null) rememberSurface<GenresView>(Surface.Genres, open = true).value else null
    BrowseScreen(
        // the core names a genre in the current display language; the label it was opened with is a stand-in
        BrowseState(view?.genreLabel?.takeIf { genre != null } ?: label, view, sort, genres, filter, showTitle),
        BrowseActions(
            navigation = navigation,
            onSort = { sort = it },
            onGenre = { filter = it },
            onMore = { send(Event.BrowseMoreRequested(key)) },
            onRefresh = { send(Event.RefreshRequested(Surface.Browse(key))) },
            onBack = onBack,
        ),
    )
}

/** Calls [onNearEnd] when the grid scrolls within a page of its end, as the web does. */
@Composable
internal fun LoadMoreWhenNearEnd(grid: LazyGridState, count: Int, onNearEnd: () -> Unit) {
    val latest by rememberUpdatedState(onNearEnd)
    LaunchedEffect(grid, count) {
        snapshotFlow { grid.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0 }
            .collect { last -> if (count > 0 && last >= count - NEAR_END) latest() }
    }
}

private const val NEAR_END = 12

/** The sort options in the order the web lists them. */
val SortOptions = listOf(BrowseSort.Added, BrowseSort.Name, BrowseSort.Year)

@Composable
fun BrowseSort.label(): String = stringResource(
    when (this) {
        BrowseSort.Added -> R.string.catalog_sort_recently_added
        BrowseSort.Name -> R.string.catalog_sort_name
        BrowseSort.Year -> R.string.catalog_sort_year
    },
)
