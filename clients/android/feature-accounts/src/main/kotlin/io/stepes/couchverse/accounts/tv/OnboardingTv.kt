package io.stepes.couchverse.accounts.tv

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.fadeIn
import androidx.compose.animation.slideInHorizontally
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
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
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.tv.foundation.ExperimentalTvFoundationApi
import androidx.tv.foundation.text.PlatformImeOptions
import androidx.tv.foundation.text.TvKeyboardAlignment
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.ServersView
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.GlowBackdrop
import io.stepes.couchverse.design.components.LogoMark
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.design.theme.Motion
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.focusOnStart

@Composable
internal fun WelcomeTv(onAddServer: () -> Unit) {
    val focus = remember { FocusRequester() }
    TvOnboardingPage {
        LogoMark(size = 88.dp)
        Text(
            stringResource(R.string.onboarding_welcome_title),
            style = MaterialTheme.typography.displaySmall,
            color = Tokens.Palette.text,
            modifier = Modifier.semantics { heading() },
        )
        Text(
            stringResource(R.string.onboarding_welcome_body),
            style = MaterialTheme.typography.titleMedium,
            color = Tokens.Palette.muted,
        )
        TvActionButton(
            stringResource(R.string.servers_add),
            onClick = onAddServer,
            primary = true,
            modifier = Modifier.padding(top = 16.dp).focusOnStart(focus),
        )
    }
}

/** The keyboard opens on the right, so the explanation and the field stay visible on the left. */
@OptIn(ExperimentalTvFoundationApi::class)
@Composable
internal fun AddServerTv(view: ServersView?, onSubmit: (String) -> Unit, onBack: (() -> Unit)?) {
    var address by rememberSaveable { mutableStateOf(view?.add?.address.orEmpty()) }
    val add = view?.add
    val loading = add?.status == LoadStatus.Loading
    val failed = add?.status == LoadStatus.Failed
    val submit = { if (address.isNotBlank() && !loading) onSubmit(address.trim()) }
    val focus = remember { FocusRequester() }
    TvOnboardingPage {
        Text(
            stringResource(R.string.onboarding_add_server_title),
            style = MaterialTheme.typography.headlineMedium,
            color = Tokens.Palette.text,
            modifier = Modifier.semantics { heading() },
        )
        Text(
            stringResource(R.string.onboarding_add_server_body),
            style = MaterialTheme.typography.bodyLarge,
            color = Tokens.Palette.muted,
        )
        OutlinedTextField(
            value = address,
            onValueChange = { address = it },
            label = { androidx.compose.material3.Text(stringResource(R.string.onboarding_server_address)) },
            placeholder = { androidx.compose.material3.Text(stringResource(R.string.onboarding_server_address_placeholder)) },
            singleLine = true,
            enabled = !loading,
            isError = failed,
            keyboardOptions = KeyboardOptions(
                keyboardType = KeyboardType.Uri,
                imeAction = ImeAction.Go,
                autoCorrectEnabled = false,
                platformImeOptions = PlatformImeOptions(TvKeyboardAlignment.Right),
            ),
            keyboardActions = KeyboardActions(onGo = { submit() }),
            modifier = Modifier.fillMaxWidth().focusOnStart(focus),
        )
        if (failed) {
            Text(
                problemMessage(add.problem),
                style = MaterialTheme.typography.bodyMedium,
                color = Tokens.Palette.danger,
                modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
            )
        }
        Row(horizontalArrangement = Arrangement.spacedBy(16.dp), modifier = Modifier.padding(top = 8.dp)) {
            TvActionButton(
                stringResource(if (loading) R.string.onboarding_connecting else R.string.onboarding_connect),
                onClick = submit,
                primary = true,
                enabled = address.isNotBlank() && !loading,
            )
            if (onBack != null) {
                TvActionButton(stringResource(R.string.common_back), onClick = onBack)
            }
        }
    }
}

/** An onboarding step on the TV: the glow, and a column of content on the left half. */
@Composable
internal fun TvOnboardingPage(content: @Composable ColumnScope.() -> Unit) {
    Box(Modifier.fillMaxSize()) {
        GlowBackdrop()
        val shown = remember { MutableTransitionState(false).apply { targetState = true } }
        AnimatedVisibility(
            visibleState = shown,
            enter = fadeIn(Motion.enter()) + slideInHorizontally(Motion.smooth()) { -it / 10 },
        ) {
            Column(
                Modifier
                    .fillMaxHeight()
                    .padding(horizontal = TvSafe.horizontal * 2, vertical = TvSafe.vertical)
                    .widthIn(max = 520.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp, Alignment.CenterVertically),
                content = content,
            )
        }
    }
}
