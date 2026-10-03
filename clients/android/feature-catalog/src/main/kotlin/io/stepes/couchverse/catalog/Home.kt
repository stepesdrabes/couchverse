package io.stepes.couchverse.catalog

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import io.stepes.couchverse.catalog.phone.HomePhone
import io.stepes.couchverse.catalog.phone.HomeSkeletonPhone
import io.stepes.couchverse.catalog.tv.HomeSkeletonTv
import io.stepes.couchverse.catalog.tv.HomeTv
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.HomeView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.core.WatchlistChange
import io.stepes.couchverse.design.components.LoadState
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv

class HomeActions(
    val navigation: CatalogNavigation,
    val onListChange: (titleId: String, listed: Boolean) -> Unit,
    val onRefresh: () -> Unit,
)

/** Home: the featured heroes, continue watching and the rows the admin set up. */
@Composable
fun HomeScreen(view: HomeView?, actions: HomeActions) {
    val tv = LocalIsTv.current
    LoadState(
        status = view?.status,
        skeleton = { if (tv) HomeSkeletonTv() else HomeSkeletonPhone() },
        failed = { LoadFailed(view?.problem, actions.onRefresh) },
    ) {
        if (view != null) {
            if (tv) HomeTv(view, actions) else HomePhone(view, actions)
        }
    }
}

@Composable
fun HomeRoute(navigation: CatalogNavigation) {
    val send = rememberSend()
    val view by rememberSurface<HomeView>(Surface.Home, open = true)
    HomeScreen(
        view,
        HomeActions(
            navigation = navigation,
            onListChange = { id, listed -> send(Event.WatchlistChanged(WatchlistChange(id, listed))) },
            onRefresh = { send(Event.RefreshRequested(Surface.Home)) },
        ),
    )
}
