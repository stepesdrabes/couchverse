package io.stepes.couchverse.accounts

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import io.stepes.couchverse.accounts.phone.SignInPhone
import io.stepes.couchverse.accounts.tv.SignInTv
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.PairingView
import io.stepes.couchverse.core.PasswordSignIn
import io.stepes.couchverse.core.Server
import io.stepes.couchverse.core.ServerRef
import io.stepes.couchverse.core.ServersView
import io.stepes.couchverse.core.SignInView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.runtime.LocalCoreRuntime
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv
import kotlinx.coroutines.delay

/** What the sign-in screen shows; [signIn] only counts when it is about [server]. */
class SignInState(
    val server: Server?,
    signIn: SignInView?,
    /** Seconds until the pairing code expires. */
    val remainingSeconds: Long,
    val username: String = "",
) {
    val signIn: SignInView? = signIn?.takeIf { it.serverId == null || it.serverId == server?.id }
    val pairing: PairingView? get() = signIn?.pairing
}

class SignInActions(
    val onPassword: (username: String, password: String) -> Unit,
    val onStartPairing: () -> Unit,
    val onCancelPairing: () -> Unit,
    val onBack: (() -> Unit)?,
)

/**
 * Signing in to a server: a password form, and pairing with another device (a code and a QR
 * code it approves). The TV leads with pairing, since typing on a remote is slow.
 */
@Composable
fun SignInScreen(state: SignInState, actions: SignInActions) {
    if (LocalIsTv.current) SignInTv(state, actions) else SignInPhone(state, actions)
}

/** [SignInScreen] over the core. The TV starts pairing as soon as it shows. */
@Composable
fun SignInRoute(serverId: String, username: String, onBack: (() -> Unit)?) {
    val runtime = LocalCoreRuntime.current
    val send = rememberSend()
    val servers by rememberSurface<ServersView>(Surface.Servers)
    val signIn by rememberSurface<SignInView>(Surface.SignIn)
    val tv = LocalIsTv.current
    DisposableEffect(serverId) {
        val first = PairingScreens.open()
        if (tv && first) send(Event.PairingStarted(ServerRef(serverId)))
        onDispose { if (PairingScreens.close()) send(Event.PairingCancelled) }
    }
    val remaining = rememberCountdown(signIn?.pairing?.expiresAtMs?.toLong(), runtime::nowMs)
    SignInScreen(
        SignInState(servers?.servers?.firstOrNull { it.id == serverId }, signIn, remaining, username),
        SignInActions(
            onPassword = { user, password -> send(Event.PasswordSignInSubmitted(PasswordSignIn(serverId, user, password))) },
            onStartPairing = { send(Event.PairingStarted(ServerRef(serverId))) },
            onCancelPairing = { send(Event.PairingCancelled) },
            onBack = onBack,
        ),
    )
}

/**
 * Sign-in screens on screen right now. A navigation that replaces one with another composes the
 * new screen before it disposes the old one, so pairing starts with the first and is cancelled
 * with the last, never by the screen that is going away.
 */
private object PairingScreens {
    private var open = 0

    /** Returns whether this is the only one. */
    fun open(): Boolean = ++open == 1

    /** Returns whether none is left. */
    fun close(): Boolean = --open == 0
}

/** Whole seconds left until [deadlineMs] on the core's monotonic [clock], ticking each second. */
@Composable
fun rememberCountdown(deadlineMs: Long?, clock: () -> Long): Long {
    var remaining by remember(deadlineMs) { mutableLongStateOf(secondsLeft(deadlineMs, clock())) }
    LaunchedEffect(deadlineMs) {
        while (deadlineMs != null && remaining > 0) {
            val now = clock()
            remaining = secondsLeft(deadlineMs, now)
            // wake on the next whole second, so the display never skips one
            delay(((deadlineMs - now) % 1000).let { if (it <= 0) 1000 else it })
        }
        remaining = secondsLeft(deadlineMs, clock())
    }
    return remaining
}

internal fun secondsLeft(deadlineMs: Long?, now: Long): Long =
    if (deadlineMs == null) 0 else ((deadlineMs - now + 999) / 1000).coerceAtLeast(0)

/** "9:41" for a countdown. */
fun formatCountdown(seconds: Long): String = "%d:%02d".format(seconds / 60, seconds % 60)

/** The pairing page without the code, as people type it: `https://media.example.com/pair`. */
fun pairingPage(verifyUrl: String): String = verifyUrl.substringBefore('?')
