package io.stepes.couchverse.accounts

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import io.stepes.couchverse.accounts.phone.AddServerPhone
import io.stepes.couchverse.accounts.phone.WelcomePhone
import io.stepes.couchverse.accounts.tv.AddServerTv
import io.stepes.couchverse.accounts.tv.WelcomeTv
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.ServerAddress
import io.stepes.couchverse.core.ServersView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv

/** The first screen without a server: what Couchverse is, and how to add one. */
@Composable
fun WelcomeScreen(onAddServer: () -> Unit, onScan: () -> Unit) {
    if (LocalIsTv.current) WelcomeTv(onAddServer) else WelcomePhone(onAddServer, onScan)
}

/** Adding a server by address; [onScan] is the phone's way in with a "Connect a device" QR. */
@Composable
fun AddServerScreen(
    view: ServersView?,
    onSubmit: (String) -> Unit,
    onBack: (() -> Unit)?,
    onScan: () -> Unit,
) {
    if (LocalIsTv.current) AddServerTv(view, onSubmit, onBack) else AddServerPhone(view, onSubmit, onBack, onScan)
}

/** [AddServerScreen] over the core; moves on with the new server's id once it checked out. */
@Composable
fun AddServerRoute(onAdded: (serverId: String) -> Unit, onBack: (() -> Unit)?, onScan: () -> Unit) {
    val send = rememberSend()
    val view by rememberSurface<ServersView>(Surface.Servers)
    // the view keeps the last success, so only an address submitted here moves on
    var submitted by rememberSaveable { mutableStateOf(false) }
    val add = view?.add
    LaunchedEffect(add) {
        val added = add?.added
        if (submitted && add?.status == LoadStatus.Loaded && added != null) {
            submitted = false
            onAdded(added)
        }
    }
    AddServerScreen(
        view = view?.takeIf { submitted || it.add.status != LoadStatus.Loaded },
        onSubmit = { address ->
            submitted = true
            send(Event.ServerAddressSubmitted(ServerAddress(address)))
        },
        onBack = onBack,
        onScan = onScan,
    )
}
