package io.stepes.couchverse.ui

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraphBuilder
import androidx.navigation.NavHostController
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import io.stepes.couchverse.catalog.BrowseRoute
import io.stepes.couchverse.catalog.CatalogNavigation
import io.stepes.couchverse.catalog.GenresRoute
import io.stepes.couchverse.catalog.HomeRoute
import io.stepes.couchverse.catalog.MyListRoute
import io.stepes.couchverse.catalog.SearchRoute
import io.stepes.couchverse.catalog.TitleRoute
import io.stepes.couchverse.core.TitleKind
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.navigation.Genre
import io.stepes.couchverse.navigation.Title
import io.stepes.couchverse.ui.phone.PhoneMain
import io.stepes.couchverse.ui.tv.TvMain

/**
 * The signed-in app for one account: tabs on phones, a sidebar on TV, over one nested
 * navigation of catalog screens. [openTitle] is a title a `couchverse://title/...` link asked for.
 */
@Composable
fun MainScreen(accountId: String, version: String, openTitle: String?, onTitleOpened: () -> Unit, root: RootActions) {
    val nav = rememberNavController()
    val catalog = remember(nav) {
        CatalogNavigation(
            openTitle = { nav.navigate(Title(it)) },
            play = root.onPlay,
            openGenre = { name, label -> nav.navigate(Genre(name, label)) },
        )
    }
    LaunchedEffect(openTitle) {
        if (openTitle != null) {
            nav.navigate(Title(openTitle))
            onTitleOpened()
        }
    }
    if (LocalIsTv.current) TvMain(nav, catalog, accountId, version, root) else PhoneMain(nav, catalog, accountId, version, root)
}

/** The screens every idiom reaches from anywhere: a title and a genre. */
fun NavGraphBuilder.detailScreens(nav: NavHostController, catalog: CatalogNavigation) {
    val back: () -> Unit = { nav.popBackStack() }
    composable<Title> { entry -> TitleRoute(entry.toRoute<Title>().slug, catalog, onBack = back) }
    composable<Genre> { entry ->
        val genre = entry.toRoute<Genre>()
        Page { BrowseRoute(kind = null, genre = genre.name, label = genre.label, navigation = catalog, onBack = back) }
    }
}

fun NavGraphBuilder.browseScreens(catalog: CatalogNavigation) {
    composable<io.stepes.couchverse.navigation.Movies> {
        Page { BrowseRoute(TitleKind.Movie, null, stringResource(R.string.nav_movies), catalog) }
    }
    composable<io.stepes.couchverse.navigation.Series> {
        Page { BrowseRoute(TitleKind.Series, null, stringResource(R.string.nav_series), catalog) }
    }
    composable<io.stepes.couchverse.navigation.Genres> { Page { GenresRoute(catalog) } }
    composable<io.stepes.couchverse.navigation.MyList> { Page { MyListRoute(catalog) } }
    composable<io.stepes.couchverse.navigation.Search> { Page { SearchRoute(catalog) } }
    composable<io.stepes.couchverse.navigation.Home> { HomeRoute(catalog) }
}

/** A screen below the status bar (phones draw edge to edge; the home hero goes under it). */
@Composable
fun Page(content: @Composable () -> Unit) {
    Box(Modifier.fillMaxSize().then(if (LocalIsTv.current) Modifier else Modifier.statusBarsPadding())) { content() }
}

/** Whether [destination] is the top-level screen [route]. */
fun androidx.navigation.NavDestination?.isTop(route: Any): Boolean = this?.hasRoute(route::class) == true
