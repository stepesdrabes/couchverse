package io.stepes.couchverse.accounts.phone

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.autofill.ContentType
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentType
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.accounts.PairingPanel
import io.stepes.couchverse.accounts.SignInActions
import io.stepes.couchverse.accounts.SignInState
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.phone.inkButtonColors
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.InsecureBadge
import io.stepes.couchverse.design.text.problemMessage

@Composable
internal fun SignInPhone(state: SignInState, actions: SignInActions) {
    var username by rememberSaveable { mutableStateOf(state.username) }
    var password by rememberSaveable { mutableStateOf("") }
    val view = state.signIn
    val pairing = state.pairing
    // both ways in share the view's status, so the screen remembers which one the user chose
    var pairingChosen by rememberSaveable { mutableStateOf(false) }
    val busy = view?.status == LoadStatus.Loading
    val passwordFailed = view?.status == LoadStatus.Failed && !pairingChosen
    val canSubmit = username.isNotBlank() && password.isNotEmpty() && !busy
    val submit = { if (canSubmit) actions.onPassword(username.trim(), password) }
    val passwordFocus = remember { FocusRequester() }

    OnboardingPage(actions.onBack) {
        Text(
            stringResource(R.string.accounts_sign_in_title, state.server?.name.orEmpty()),
            style = MaterialTheme.typography.headlineMedium,
            color = Tokens.Palette.text,
            modifier = Modifier.semantics { heading() },
        )
        if (state.server?.insecure == true) InsecureBadge()
        Text(stringResource(R.string.login_subtitle), style = MaterialTheme.typography.bodyLarge, color = Tokens.Palette.muted)
        OutlinedTextField(
            value = username,
            onValueChange = { username = it },
            label = { Text(stringResource(R.string.login_username)) },
            singleLine = true,
            enabled = !busy,
            keyboardOptions = KeyboardOptions(autoCorrectEnabled = false, imeAction = ImeAction.Next),
            keyboardActions = KeyboardActions(onNext = { passwordFocus.requestFocus() }),
            modifier = Modifier.fillMaxWidth().semantics { contentType = ContentType.Username },
        )
        OutlinedTextField(
            value = password,
            onValueChange = { password = it },
            label = { Text(stringResource(R.string.login_password)) },
            singleLine = true,
            enabled = !busy,
            isError = passwordFailed,
            visualTransformation = PasswordVisualTransformation(),
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Done),
            keyboardActions = KeyboardActions(onDone = { submit() }),
            supportingText = if (passwordFailed) {
                {
                    Text(
                        problemMessage(view.problem),
                        modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
                    )
                }
            } else {
                null
            },
            modifier = Modifier
                .fillMaxWidth()
                .focusRequester(passwordFocus)
                .semantics { contentType = ContentType.Password },
        )
        Button(onClick = submit, enabled = canSubmit, modifier = Modifier.fillMaxWidth().heightIn(min = 52.dp)) {
            if (busy && !pairingChosen) {
                CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = MaterialTheme.colorScheme.onPrimary)
                Spacer(Modifier.width(10.dp))
                Text(stringResource(R.string.accounts_signing_in))
            } else {
                Text(stringResource(R.string.login_submit))
            }
        }
        Text(stringResource(R.string.login_accounts_note), style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.faint)
        HorizontalDivider(color = Tokens.Palette.edge, modifier = Modifier.padding(vertical = 8.dp))
        if (!pairingChosen && pairing == null) {
            OutlinedButton(
                onClick = {
                    pairingChosen = true
                    actions.onStartPairing()
                },
                modifier = Modifier.fillMaxWidth().heightIn(min = 52.dp),
            ) {
                Text(stringResource(R.string.accounts_sign_in_with_device))
            }
        } else {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                PairingPanel(
                    pairing = pairing,
                    loading = busy,
                    problem = view?.problem?.takeIf { view.status == LoadStatus.Failed },
                    remainingSeconds = state.remainingSeconds,
                    qrSize = 132.dp,
                    codeStyle = MaterialTheme.typography.headlineMedium,
                    newCode = { OutlinedButton(onClick = actions.onStartPairing) { Text(stringResource(R.string.accounts_pairing_new_code)) } },
                )
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End, verticalAlignment = Alignment.CenterVertically) {
                    TextButton(
                        onClick = {
                            pairingChosen = false
                            actions.onCancelPairing()
                        },
                        colors = inkButtonColors(),
                    ) { Text(stringResource(R.string.common_cancel)) }
                }
            }
        }
    }
}
