package io.stepes.couchverse.couch.tv

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.widthIn
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
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.focusOnStart
import io.stepes.couchverse.design.tv.remoteLeavesField

/**
 * Joining to watch on the TV: focus starts on what is missing (a guest's server, else the code),
 * and the remote can leave either field.
 */
@Composable
internal fun JoinCouchTv(view: CouchView?, invite: CouchInvite, guest: Boolean, actions: JoinCouchActions) {
    Box(Modifier.fillMaxSize().safeDrawingPadding(), contentAlignment = Alignment.Center) {
        Column(Modifier.widthIn(max = 480.dp).padding(horizontal = TvSafe.horizontal), verticalArrangement = Arrangement.spacedBy(16.dp)) {
            CodeForm(
                view,
                invite,
                guest,
                actions,
                field = { value, start -> Modifier.focusOnStart(enabled = start).remoteLeavesField(value.isEmpty()) },
            ) { entry ->
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    TvActionButton(stringResource(R.string.couch_join), onClick = entry.join, primary = true, enabled = entry.ready)
                    TvActionButton(stringResource(R.string.common_back), onClick = actions.onBack)
                }
            }
        }
    }
}
