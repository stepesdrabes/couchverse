package io.stepes.couchverse.settings

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.ApprovalOutcome
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.PairingApproval
import io.stepes.couchverse.core.PairingApprovalView
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.core.UserCode
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.remoteLeavesField

class ApproveActions(
    val onLookup: (code: String) -> Unit,
    val onApprove: (code: String, deviceName: String) -> Unit,
    val onDeny: (code: String) -> Unit,
    /** Back to entering a code. */
    val onAnother: () -> Unit,
    val onScan: (() -> Unit)?,
    val onDone: () -> Unit,
)

/**
 * Approving another device's sign-in: its code (typed, or scanned from its screen), then who is
 * asking, then the outcome. [view] is `null` until a code was looked up from here.
 */
@Composable
fun ApproveScreen(view: PairingApprovalView?, userName: String, actions: ApproveActions) {
    val tv = LocalIsTv.current
    Column(
        Modifier
            .fillMaxSize()
            .then(if (tv) Modifier.padding(horizontal = TvSafe.horizontal, vertical = TvSafe.vertical) else Modifier.statusBarsPadding())
            .imePadding()
            .verticalScroll(rememberScrollState()),
    ) {
        Header(stringResource(R.string.pair_heading), actions.onDone)
        Column(
            Modifier.padding(16.dp).widthIn(max = 560.dp).semantics { liveRegion = LiveRegionMode.Polite },
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            when {
                view == null -> CodeEntry(actions)
                view.status == LoadStatus.Loading -> CircularProgressIndicator()
                view.outcome == ApprovalOutcome.Approved -> Outcome(
                    stringResource(R.string.pair_approved_title, view.deviceName),
                    stringResource(R.string.pair_approved_body),
                    actions.onDone,
                )
                view.outcome == ApprovalOutcome.Denied -> Outcome(
                    stringResource(R.string.pair_denied_title),
                    stringResource(R.string.pair_denied_body, view.deviceName),
                    actions.onDone,
                )
                view.status == LoadStatus.NotFound -> Retry(stringResource(R.string.pair_code_unknown), actions.onAnother)
                view.status == LoadStatus.Failed -> Retry(problemMessage(view.problem), actions.onAnother)
                else -> Request(view, userName, actions)
            }
        }
    }
}

@Composable
private fun CodeEntry(actions: ApproveActions) {
    var code by rememberSaveable { mutableStateOf("") }
    val submit = { if (code.isNotBlank()) actions.onLookup(code.trim().uppercase()) }
    Text(stringResource(R.string.pair_intro), style = MaterialTheme.typography.bodyLarge, color = Tokens.Palette.muted)
    OutlinedTextField(
        value = code,
        onValueChange = { code = it },
        label = { Text(stringResource(R.string.pair_code_label)) },
        singleLine = true,
        keyboardOptions = KeyboardOptions(
            capitalization = KeyboardCapitalization.Characters,
            autoCorrectEnabled = false,
            imeAction = ImeAction.Go,
        ),
        keyboardActions = KeyboardActions(onGo = { submit() }),
        modifier = Modifier.fillMaxWidth().remoteLeavesField(code.isEmpty()),
    )
    Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        PrimaryButton(stringResource(R.string.pair_continue), enabled = code.isNotBlank(), onClick = submit)
        actions.onScan?.let { scan ->
            OutlinedButton(onClick = scan) {
                Icon(CouchverseIcons.ScanCode, contentDescription = null, modifier = Modifier.size(18.dp))
                Spacer(Modifier.width(8.dp))
                Text(stringResource(R.string.pair_scan_code))
            }
        }
    }
}

@Composable
private fun Request(view: PairingApprovalView, userName: String, actions: ApproveActions) {
    var name by rememberSaveable(view.code) { mutableStateOf(view.deviceName) }
    Text(
        stringResource(R.string.pair_request_title, view.deviceName, userName),
        style = MaterialTheme.typography.titleLarge,
        color = Tokens.Palette.text,
    )
    Text(platformName(view.platform), style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.muted)
    Text(stringResource(R.string.pair_request_warning), style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.danger)
    OutlinedTextField(
        value = name,
        onValueChange = { name = it },
        label = { Text(stringResource(R.string.pair_device_name)) },
        supportingText = { Text(stringResource(R.string.pair_device_name_hint)) },
        singleLine = true,
        modifier = Modifier.fillMaxWidth().remoteLeavesField(name.isEmpty()),
    )
    Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        PrimaryButton(stringResource(R.string.pair_approve), onClick = { actions.onApprove(view.code, name.trim()) })
        SecondaryButton(stringResource(R.string.pair_deny), onClick = { actions.onDeny(view.code) })
    }
}

@Composable
private fun Outcome(title: String, body: String, onDone: () -> Unit) {
    Text(title, style = MaterialTheme.typography.titleLarge, color = Tokens.Palette.text)
    Text(body, style = MaterialTheme.typography.bodyLarge, color = Tokens.Palette.muted)
    PrimaryButton(stringResource(R.string.pair_done), onClick = onDone)
}

@Composable
private fun Retry(message: String, onAnother: () -> Unit) {
    Text(message, style = MaterialTheme.typography.bodyLarge, color = Tokens.Palette.text)
    PrimaryButton(stringResource(R.string.pair_another_code), onClick = onAnother)
}

@Composable
private fun PrimaryButton(text: String, onClick: () -> Unit, enabled: Boolean = true) {
    if (LocalIsTv.current) TvActionButton(text, onClick = onClick, primary = true, enabled = enabled)
    else Button(onClick = onClick, enabled = enabled) { Text(text) }
}

@Composable
private fun SecondaryButton(text: String, onClick: () -> Unit) {
    if (LocalIsTv.current) TvActionButton(text, onClick = onClick) else OutlinedButton(onClick = onClick) { Text(text) }
}

/**
 * [ApproveScreen] over the core. [opened] is true when a scanned or tapped link already asked
 * the core for the request, so the screen shows it instead of the code field.
 */
@Composable
fun ApproveRoute(opened: Boolean, onScan: (() -> Unit)?, onDone: () -> Unit) {
    val send = rememberSend()
    val view by rememberSurface<PairingApprovalView>(Surface.PairingApproval)
    val session by rememberSurface<SessionView>(Surface.Session)
    var asked by rememberSaveable { mutableStateOf(opened) }
    LaunchedEffect(opened) { if (opened) asked = true }
    ApproveScreen(
        view = view.takeIf { asked },
        userName = session?.user?.displayName.orEmpty(),
        actions = ApproveActions(
            onLookup = { code ->
                asked = true
                send(Event.PairingApprovalOpened(UserCode(code)))
            },
            onApprove = { code, name -> send(Event.PairingApproved(PairingApproval(code, name))) },
            onDeny = { send(Event.PairingDenied(UserCode(it))) },
            onAnother = { asked = false },
            onScan = onScan,
            onDone = onDone,
        ),
    )
}
