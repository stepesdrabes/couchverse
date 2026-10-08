package io.stepes.couchverse.couch.phone

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.couch.CodeForm
import io.stepes.couchverse.couch.CouchInvite
import io.stepes.couchverse.couch.JoinCouchActions
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.phone.inkButtonColors

/**
 * Joining to watch here or to steer this account's player elsewhere, or scanning the code. A
 * guest has no player of its own to steer.
 */
@Composable
internal fun JoinCouchPhone(view: CouchView?, invite: CouchInvite, guest: Boolean, actions: JoinCouchActions) {
    Box(Modifier.fillMaxSize().safeDrawingPadding(), contentAlignment = Alignment.Center) {
        IconButton(onClick = actions.onBack, modifier = Modifier.align(Alignment.TopStart).padding(8.dp)) {
            Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back))
        }
        Column(Modifier.widthIn(max = 480.dp).padding(horizontal = 24.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
            CodeForm(view, invite, guest, actions, field = { _, _ -> Modifier }) { entry ->
                Button(onClick = entry.join, enabled = entry.ready, modifier = Modifier.fillMaxWidth()) {
                    Text(if (entry.joining) stringResource(R.string.couch_connecting) else stringResource(R.string.couch_join))
                }
                if (!guest) {
                    OutlinedButton(onClick = entry.remote, enabled = entry.ready, modifier = Modifier.fillMaxWidth()) {
                        Text(stringResource(R.string.couch_join_remote))
                    }
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
