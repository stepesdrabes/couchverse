package io.stepes.couchverse.accounts

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.size
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import io.stepes.couchverse.core.PairingState
import io.stepes.couchverse.core.PairingView
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.QrCode
import io.stepes.couchverse.design.text.problemMessage

/**
 * Pairing as the device being signed in sees it: a QR code of the server's pairing page, the
 * code to type there, and how long it lasts. [newCode] is the idiom's button for starting over.
 */
@Composable
internal fun PairingPanel(
    pairing: PairingView?,
    loading: Boolean,
    problem: Problem?,
    remainingSeconds: Long,
    qrSize: Dp,
    codeStyle: androidx.compose.ui.text.TextStyle,
    newCode: @Composable () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(28.dp), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(qrSize), contentAlignment = Alignment.Center) {
            when {
                pairing != null && pairing.state == PairingState.Waiting ->
                    QrCode(pairing.verifyUrl, stringResource(R.string.pairing_qr), Modifier.size(qrSize))
                pairing == null && loading -> CircularProgressIndicator()
                else -> {}
            }
        }
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            when {
                pairing == null && problem != null -> {
                    Text(problemMessage(problem), style = MaterialTheme.typography.bodyLarge, color = Tokens.Palette.danger)
                    newCode()
                }
                pairing == null -> {
                    Text(stringResource(R.string.pairing_scan), style = MaterialTheme.typography.bodyLarge, color = Tokens.Palette.muted)
                    // a password attempt cancels a running pairing; this starts another
                    if (!loading) newCode()
                }
                pairing.state == PairingState.Waiting -> Waiting(pairing, remainingSeconds, codeStyle)
                else -> {
                    val expired = pairing.state == PairingState.Expired
                    Column(
                        Modifier.semantics { liveRegion = LiveRegionMode.Polite },
                        verticalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        Text(
                            stringResource(if (expired) R.string.pair_expired_title else R.string.pair_denied_title),
                            style = MaterialTheme.typography.titleLarge,
                            color = Tokens.Palette.text,
                        )
                        Text(
                            stringResource(if (expired) R.string.pairing_expired_body else R.string.pairing_denied_body),
                            style = MaterialTheme.typography.bodyLarge,
                            color = Tokens.Palette.muted,
                        )
                    }
                    newCode()
                }
            }
        }
    }
}

@Composable
private fun Waiting(pairing: PairingView, remainingSeconds: Long, codeStyle: androidx.compose.ui.text.TextStyle) {
    Text(
        stringResource(R.string.pairing_scan),
        style = MaterialTheme.typography.bodyLarge,
        color = Tokens.Palette.text,
    )
    Text(
        stringResource(R.string.pairing_manual, pairingPage(pairing.verifyUrl)),
        style = MaterialTheme.typography.bodyMedium,
        color = Tokens.Palette.muted,
    )
    val spelled = pairing.userCode.toList().joinToString(" ")
    Text(
        pairing.userCode,
        style = codeStyle.copy(fontFamily = FontFamily.Monospace, fontWeight = FontWeight.Bold, letterSpacing = 0.12.em),
        color = Tokens.Palette.text,
        maxLines = 1,
        softWrap = false,
        // read out character by character
        modifier = Modifier.semantics { contentDescription = spelled },
    )
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        val style = MaterialTheme.typography.bodyMedium
        Text(stringResource(R.string.pair_expires_in, formatCountdown(remainingSeconds)), style = style, color = Tokens.Palette.faint)
        Text("·", style = style, color = Tokens.Palette.faint)
        Text(stringResource(R.string.pairing_waiting), style = style, color = Tokens.Palette.faint)
    }
}
