package io.stepes.couchverse.catalog

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import io.stepes.couchverse.catalog.phone.GenreGridPhone
import io.stepes.couchverse.catalog.phone.GridSkeletonPhone
import io.stepes.couchverse.catalog.phone.PhoneHeader
import io.stepes.couchverse.catalog.phone.PosterGridPhone
import io.stepes.couchverse.catalog.tv.GenreGridTv
import io.stepes.couchverse.catalog.tv.GridSkeletonTv
import io.stepes.couchverse.catalog.tv.PosterGridTv
import io.stepes.couchverse.catalog.tv.TvHeader
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.GenresView
import io.stepes.couchverse.core.MyListView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.components.LoadState
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv

/** The genres as tiles; each opens its listing. */
@Composable
fun GenresScreen(view: GenresView?, navigation: CatalogNavigation, onRefresh: () -> Unit, showTitle: Boolean = true) {
    val tv = LocalIsTv.current
    TitledPage(stringResource(R.string.nav_genres).takeIf { showTitle }) {
        LoadState(
            status = view?.status,
            skeleton = { if (tv) GridSkeletonTv() else GridSkeletonPhone() },
            failed = { LoadFailed(view?.problem, onRefresh) },
        ) {
            val genres = view?.genres.orEmpty()
            when {
                genres.isEmpty() -> Empty(
                    stringResource(R.string.catalog_genres_empty_title),
                    stringResource(R.string.catalog_genres_empty_message),
                )
                tv -> GenreGridTv(genres) { navigation.openGenre(it.name, it.label) }
                else -> GenreGridPhone(genres) { navigation.openGenre(it.name, it.label) }
            }
        }
    }
}

@Composable
fun GenresRoute(navigation: CatalogNavigation, showTitle: Boolean = true) {
    val send = rememberSend()
    val view by rememberSurface<GenresView>(Surface.Genres, open = true)
    GenresScreen(view, navigation, onRefresh = { send(Event.RefreshRequested(Surface.Genres)) }, showTitle = showTitle)
}

/** The titles the viewer saved. */
@Composable
fun MyListScreen(view: MyListView?, navigation: CatalogNavigation, onRefresh: () -> Unit) {
    val tv = LocalIsTv.current
    TitledPage(stringResource(R.string.nav_my_list)) {
        LoadState(
            status = view?.status,
            skeleton = { if (tv) GridSkeletonTv() else GridSkeletonPhone() },
            failed = { LoadFailed(view?.problem, onRefresh) },
        ) {
            val cards = view?.cards.orEmpty()
            when {
                cards.isEmpty() -> Empty(
                    stringResource(R.string.catalog_my_list_empty_title),
                    stringResource(R.string.catalog_my_list_empty_message),
                )
                tv -> PosterGridTv(cards, navigation, loadingMore = false)
                else -> PosterGridPhone(cards, navigation, loadingMore = false)
            }
        }
    }
}

@Composable
fun MyListRoute(navigation: CatalogNavigation) {
    val send = rememberSend()
    val view by rememberSurface<MyListView>(Surface.MyList, open = true)
    MyListScreen(view, navigation, onRefresh = { send(Event.RefreshRequested(Surface.MyList)) })
}

/** A page with its title in the idiom's header style, or none when it is embedded in tabs. */
@Composable
internal fun TitledPage(title: String?, content: @Composable () -> Unit) {
    Column(Modifier.fillMaxSize()) {
        if (title != null) {
            if (LocalIsTv.current) TvHeader(title) else PhoneHeader(title, onBack = null)
        }
        Box(Modifier.weight(1f).padding()) { content() }
    }
}
