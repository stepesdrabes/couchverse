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
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
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

class JoinCouchActions(
    val onJoin: (String) -> Unit,
    /** Steer this account's own player on another device instead of watching here. */
    val onRemote: (String) -> Unit,
    val onScan: (() -> Unit)?,
    val onBack: () -> Unit,
)

/**
 * Joining a couch session by its six-digit code, which a link or a scanned QR code fills in.
 * [view] says whether a join is under way or failed.
 */
@Composable
fun JoinCouchScreen(view: CouchView?, initialCode: String, actions: JoinCouchActions) {
    if (LocalIsTv.current) JoinCouchTv(view, initialCode, actions) else JoinCouchPhone(view, initialCode, actions)
}

/**
 * The heading, the hint and the code field both idioms show. [field] is what the idiom adds to
 * the field and [buttons] its ways on, given the digits typed and whether they can be sent.
 */
@Composable
internal fun ColumnScope.CodeForm(
    view: CouchView?,
    initialCode: String,
    onJoin: (String) -> Unit,
    field: @Composable (code: String) -> Modifier,
    buttons: @Composable ColumnScope.(digits: String, ready: Boolean) -> Unit,
) {
    var code by rememberSaveable { mutableStateOf(initialCode) }
    val digits = code.filter(Char::isDigit)
    val ready = digits.length == 6 && view?.status != CouchStatus.Connecting
    val failed = view?.status == CouchStatus.Ended && view.problem != null
    Text(
        stringResource(R.string.couch_join_title),
        style = MaterialTheme.typography.headlineSmall,
        color = Tokens.Palette.text,
        modifier = Modifier.semantics { heading() },
    )
    Text(stringResource(R.string.couch_join_hint), style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.muted)
    OutlinedTextField(
        value = code,
        onValueChange = { code = it.filter(Char::isDigit).take(6) },
        label = { Text(stringResource(R.string.couch_join_code)) },
        singleLine = true,
        isError = failed,
        supportingText = if (failed) {
            { Text(view.problem?.let { problemMessage(it) } ?: stringResource(R.string.couch_join_failed)) }
        } else {
            null
        },
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword, imeAction = ImeAction.Go),
        keyboardActions = KeyboardActions(onGo = { if (ready) onJoin(digits) }),
        modifier = Modifier.fillMaxWidth().then(field(code)),
    )
    buttons(digits, ready)
}

/** [JoinCouchScreen] over the core; the root moves on to the player or the remote once joined. */
@Composable
fun JoinCouchRoute(initialCode: String, onScan: (() -> Unit)?, onBack: () -> Unit) {
    val send = rememberSend()
    val view by rememberSurface<CouchView>(Surface.Couch)
    JoinCouchScreen(
        view,
        initialCode,
        JoinCouchActions(
            onJoin = { send(Event.CouchJoinRequested(CouchCode(it))) },
            onRemote = { send(Event.CouchRemoteRequested(CouchCode(it))) },
            onScan = onScan,
            onBack = onBack,
        ),
    )
}

/**
 * The six-digit code in a couch link: `couchverse://couch/123456`, or the join page a host's QR
 * code shows (`https://media.example.com/couch/123456`).
 */
fun couchCode(url: String): String? {
    val match = Regex("^(?:couchverse://|https?://[^/]+/)couch/(\\d{6})/?(?:[?#].*)?$").find(url.trim()) ?: return null
    return match.groupValues[1]
}
