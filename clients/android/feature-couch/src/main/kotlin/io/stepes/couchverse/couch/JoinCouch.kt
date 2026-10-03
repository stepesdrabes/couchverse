package io.stepes.couchverse.couch

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.CouchCode
import io.stepes.couchverse.core.CouchStatus
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.phone.inkButtonColors
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.focusOnStart
import io.stepes.couchverse.design.tv.remoteLeavesField

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
    val tv = LocalIsTv.current
    var code by rememberSaveable { mutableStateOf(initialCode) }
    val digits = code.filter(Char::isDigit)
    val ready = digits.length == 6 && view?.status != CouchStatus.Connecting
    val failed = view?.status == CouchStatus.Ended && view.problem != null
    Box(Modifier.fillMaxSize().safeDrawingPadding(), contentAlignment = Alignment.Center) {
        if (!tv) {
            IconButton(onClick = actions.onBack, modifier = Modifier.align(Alignment.TopStart).padding(8.dp)) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back))
            }
        }
        Column(
            Modifier.widthIn(max = 480.dp).padding(horizontal = if (tv) TvSafe.horizontal else 24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
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
                    { Text(view?.problem?.let { problemMessage(it) } ?: stringResource(R.string.couch_join_failed)) }
                } else {
                    null
                },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword, imeAction = ImeAction.Go),
                keyboardActions = KeyboardActions(onGo = { if (ready) actions.onJoin(digits) }),
                modifier = Modifier
                    .fillMaxWidth()
                    .then(if (tv) Modifier.focusOnStart().remoteLeavesField(code.isEmpty()) else Modifier),
            )
            if (tv) {
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    TvActionButton(stringResource(R.string.couch_join), onClick = { actions.onJoin(digits) }, primary = true, enabled = ready)
                    TvActionButton(stringResource(R.string.common_back), onClick = actions.onBack)
                }
            } else {
                Button(onClick = { actions.onJoin(digits) }, enabled = ready, modifier = Modifier.fillMaxWidth()) {
                    Text(if (view?.status == CouchStatus.Connecting) stringResource(R.string.couch_connecting) else stringResource(R.string.couch_join))
                }
                OutlinedButton(onClick = { actions.onRemote(digits) }, enabled = ready, modifier = Modifier.fillMaxWidth()) {
                    Text(stringResource(R.string.couch_join_remote))
                }
                actions.onScan?.let { scan ->
                    TextButton(onClick = scan, colors = inkButtonColors(), modifier = Modifier.align(Alignment.CenterHorizontally)) {
                        Icon(CouchverseIcons.ScanCode, contentDescription = null)
                        Text(stringResource(R.string.scanner_title), modifier = Modifier.padding(start = 8.dp))
                    }
                }
            }
        }
    }
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
