package io.stepes.couchverse.couch

import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import io.stepes.couchverse.core.AppPhase
import io.stepes.couchverse.core.AppView
import io.stepes.couchverse.core.CouchCode
import io.stepes.couchverse.core.CouchStatus
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.couch.phone.JoinCouchPhone
import io.stepes.couchverse.couch.tv.JoinCouchTv
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.design.theme.LocalIsTv
import java.net.URI
import java.net.URLDecoder

class JoinCouchActions(
    /** Joins as a viewer: through the account on its own server, as a guest on any other. */
    val onJoin: (CouchCode) -> Unit,
    /** Steer this account's own player on another device instead of watching here. */
    val onRemote: (String) -> Unit,
    val onScan: (() -> Unit)?,
    val onBack: () -> Unit,
)

/** A couch to join: its code, and the server it is on when a link or a join page named one. */
data class CouchInvite(val code: String = "", val server: String? = null)

/**
 * Joining a couch session by its six-digit code, which a link or a scanned QR code fills in
 * (with the server they name). Without an account, a [guest] also gives the server's address
 * and joins without one. [view] says whether a join is under way or failed.
 */
@Composable
fun JoinCouchScreen(view: CouchView?, invite: CouchInvite, guest: Boolean, actions: JoinCouchActions) {
    if (LocalIsTv.current) JoinCouchTv(view, invite, guest, actions) else JoinCouchPhone(view, invite, guest, actions)
}

/** The form as the idiom's buttons see it: whether it can be sent, and the ways to send it. */
internal class CodeEntry(val ready: Boolean, val joining: Boolean, val join: () -> Unit, val remote: () -> Unit)

/**
 * The heading, the hint, the server's address for a [guest], and the code: what both idioms
 * show. [field] is what the idiom adds to a field, given its text and whether focus starts
 * there; [buttons] are its ways on.
 */
@Composable
internal fun ColumnScope.CodeForm(
    view: CouchView?,
    invite: CouchInvite,
    guest: Boolean,
    actions: JoinCouchActions,
    field: @Composable (value: String, start: Boolean) -> Modifier,
    buttons: @Composable ColumnScope.(CodeEntry) -> Unit,
) {
    var code by rememberSaveable { mutableStateOf(invite.code) }
    var server by rememberSaveable { mutableStateOf(invite.server.orEmpty()) }
    // this form asked to join, so the core's couch view describes its attempt
    var attempted by rememberSaveable { mutableStateOf(false) }
    val startOnServer = remember { guest && invite.server.isNullOrBlank() }
    val digits = code.filter(Char::isDigit)
    val address = server.trim()
    val joining = attempted && view?.status == CouchStatus.Connecting && view.role == null
    val problem = view?.problem?.takeIf { attempted && view.status == CouchStatus.Idle }
    val ready = digits.length == 6 && !joining && !(guest && address.isEmpty())
    val entry = CodeEntry(
        ready = ready,
        joining = joining,
        join = {
            if (ready) {
                attempted = true
                actions.onJoin(CouchCode(digits, address.ifEmpty { null }))
            }
        },
        remote = {
            if (ready) {
                attempted = true
                actions.onRemote(digits)
            }
        },
    )
    Text(
        stringResource(R.string.couch_join_title),
        style = MaterialTheme.typography.headlineSmall,
        color = Tokens.Palette.text,
        modifier = Modifier.semantics { heading() },
    )
    Text(
        stringResource(if (guest) R.string.couch_join_guest_hint else R.string.couch_join_hint),
        style = MaterialTheme.typography.bodyMedium,
        color = Tokens.Palette.muted,
    )
    if (guest) {
        OutlinedTextField(
            value = server,
            onValueChange = { server = it },
            label = { Text(stringResource(R.string.servers_address_label)) },
            placeholder = { Text(stringResource(R.string.servers_address_placeholder)) },
            singleLine = true,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Next, autoCorrectEnabled = false),
            modifier = Modifier.fillMaxWidth().then(field(server, startOnServer)),
        )
    }
    OutlinedTextField(
        value = code,
        onValueChange = { code = it.filter(Char::isDigit).take(6) },
        label = { Text(stringResource(R.string.couch_join_code)) },
        singleLine = true,
        isError = problem != null,
        supportingText = problem?.let { { Text(problemMessage(it)) } },
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword, imeAction = ImeAction.Go),
        keyboardActions = KeyboardActions(onGo = { entry.join() }),
        modifier = Modifier.fillMaxWidth().then(field(code, !startOnServer)),
    )
    buttons(entry)
}

/** [JoinCouchScreen] over the core; the root moves on to the player or the remote once joined. */
@Composable
fun JoinCouchRoute(invite: CouchInvite, onScan: (() -> Unit)?, onBack: () -> Unit) {
    val send = rememberSend()
    val view by rememberSurface<CouchView>(Surface.Couch)
    val app by rememberSurface<AppView>(Surface.App)
    JoinCouchScreen(
        view,
        invite,
        guest = app?.phase != AppPhase.Ready,
        JoinCouchActions(
            onJoin = { send(Event.CouchJoinRequested(it)) },
            onRemote = { send(Event.CouchRemoteRequested(CouchCode(it))) },
            onScan = onScan,
            onBack = onBack,
        ),
    )
}

/**
 * The couch a link names: `couchverse://couch/123456`, with the server it is on as `?server=`
 * (the web's "Open in the app"), or the join page a host's QR code shows
 * (`https://media.example.com/couch/123456`), whose server is its origin.
 */
fun couchInvite(url: String): CouchInvite? {
    val uri = runCatching { URI(url.trim()) }.getOrNull() ?: return null
    val path = uri.path.orEmpty().split('/').filter { it.isNotEmpty() }
    val (parts, server) = when (uri.scheme?.lowercase()) {
        "couchverse" -> listOfNotNull(uri.host) + path to queryParameter(uri.rawQuery, "server")
        "http", "https" -> path to uri.rawAuthority?.let { "${uri.scheme.lowercase()}://$it" }
        else -> return null
    }
    val code = parts.takeIf { it.size == 2 && it[0].equals("couch", ignoreCase = true) }?.get(1)
    return code?.takeIf { it.length == 6 && it.all(Char::isDigit) }?.let { CouchInvite(it, server) }
}

private fun queryParameter(query: String?, name: String): String? =
    query?.split('&')
        ?.firstOrNull { it.substringBefore('=') == name }
        ?.substringAfter('=', "")
        ?.let { URLDecoder.decode(it, "UTF-8") }
        ?.takeIf { it.isNotBlank() }
