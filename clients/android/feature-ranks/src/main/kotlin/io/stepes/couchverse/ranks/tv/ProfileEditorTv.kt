package io.stepes.couchverse.ranks.tv

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.focusOnStart
import io.stepes.couchverse.design.tv.remoteLeavesField
import io.stepes.couchverse.ranks.EditorForm
import io.stepes.couchverse.ranks.EditorHeading
import io.stepes.couchverse.ranks.ProfileEditorActions
import io.stepes.couchverse.ranks.ProfileEditorState

/** The name, bio and password; a TV has no photo picker, so pictures are changed on a phone. */
@Composable
internal fun ProfileEditorTv(state: ProfileEditorState, actions: ProfileEditorActions) {
    Column(
        Modifier
            .fillMaxSize()
            .padding(horizontal = TvSafe.horizontal, vertical = TvSafe.vertical)
            .verticalScroll(rememberScrollState())
            .padding(vertical = 8.dp)
            .widthIn(max = 640.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) { EditorHeading() }
        EditorForm(
            state,
            actions,
            extras = { value, start -> Modifier.remoteLeavesField(value.isEmpty()).then(if (start) Modifier.focusOnStart() else Modifier) },
        ) { label, primary, enabled, onClick ->
            TvActionButton(label, onClick = onClick, primary = primary, enabled = enabled)
        }
    }
}
