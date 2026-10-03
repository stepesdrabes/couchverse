package io.stepes.couchverse.accounts

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.SignInView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.GlowBackdrop
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.text.problemMessage

/**
 * Signing in from a scanned "Connect a device" link: the core identifies the server and redeems
 * the code, and the app moves on by itself once the account is in.
 */
@Composable
fun ConnectingScreen(view: SignInView?, onBack: () -> Unit) {
    val failed = view?.status == LoadStatus.Failed
    Box(Modifier.fillMaxSize()) {
        GlowBackdrop(intensity = 0.6f)
        Column(
            Modifier.align(Alignment.Center).padding(32.dp).semantics { liveRegion = LiveRegionMode.Polite },
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(20.dp),
        ) {
            if (failed) {
                Text(
                    problemMessage(view.problem),
                    style = MaterialTheme.typography.titleMedium,
                    color = Tokens.Palette.text,
                    textAlign = TextAlign.Center,
                )
                Button(onClick = onBack) { Text(stringResource(R.string.common_back)) }
            } else {
                CircularProgressIndicator()
                Text(stringResource(R.string.login_signing_in), style = MaterialTheme.typography.titleMedium, color = Tokens.Palette.text)
            }
        }
    }
}

@Composable
fun ConnectingRoute(onBack: () -> Unit) {
    val view by rememberSurface<SignInView>(Surface.SignIn)
    ConnectingScreen(view, onBack)
}
