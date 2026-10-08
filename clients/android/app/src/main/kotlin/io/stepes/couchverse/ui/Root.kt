package io.stepes.couchverse.ui

import androidx.compose.animation.AnimatedContentScope
import androidx.compose.animation.ExperimentalSharedTransitionApi
import androidx.compose.animation.SharedTransitionLayout
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import io.stepes.couchverse.accounts.AddServerRoute
import io.stepes.couchverse.accounts.ChooseServerRoute
import io.stepes.couchverse.accounts.ConnectingRoute
import io.stepes.couchverse.accounts.SignInRoute
import io.stepes.couchverse.accounts.WelcomeScreen
import io.stepes.couchverse.accounts.WhosWatchingRoute
import io.stepes.couchverse.accounts.isConnectLink
import io.stepes.couchverse.accounts.phone.QrScannerScreen
import io.stepes.couchverse.core.AppPhase
import io.stepes.couchverse.core.AppView
import io.stepes.couchverse.core.CouchRole
import io.stepes.couchverse.core.CouchStatus
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.Link
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.PlayKind
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.core.ServersView
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.core.runtime.CoreRuntime
import io.stepes.couchverse.couch.CouchRemoteRoute
import io.stepes.couchverse.couch.JoinCouchRoute
import io.stepes.couchverse.couch.active
import io.stepes.couchverse.couch.CouchInvite
import io.stepes.couchverse.couch.couchInvite
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.LogoMark
import io.stepes.couchverse.design.runtime.LocalCoreRuntime
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.AccentColors
import io.stepes.couchverse.design.theme.CouchverseTheme
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.theme.LocalReducedMotion
import io.stepes.couchverse.design.theme.LocalRootAnimatedScope
import io.stepes.couchverse.design.theme.LocalSharedTransitionScope
import io.stepes.couchverse.design.theme.Motion
import io.stepes.couchverse.design.theme.ProvideDisplayLanguage
import io.stepes.couchverse.navigation.AddServer
import io.stepes.couchverse.navigation.AppLink
import io.stepes.couchverse.navigation.Approve
import io.stepes.couchverse.navigation.ChooseServer
import io.stepes.couchverse.navigation.Connecting
import io.stepes.couchverse.navigation.CouchRemote
import io.stepes.couchverse.navigation.Devices
import io.stepes.couchverse.navigation.JoinCouch
import io.stepes.couchverse.navigation.Main
import io.stepes.couchverse.navigation.PendingLinks
import io.stepes.couchverse.navigation.Scan
import io.stepes.couchverse.navigation.SignIn
import io.stepes.couchverse.navigation.Splash
import io.stepes.couchverse.navigation.Watch
import io.stepes.couchverse.navigation.WatchCouch
import io.stepes.couchverse.navigation.WatchDownload
import io.stepes.couchverse.navigation.Welcome
import io.stepes.couchverse.navigation.WhosWatching
import io.stepes.couchverse.playback.LocalPlaybackEngine
import io.stepes.couchverse.playback.PlaybackEngine
import io.stepes.couchverse.playback.PlayerRoute
import io.stepes.couchverse.playback.PlayerStart
import io.stepes.couchverse.ranks.CelebrationRoute
import io.stepes.couchverse.settings.ApproveRoute
import io.stepes.couchverse.settings.DevicesRoute
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.filterNotNull
import kotlinx.coroutines.flow.first
import androidx.compose.ui.platform.LocalConfiguration

/**
 * The whole app: the session's accent and display language around a root navigation that
 * follows the core's phase (welcome, sign in, who's watching, the signed-in screens).
 */
@Composable
fun CouchverseRoot(runtime: CoreRuntime, playback: PlaybackEngine?, tv: Boolean, links: PendingLinks, version: String) {
    CompositionLocalProvider(LocalCoreRuntime provides runtime, LocalPlaybackEngine provides playback) {
        val session by rememberSurface<SessionView>(Surface.Session)
        CouchverseTheme(accent = AccentColors.of(session?.accent), tv = tv) {
            ProvideDisplayLanguage(session?.language ?: LocalConfiguration.current.locales[0].language) {
                Box(Modifier.fillMaxSize().background(Tokens.Palette.bg)) {
                    RootNavigation(links, version)
                    Notices()
                    // a guest's couch has its notification too, without an account
                    NotificationPermission()
                    if (session?.status == LoadStatus.Loaded) CelebrationRoute()
                }
            }
        }
    }
}

