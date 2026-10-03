package io.stepes.couchverse.accounts.tv

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.OutlinedTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
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
import androidx.tv.foundation.ExperimentalTvFoundationApi
import androidx.tv.foundation.text.PlatformImeOptions
import androidx.tv.foundation.text.TvKeyboardAlignment
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import io.stepes.couchverse.accounts.PairingPanel
import io.stepes.couchverse.accounts.SignInActions
import io.stepes.couchverse.accounts.SignInState
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.GlowBackdrop
import io.stepes.couchverse.design.components.InsecureBadge
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.focusOnStart

/** Pairing fills the left; a password is the slower way in, behind a button on the right. */
@Composable
internal fun SignInTv(state: SignInState, actions: SignInActions) {
    var withPassword by rememberSaveable { mutableStateOf(false) }
    val view = state.signIn
    val pairing = state.pairing
    val busy = view?.status == LoadStatus.Loading
    Box(Modifier.fillMaxSize()) {
        GlowBackdrop(intensity = 0.55f)
        Column(
            Modifier.fillMaxSize().padding(horizontal = TvSafe.horizontal, vertical = TvSafe.vertical),
            verticalArrangement = Arrangement.spacedBy(20.dp),
        ) {
            Row(horizontalArrangement = Arrangement.spacedBy(16.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(
                    stringResource(R.string.login_server_title, state.server?.name.orEmpty()),
                    style = MaterialTheme.typography.headlineMedium,
                    color = Tokens.Palette.text,
                    modifier = Modifier.semantics { heading() },
                )
                if (state.server?.insecure == true) InsecureBadge()
            }
            Row(Modifier.weight(1f), horizontalArrangement = Arrangement.spacedBy(40.dp)) {
                Column(
                    Modifier
                        .weight(1.7f)
                        .background(Tokens.Palette.surface.copy(alpha = 0.72f), RoundedCornerShape(20.dp))
                        .padding(28.dp),
                    verticalArrangement = Arrangement.spacedBy(16.dp),
                ) {
                    Text(stringResource(R.string.pairing_title), style = MaterialTheme.typography.titleLarge, color = Tokens.Palette.text)
                    PairingPanel(
                        pairing = pairing,
                        // pairing starts with the screen, so until it fails it is on its way
                        loading = !withPassword && view?.status != LoadStatus.Failed,
                        problem = view?.problem?.takeIf { view.status == LoadStatus.Failed && !withPassword },
                        remainingSeconds = state.remainingSeconds,
                        qrSize = 176.dp,
                        codeStyle = MaterialTheme.typography.headlineLarge,
                        newCode = {
                            TvActionButton(stringResource(R.string.pairing_new_code), onClick = actions.onStartPairing, primary = true)
                        },
                    )
                }
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(14.dp)) {
                    if (withPassword) {
                        PasswordFormTv(state, actions, busy)
                    } else {
                        Text(
                            stringResource(R.string.login_accounts_note),
                            style = MaterialTheme.typography.bodyMedium,
                            color = Tokens.Palette.muted,
                        )
                        TvActionButton(stringResource(R.string.login_with_password), onClick = { withPassword = true })
                    }
                    actions.onBack?.let { back -> TvActionButton(stringResource(R.string.common_back), onClick = back) }
                }
            }
        }
    }
}

@OptIn(ExperimentalTvFoundationApi::class)
@Composable
private fun PasswordFormTv(state: SignInState, actions: SignInActions, busy: Boolean) {
    var username by rememberSaveable { mutableStateOf(state.username) }
    var password by rememberSaveable { mutableStateOf("") }
    val view = state.signIn
    val failed = view?.status == LoadStatus.Failed && view.pairing == null
    val focus = remember { FocusRequester() }
    val passwordFocus = remember { FocusRequester() }
    val canSubmit = username.isNotBlank() && password.isNotEmpty() && !busy
    val submit = { if (canSubmit) actions.onPassword(username.trim(), password) }
    // the keyboard opens on the left, over the pairing panel, so the form stays in view
    val keyboard = PlatformImeOptions(TvKeyboardAlignment.Left)
    OutlinedTextField(
        value = username,
        onValueChange = { username = it },
        label = { androidx.compose.material3.Text(stringResource(R.string.login_username)) },
        singleLine = true,
        enabled = !busy,
        keyboardOptions = KeyboardOptions(autoCorrectEnabled = false, imeAction = ImeAction.Next, platformImeOptions = keyboard),
        keyboardActions = KeyboardActions(onNext = { passwordFocus.requestFocus() }),
        modifier = Modifier.fillMaxWidth().focusOnStart(focus).semantics { contentType = ContentType.Username },
    )
    OutlinedTextField(
        value = password,
        onValueChange = { password = it },
        label = { androidx.compose.material3.Text(stringResource(R.string.login_password)) },
        singleLine = true,
        enabled = !busy,
        isError = failed,
        visualTransformation = PasswordVisualTransformation(),
        keyboardOptions = KeyboardOptions(
            keyboardType = KeyboardType.Password,
            imeAction = ImeAction.Done,
            platformImeOptions = keyboard,
        ),
        keyboardActions = KeyboardActions(onDone = { submit() }),
        modifier = Modifier
            .fillMaxWidth()
            .focusRequester(passwordFocus)
            .semantics { contentType = ContentType.Password },
    )
    if (failed) {
        Text(
            problemMessage(view.problem),
            style = MaterialTheme.typography.bodyMedium,
            color = Tokens.Palette.danger,
            modifier = Modifier.width(360.dp).semantics { liveRegion = LiveRegionMode.Polite },
        )
    }
    TvActionButton(
        stringResource(if (busy) R.string.login_signing_in else R.string.login_submit),
        onClick = submit,
        primary = true,
        enabled = canSubmit,
    )
}
