package io.stepes.couchverse.navigation

import kotlinx.serialization.Serializable

// The root screens: one per app phase, plus the flows that sit above the main screens.

@Serializable
object Splash

@Serializable
object Welcome

@Serializable
object AddServer

/** The phone's camera; [approve] scans another device's pairing code instead of a connect code. */
@Serializable
data class Scan(val approve: Boolean = false)

@Serializable
data class SignIn(val serverId: String, val username: String = "")

@Serializable
object ChooseServer

@Serializable
object Connecting

@Serializable
object WhosWatching

/** The signed-in app; keyed by account so a switch starts on a fresh home. */
@Serializable
data class Main(val accountId: String)

/** Approving another device; [opened] when a link already asked the core for the request. */
@Serializable
data class Approve(val opened: Boolean = false)

@Serializable
object Devices

// The main screens, inside Main.

@Serializable
object Home

/** The phone's Movies, Series and Genres tabs. */
@Serializable
object Browse

@Serializable
object Movies

@Serializable
object Series

@Serializable
object Genres

@Serializable
object MyList

@Serializable
object Search

/** Settings; on phones also the account (profile) tab. */
@Serializable
object Account

@Serializable
data class Title(val slug: String)

@Serializable
data class Genre(val name: String, val label: String)

@Serializable
data class Play(val kind: String, val id: String)