@OptIn(ExperimentalSharedTransitionApi::class)
@Composable
private fun RootNavigation(links: PendingLinks, version: String) {
    val runtime = LocalCoreRuntime.current
    val tv = LocalIsTv.current
    val nav = rememberNavController()
    val app by rememberSurface<AppView>(Surface.App)
    val servers = rememberSurface<ServersView>(Surface.Servers)
    val phase = app?.phase
    val active = app?.activeAccount
    var pendingTitle by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(phase, active) {
        val target: Any = when (phase) {
            null, AppPhase.Starting -> Splash
            AppPhase.Welcome -> Welcome
            AppPhase.SignIn -> {
                // a connect link carries on by itself once its server checked out
                if (nav.currentBackStackEntry?.isRoute<Connecting>() == true) return@LaunchedEffect
                val known = snapshotFlow { servers.value }.filterNotNull().first().servers
                known.singleOrNull()?.let { SignIn(it.id) } ?: ChooseServer
            }
            AppPhase.ChooseAccount -> WhosWatching
            AppPhase.Ready -> Main(active ?: return@LaunchedEffect)
        }
        nav.navigate(target) {
            popUpTo(nav.graph.id) { inclusive = true }
            launchSingleTop = true
        }
    }

    LaunchedEffect(links.link, phase) {
        when (val link = links.link) {
            is AppLink.Connect -> if (phase != null && phase != AppPhase.Starting) {
                links.consume()
                runtime.send(Event.LinkOpened(Link(link.url)))
                nav.navigate(Connecting)
            }
            is AppLink.Pair -> if (phase == AppPhase.Ready) {
                links.consume()
                runtime.send(Event.LinkOpened(Link(link.url)))
                nav.navigate(Approve(opened = true))
            }
            is AppLink.OpenTitle -> if (phase == AppPhase.Ready) {
                links.consume()
                pendingTitle = link.slug
            }
            is AppLink.Couch -> if (link.opensIn(phase)) {
                links.consume()
                nav.navigate(JoinCouch(link.invite.code, link.invite.server.orEmpty()))
            }
            is AppLink.Play -> if (phase == AppPhase.Ready) {
                links.consume()
                nav.navigate(Watch(link.kind, link.id))
            }
            null -> {}
        }
    }

    val back: () -> Unit = { nav.popBackStack() }
    CouchNavigation(nav)
    val reduced = LocalReducedMotion.current
    val enter = Motion.enter<Float>()
    val rise = Motion.smooth<IntOffset>()
    SharedTransitionLayout {
        CompositionLocalProvider(LocalSharedTransitionScope provides this) {
            NavHost(
                navController = nav,
                startDestination = Splash,
                enterTransition = {
                    // the signed-in screens rise from the dark behind "Who's watching?" (plan 12.3)
                    if (targetState.isRoute<Main>() && !reduced) {
                        fadeIn(enter) + slideInVertically(rise) { it / 12 }
                    } else {
                        fadeIn(enter)
                    }
                },
                exitTransition = { fadeOut(enter) },
                popEnterTransition = { fadeIn(enter) },
                popExitTransition = { fadeOut(enter) },
            ) {
                screen<Splash> { SplashScreen() }
                screen<Welcome> {
                    WelcomeScreen(
                        onAddServer = { nav.navigate(AddServer) },
                        onScan = { nav.navigate(Scan()) },
                        onJoinCouch = { nav.navigate(JoinCouch()) },
                    )
                }
                screen<AddServer> {
                    AddServerRoute(
                        onAdded = { id ->
                            nav.navigate(SignIn(id)) {
                                popUpTo<AddServer> { inclusive = true }
                                launchSingleTop = true
                            }
                        },
                        onBack = back.takeIf { nav.previousBackStackEntry != null },
                        onScan = { nav.navigate(Scan()) },
                    )
                }
                screen<Scan> { entry ->
                    val approve = entry.toRoute<Scan>().approve
                    QrScannerScreen(
                        hint = stringResource(if (approve) R.string.scanner_hint else R.string.onboarding_scan_hint),
                        onLink = { url ->
                            val couch = couchInvite(url)
                            if (couch == null) runtime.send(Event.LinkOpened(Link(url)))
                            val next: Any = when {
                                couch != null -> JoinCouch(couch.code, couch.server.orEmpty())
                                isConnectLink(url) -> Connecting
                                else -> Approve(opened = true)
                            }
                            nav.navigate(next) { popUpTo<Scan> { inclusive = true } }
                        },
                        onBack = back,
                    )
                }
                screen<SignIn> { entry ->
                    val route = entry.toRoute<SignIn>()
                    SignInRoute(route.serverId, route.username, onBack = back.takeIf { nav.previousBackStackEntry != null })
                }
                screen<ChooseServer> {
                    ChooseServerRoute(onPick = { nav.navigate(SignIn(it.id)) }, onAddServer = { nav.navigate(AddServer) })
                }
                screen<Connecting> {
                    ConnectingRoute(onBack = { if (!nav.popBackStack()) nav.navigate(Welcome) })
                }
                screen<WhosWatching> {
                    WhosWatchingRoute(
                        onSignInAgain = { nav.navigate(SignIn(it.serverId, it.username)) },
                        onAdd = { nav.navigate(ChooseServer) },
                        resume = { account ->
                            // picking the account that is already on just goes back to it
                            (phase == AppPhase.Ready && account.id == active).also { same ->
                                if (same) nav.navigate(Main(account.id)) { popUpTo(nav.graph.id) { inclusive = true } }
                            }
                        },
                    )
                }
                screen<Main> { entry ->
                    val title = pendingTitle
                    MainScreen(
                        accountId = entry.toRoute<Main>().accountId,
                        version = version,
                        openTitle = title,
                        onTitleOpened = { pendingTitle = null },
                        root = RootActions(
                            onWhosWatching = { nav.navigate(WhosWatching) },
                            onAddAccount = { nav.navigate(ChooseServer) },
                            onAddServer = { nav.navigate(AddServer) },
                            onSignInAgain = { nav.navigate(SignIn(it.serverId, it.username)) },
                            onDevices = { nav.navigate(Devices) },
                            onApprove = { nav.navigate(Approve()) },
                            onPlay = { nav.navigate(Watch(it.kind.string, it.id)) },
                            onPlayDownload = { nav.navigate(WatchDownload(it)) },
                            onJoinCouch = { nav.navigate(JoinCouch()) },
                        ),
                    )
                }
                screen<Approve> { entry ->
                    ApproveRoute(
                        opened = entry.toRoute<Approve>().opened,
                        onScan = if (tv) null else ({ nav.navigate(Scan(approve = true)) }),
                        onDone = back,
                    )
                }
                screen<Devices> { DevicesRoute(onBack = back) }
                screen<Watch> { entry ->
                    val route = entry.toRoute<Watch>()
                    val kind = PlayKind.entries.first { it.string == route.kind }
                    PlayerRoute(PlayerStart.Title(PlayTarget(kind, route.id)), onBack = back)
                }
                screen<WatchDownload> { entry ->
                    PlayerRoute(PlayerStart.Download(entry.toRoute<WatchDownload>().id), onBack = back)
                }
                screen<WatchCouch> {
                    PlayerRoute(
                        PlayerStart.Couch,
                        onBack = {
                            runtime.send(Event.CouchLeft)
                            back()
                        },
                    )
                }
                screen<JoinCouch> { entry ->
                    val route = entry.toRoute<JoinCouch>()
                    JoinCouchRoute(
                        invite = CouchInvite(route.code, route.server.ifEmpty { null }),
                        onScan = if (tv) null else ({ nav.navigate(Scan()) }),
                        onBack = back,
                    )
                }
                screen<CouchRemote> { CouchRemoteRoute(onLeft = back) }
            }
        }
    }
}

