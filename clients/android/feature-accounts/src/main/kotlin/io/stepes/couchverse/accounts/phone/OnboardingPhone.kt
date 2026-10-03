package io.stepes.couchverse.accounts.phone

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.fadeIn
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
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
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.ServersView
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.components.GlowBackdrop
import io.stepes.couchverse.design.components.LogoMark
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.design.theme.Motion

@Composable
internal fun WelcomePhone(onAddServer: () -> Unit, onScan: () -> Unit) {
    Box(Modifier.fillMaxSize()) {
        GlowBackdrop()
        val shown = remember { MutableTransitionState(false).apply { targetState = true } }
        AnimatedVisibility(
            visibleState = shown,
            enter = fadeIn(Motion.enter()) + slideInVertically(Motion.smooth()) { it / 8 },
        ) {
            Column(
                Modifier
                    .fillMaxSize()
                    .safeDrawingPadding()
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = 24.dp, vertical = 32.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                Spacer(Modifier.heightIn(min = 96.dp))
                LogoMark(size = 72.dp)
                Text(
                    stringResource(R.string.onboarding_welcome_title),
                    style = MaterialTheme.typography.displaySmall,
                    color = Tokens.Palette.text,
                    modifier = Modifier.semantics { heading() },
                )
                Text(
                    stringResource(R.string.onboarding_welcome_message),
                    style = MaterialTheme.typography.bodyLarge,
                    color = Tokens.Palette.muted,
                )
                Spacer(Modifier.heightIn(min = 48.dp))
                Button(onClick = onAddServer, modifier = Modifier.fillMaxWidth().heightIn(min = 52.dp)) {
                    Text(stringResource(R.string.servers_add_title))
                }
                OutlinedButton(onClick = onScan, modifier = Modifier.fillMaxWidth().heightIn(min = 52.dp)) {
                    Icon(CouchverseIcons.ScanCode, contentDescription = null, modifier = Modifier.size(20.dp))
                    Spacer(Modifier.width(8.dp))
                    Text(stringResource(R.string.scanner_title))
                }
            }
        }
    }
}

@Composable
internal fun AddServerPhone(view: ServersView?, onSubmit: (String) -> Unit, onBack: (() -> Unit)?, onScan: () -> Unit) {
    var address by rememberSaveable { mutableStateOf(view?.add?.address.orEmpty()) }
    val add = view?.add
    val loading = add?.status == LoadStatus.Loading
    val failed = add?.status == LoadStatus.Failed
    val submit = { if (address.isNotBlank() && !loading) onSubmit(address.trim()) }
    OnboardingPage(onBack) {
        Text(
            stringResource(R.string.servers_add_title),
            style = MaterialTheme.typography.headlineMedium,
            color = Tokens.Palette.text,
            modifier = Modifier.semantics { heading() },
        )
        Text(
            stringResource(R.string.servers_address_hint),
            style = MaterialTheme.typography.bodyLarge,
            color = Tokens.Palette.muted,
        )
        OutlinedTextField(
            value = address,
            onValueChange = { address = it },
            label = { Text(stringResource(R.string.servers_address_label)) },
            placeholder = { Text(stringResource(R.string.servers_address_placeholder)) },
            singleLine = true,
            enabled = !loading,
            isError = failed,
            supportingText = if (failed) {
                { Text(problemMessage(add.problem), modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite }) }
            } else {
                null
            },
            keyboardOptions = KeyboardOptions(
                keyboardType = KeyboardType.Uri,
                imeAction = ImeAction.Go,
                autoCorrectEnabled = false,
            ),
            keyboardActions = KeyboardActions(onGo = { submit() }),
            modifier = Modifier.fillMaxWidth(),
        )
        Button(
            onClick = submit,
            enabled = address.isNotBlank() && !loading,
            modifier = Modifier.fillMaxWidth().heightIn(min = 52.dp),
        ) {
            if (loading) {
                CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = MaterialTheme.colorScheme.onPrimary)
                Spacer(Modifier.width(10.dp))
                Text(stringResource(R.string.onboarding_connecting))
            } else {
                Text(stringResource(R.string.servers_connect))
            }
        }
        TextButton(onClick = onScan, modifier = Modifier.align(Alignment.CenterHorizontally)) {
            Icon(CouchverseIcons.ScanCode, contentDescription = null, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(8.dp))
            Text(stringResource(R.string.scanner_title))
        }
    }
}

/** A scrolling onboarding step over the glow, with a back arrow when there is somewhere to go. */
@Composable
internal fun OnboardingPage(onBack: (() -> Unit)?, content: @Composable ColumnScope.() -> Unit) {
    Box(Modifier.fillMaxSize()) {
        GlowBackdrop(intensity = 0.6f)
        Column(
            Modifier
                .fillMaxSize()
                .safeDrawingPadding()
                .imePadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 24.dp, vertical = 16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            if (onBack != null) {
                IconButton(onClick = onBack, modifier = Modifier.padding(start = 0.dp)) {
                    Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back))
                }
            } else {
                Spacer(Modifier.heightIn(min = 48.dp))
            }
            Column(
                Modifier.widthIn(max = 520.dp).align(Alignment.CenterHorizontally),
                verticalArrangement = Arrangement.spacedBy(16.dp),
                content = content,
            )
        }
    }
}
