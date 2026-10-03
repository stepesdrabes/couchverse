package io.stepes.couchverse.ui.tv

import androidx.activity.compose.BackHandler
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Settings
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.tv.material3.DrawerValue
import androidx.tv.material3.Icon
import androidx.tv.material3.ModalNavigationDrawer
import androidx.tv.material3.NavigationDrawerItem
import androidx.tv.material3.Text
import androidx.tv.material3.rememberDrawerState
import io.stepes.couchverse.catalog.CatalogNavigation
import io.stepes.couchverse.core.AccountsView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.Motion
import io.stepes.couchverse.design.theme.sharedAvatar
import io.stepes.couchverse.navigation.Account
import io.stepes.couchverse.navigation.Genres
import io.stepes.couchverse.navigation.Home
import io.stepes.couchverse.navigation.Movies
import io.stepes.couchverse.navigation.MyList
import io.stepes.couchverse.navigation.Search
import io.stepes.couchverse.navigation.Series
import io.stepes.couchverse.settings.SettingsRoute
import io.stepes.couchverse.ui.RootActions
import io.stepes.couchverse.ui.browseScreens
import io.stepes.couchverse.ui.detailScreens
import io.stepes.couchverse.ui.isTop

private class Destination(val route: Any, val label: Int, val icon: ImageVector)

/** Room the collapsed sidebar takes over the content's left edge. */
private val RailWidth = 80.dp

/**
 * The sidebar (plan 10.3, the same on Google TV): the account on top, which goes back to
 * "Who's watching?", then the catalog and Settings. It expands while focus is in it. A picked
 * screen takes focus itself once its first element shows (a screen still loading leaves it
 * here). Back on a screen goes back; on Home it first moves into the sidebar, then leaves the app.
 */
@Composable
internal fun TvMain(nav: NavHostController, catalog: CatalogNavigation, accountId: String, version: String, root: RootActions) {
    val accounts by rememberSurface<AccountsView>(Surface.Accounts)
    val current = accounts?.accounts?.firstOrNull { it.id == accountId }
    val entry by nav.currentBackStackEntryAsState()
    val destination = entry?.destination
    var sidebarFocused by remember { mutableStateOf(false) }
    val destinations = remember {
        listOf(
            Destination(Home, R.string.nav_home, Icons.Filled.Home),
            Destination(Movies, R.string.nav_movies, CouchverseIcons.Movie),
            Destination(Series, R.string.nav_series, CouchverseIcons.Series),
            Destination(Genres, R.string.nav_genres, CouchverseIcons.Genres),
            Destination(MyList, R.string.nav_my_list, CouchverseIcons.Bookmark),
            Destination(Search, R.string.nav_search, Icons.Filled.Search),
            Destination(Account, R.string.nav_settings, Icons.Filled.Settings),
        )
    }
    val items = remember { List(destinations.size) { FocusRequester() } }
    val homeItem = items.first()
    val selected = destinations.indexOfFirst { destination.isTop(it.route) }
    BackHandler(enabled = destination.isTop(Home) && !sidebarFocused) { runCatching { homeItem.requestFocus() } }
    val fade = Motion.standard<Float>()

    ModalNavigationDrawer(
        drawerState = rememberDrawerState(DrawerValue.Closed),
        scrimBrush = Brush.horizontalGradient(listOf(Tokens.Palette.bg.copy(alpha = 0.9f), Color.Transparent)),
        drawerContent = {
            Column(
                Modifier
                    .fillMaxHeight()
                    .padding(12.dp)
                    .onFocusChanged { sidebarFocused = it.hasFocus }
                    // arriving from the content lands on the current destination, not the nearest row
                    .focusProperties { onEnter = { if (selected >= 0) items[selected].requestFocus() } }
                    .focusGroup(),
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                NavigationDrawerItem(
                    selected = false,
                    onClick = root.onWhosWatching,
                    leadingContent = {
                        Avatar(current?.avatarUrl, seed = current?.username.orEmpty(), size = 30.dp, modifier = Modifier.sharedAvatar(accountId))
                    },
                    supportingContent = { Text(stringResource(R.string.accounts_switch)) },
                ) {
                    Text(current?.displayName.orEmpty(), maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                Spacer(Modifier.height(24.dp))
                destinations.forEachIndexed { index, item ->
                    NavigationDrawerItem(
                        selected = destination.isTop(item.route),
                        onClick = {
                            nav.navigate(item.route) {
                                popUpTo(nav.graph.findStartDestination().id) { saveState = true }
                                launchSingleTop = true
                                restoreState = true
                            }
                        },
                        leadingContent = { Icon(item.icon, contentDescription = null) },
                        modifier = Modifier.focusRequester(items[index]),
                    ) {
                        Text(stringResource(item.label))
                    }
                }
            }
        },
    ) {
        Box(Modifier.fillMaxSize().background(Tokens.Palette.bg).padding(start = RailWidth)) {
            NavHost(
                navController = nav,
                startDestination = Home,
                enterTransition = { fadeIn(fade) },
                exitTransition = { fadeOut(fade) },
            ) {
                browseScreens(catalog)
                composable<Account> {
                    SettingsRoute(
                        version = version,
                        onSwitchAccount = root.onWhosWatching,
                        onAddAccount = root.onAddAccount,
                        onAddServer = root.onAddServer,
                        onDevices = root.onDevices,
                        onApprove = root.onApprove,
                        onJoinCouch = root.onJoinCouch,
                    )
                }
                detailScreens(nav, catalog)
            }
        }
    }
}
