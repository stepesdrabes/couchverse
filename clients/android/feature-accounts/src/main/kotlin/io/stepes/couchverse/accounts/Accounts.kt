package io.stepes.couchverse.accounts

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.graphics.Color
import io.stepes.couchverse.design.components.Identicon
import io.stepes.couchverse.accounts.phone.AccountPickerPhone
import io.stepes.couchverse.accounts.phone.AccountSwitcherSheet
import io.stepes.couchverse.accounts.tv.WhosWatchingTv
import io.stepes.couchverse.core.AccountCard
import io.stepes.couchverse.core.AccountRef
import io.stepes.couchverse.core.AccountsView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.theme.colorOf

/**
 * "Who's watching?": the TV's first screen whenever accounts exist (D10) and the phone's when
 * none is active. A signed-out account goes back to signing in rather than being picked.
 */
@Composable
fun WhosWatchingScreen(view: AccountsView?, onPick: (AccountCard) -> Unit, onAdd: () -> Unit) {
    if (LocalIsTv.current) WhosWatchingTv(view, onPick, onAdd) else AccountPickerPhone(view, onPick, onAdd)
}

/** [resume] gets the pick first and returns true when it handled it (the account already on). */
@Composable
fun WhosWatchingRoute(onSignInAgain: (AccountCard) -> Unit, onAdd: () -> Unit, resume: (AccountCard) -> Boolean = { false }) {
    val send = rememberSend()
    val view by rememberSurface<AccountsView>(Surface.Accounts)
    WhosWatchingScreen(
        view = view,
        onPick = { account -> if (!resume(account)) pick(account, send, onSignInAgain) },
        onAdd = onAdd,
    )
}

/** The phone's account switcher, a sheet over the main screens. */
@Composable
fun AccountSwitcherRoute(onDismiss: () -> Unit, onSignInAgain: (AccountCard) -> Unit, onAdd: () -> Unit) {
    val send = rememberSend()
    val view by rememberSurface<AccountsView>(Surface.Accounts)
    AccountSwitcherSheet(
        view = view,
        onPick = { account ->
            onDismiss()
            if (account.id != view?.active || !account.signedIn) pick(account, send, onSignInAgain)
        },
        onAdd = {
            onDismiss()
            onAdd()
        },
        onDismiss = onDismiss,
    )
}

private fun pick(account: AccountCard, send: (Event) -> Unit, onSignInAgain: (AccountCard) -> Unit) {
    if (account.signedIn) send(Event.AccountSelected(AccountRef(account.id))) else onSignInAgain(account)
}

/**
 * The colour an account tints "Who's watching?" with: its banner's accent, or without one the
 * hue of its identicon, so it matches the placeholder avatar.
 */
fun accountTint(account: AccountCard): Color = colorOf(account.accent?.accent) ?: Identicon.colorOf(account.username)

/** An account's colour for text on the app's surfaces: its banner's ink, else its tint. */
fun accountInk(account: AccountCard): Color = colorOf(account.accent?.ink) ?: accountTint(account)
