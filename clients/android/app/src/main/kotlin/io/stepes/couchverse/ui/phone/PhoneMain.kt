package io.stepes.couchverse.ui.phone

import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import io.stepes.couchverse.accounts.AccountSwitcherRoute
import io.stepes.couchverse.catalog.CatalogNavigation
import io.stepes.couchverse.catalog.phone.BrowseHubPhone
import io.stepes.couchverse.core.AccountsView
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.Motion
import io.stepes.couchverse.design.theme.sharedAvatar
import io.stepes.couchverse.downloads.DownloadsRoute
import io.stepes.couchverse.navigation.Account
import io.stepes.couchverse.navigation.Browse
import io.stepes.couchverse.navigation.Downloads
import io.stepes.couchverse.navigation.Home
import io.stepes.couchverse.navigation.Leaderboard
import io.stepes.couchverse.navigation.MyList
import io.stepes.couchverse.navigation.Profile
import io.stepes.couchverse.navigation.Search
import io.stepes.couchverse.settings.SettingsRoute
import io.stepes.couchverse.ui.Page
import io.stepes.couchverse.ui.RootActions
import io.stepes.couchverse.ui.browseScreens
import io.stepes.couchverse.ui.detailScreens
import io.stepes.couchverse.ui.isTop
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut

private class Tab(val route: Any, val label: Int, val icon: ImageVector?)

/** Tabs along the bottom (Home, Browse, Search, My List, the account), hidden on detail screens. */
@Composable
internal fun PhoneMain(nav: NavHostController, catalog: CatalogNavigation, accountId: String, version: String, root: RootActions) {
    val session by rememberSurface<SessionView>(Surface.Session)
    if (session?.offline == true) {
        // with the server out of reach, what is on the phone is all there is to watch
        DownloadsRoute(onPlay = root.onPlayDownload, onBack = null)
        return
    }
    var switching by remember { mutableStateOf(false) }
    val accounts by rememberSurface<AccountsView>(Surface.Accounts)
    val current = accounts?.accounts?.firstOrNull { it.id == accountId }
    val entry by nav.currentBackStackEntryAsState()
    val tabs = remember {
        listOf(
            Tab(Home, R.string.nav_home, Icons.Filled.Home),
            Tab(Browse, R.string.nav_browse, CouchverseIcons.Movie),
            Tab(Search, R.string.nav_search, Icons.Filled.Search),
            Tab(MyList, R.string.nav_my_list, CouchverseIcons.Bookmark),
            Tab(Account, R.string.nav_profile, null),
        )
    }
    val destination = entry?.destination
    val onTab = destination == null || tabs.any { destination.isTop(it.route) }
    val fade = Motion.standard<Float>()
    Scaffold(
        containerColor = Tokens.Palette.bg,
        contentWindowInsets = WindowInsets(0.dp),
        bottomBar = {
            if (onTab) {
                NavigationBar(containerColor = Tokens.Palette.surface) {
                    tabs.forEach { tab ->
                        NavigationBarItem(
                            selected = destination.isTop(tab.route),
                            onClick = {
                                nav.navigate(tab.route) {
                                    popUpTo(nav.graph.findStartDestination().id) { saveState = true }
                                    launchSingleTop = true
                                    restoreState = true
                                }
                            },
                            icon = {
                                if (tab.icon != null) {
                                    Icon(tab.icon, contentDescription = null)
                                } else {
                                    Avatar(current?.avatarUrl, seed = current?.username.orEmpty(), size = 26.dp, modifier = Modifier.sharedAvatar(accountId))
                                }
                            },
                            label = { Text(stringResource(tab.label)) },
                        )
                    }
                }
            }
        },
    ) { padding ->
        NavHost(
            navController = nav,
            startDestination = Home,
            modifier = Modifier.padding(bottom = padding.calculateBottomPadding()),
            enterTransition = { fadeIn(fade) },
            exitTransition = { fadeOut(fade) },
        ) {
            browseScreens(catalog)
            composable<Browse> { Page { BrowseHubPhone(catalog) } }
            composable<Account> {
                SettingsRoute(
                    version = version,
                    onSwitchAccount = { switching = true },
                    onAddAccount = root.onAddAccount,
                    onAddServer = root.onAddServer,
                    onDevices = root.onDevices,
                    onApprove = root.onApprove,
                    onDownloads = { nav.navigate(Downloads) },
                    onJoinCouch = root.onJoinCouch,
                    onProfile = { nav.navigate(Profile(it)) },
                    onLeaderboard = { nav.navigate(Leaderboard) },
                )
            }
            composable<Downloads> { DownloadsRoute(onPlay = root.onPlayDownload, onBack = { nav.popBackStack() }) }
            detailScreens(nav, catalog)
        }
    }
    if (switching) {
        AccountSwitcherRoute(onDismiss = { switching = false }, onSignInAgain = root.onSignInAgain, onAdd = root.onAddAccount)
    }
}
