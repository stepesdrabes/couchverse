package io.stepes.couchverse.couch

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.stepes.couchverse.core.CouchRole
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.QrCode
import io.stepes.couchverse.design.theme.LocalAccent

/**
 * The session at a glance: for the host the code and a QR code of the join page to pass
 * around, then who is on the couch, and the way out (ending it for everyone, for the host).
 * [button] draws the idiom's button.
 */
@Composable
fun CouchPanel(
    view: CouchView,
    onEnd: () -> Unit,
    onLeave: () -> Unit,
    qrSize: Dp,
    modifier: Modifier = Modifier,
    button: @Composable (label: String, onClick: () -> Unit) -> Unit,
) {
    Column(modifier, verticalArrangement = Arrangement.spacedBy(16.dp)) {
        Text(
            stringResource(R.string.couch_on_couch_count, view.members.size.toString()),
            fontSize = 22.sp,
            color = Tokens.Palette.text,
            modifier = Modifier.semantics { heading() },
        )
        couchStatusText(view)?.let { Text(it, color = Tokens.Palette.muted) }
        val share = view.shareUrl
        val code = view.code
        if (view.role == CouchRole.Host && share != null && code != null) {
            Row(horizontalArrangement = Arrangement.spacedBy(20.dp), verticalAlignment = Alignment.CenterVertically) {
                QrCode(share, stringResource(R.string.a11y_qr_code, share), Modifier.size(qrSize))
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(stringResource(R.string.couch_share_label), color = Tokens.Palette.muted)
                    Text(spacedCode(code), fontSize = 34.sp, fontFamily = FontFamily.Monospace, color = Tokens.Palette.text)
                    Text(share, color = LocalAccent.current.ink, fontSize = 13.sp, maxLines = 2)
                }
            }
        }
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            view.members.sortedByDescending { it.host }.forEach { MemberLine(it) }
        }
        if (view.role == CouchRole.Host) {
            button(stringResource(R.string.couch_end_session), onEnd)
        } else {
            button(stringResource(R.string.couch_leave), onLeave)
        }
    }
}

/** "123 456": a six-digit code read out in two halves. */
fun spacedCode(code: String): String = if (code.length == 6) "${code.take(3)} ${code.drop(3)}" else code