/**
 * Follows this device's couch role: a follower watches in the player, a remote steers from
 * the remote screen (as does a host whose account went on hosting on another device), and
 * both come back a moment after the session ends. A guest without an account follows the
 * same way.
 */
@Composable
private fun CouchNavigation(nav: androidx.navigation.NavHostController) {
    val couch by rememberSurface<CouchView>(Surface.Couch)
    val view = couch ?: return
    LaunchedEffect(view.role, view.status) {
        val here = nav.currentBackStackEntry
        val target: Any? = when (view.role) {
            CouchRole.Follower -> WatchCouch
            CouchRole.Remote -> CouchRemote
            else -> null
        }
        when {
            view.status == CouchStatus.Ended -> if (here?.isRoute<WatchCouch>() == true || here?.isRoute<CouchRemote>() == true) {
                delay(ENDED_NOTE_MS)
                nav.popBackStack()
            }
            target != null && view.active && here?.isRoute<WatchCouch>() != true && here?.isRoute<CouchRemote>() != true ->
                nav.navigate(target) {
                    // the remote stands in for a player the core closed when the account went on
                    // hosting elsewhere, so leaving it does not open that title again here
                    if (view.role == CouchRole.Remote && here?.isRoute<Watch>() == true) {
                        popUpTo<Watch> { inclusive = true }
                    } else {
                        popUpTo<JoinCouch> { inclusive = true }
                    }
                }
        }
    }
}

/** How long the end of a couch session stays on screen. */
private const val ENDED_NOTE_MS = 3_000L

/** What the signed-in screens ask of the root navigation. */
class RootActions(
    val onWhosWatching: () -> Unit,
    val onAddAccount: () -> Unit,
    val onAddServer: () -> Unit,
    val onSignInAgain: (io.stepes.couchverse.core.AccountCard) -> Unit,
    val onDevices: () -> Unit,
    val onApprove: () -> Unit,
    val onPlay: (PlayTarget) -> Unit,
    val onPlayDownload: (String) -> Unit,
    val onJoinCouch: () -> Unit,
)

/** A root destination that tells its content which animated scope it is in, for shared avatars. */
private inline fun <reified T : Any> NavGraphBuilder.screen(
    noinline content: @Composable AnimatedContentScope.(NavBackStackEntry) -> Unit,
) {
    composable<T> { entry ->
        CompositionLocalProvider(LocalRootAnimatedScope provides this) { content(entry) }
    }
}

private inline fun <reified T : Any> NavBackStackEntry.isRoute(): Boolean = destination.hasRoute(T::class)

@Composable
private fun SplashScreen() {
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { LogoMark(size = 72.dp) }
}

