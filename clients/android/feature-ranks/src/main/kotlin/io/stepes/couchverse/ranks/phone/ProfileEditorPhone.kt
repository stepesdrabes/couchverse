package io.stepes.couchverse.ranks.phone

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.ImageSlot
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Artwork
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.phone.PhoneGutter
import io.stepes.couchverse.design.phone.inkButtonColors
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.ranks.EditorForm
import io.stepes.couchverse.ranks.EditorHeading
import io.stepes.couchverse.ranks.ProfileEditorActions
import io.stepes.couchverse.ranks.ProfileEditorState

@Composable
internal fun ProfileEditorPhone(state: ProfileEditorState, actions: ProfileEditorActions) {
    Column(
        Modifier
            .fillMaxSize()
            .statusBarsPadding()
            .imePadding()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = PhoneGutter, vertical = 8.dp)
            .widthIn(max = 640.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            actions.onBack?.let { back ->
                IconButton(onClick = back) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back)) }
            }
            EditorHeading()
        }

        actions.onPick?.let { pick ->
            Pictures(state, pick, actions.onRemove)
        }

        EditorForm(state, actions, extras = { _, _ -> Modifier }) { label, primary, enabled, onClick ->
            if (primary) {
                Button(onClick = onClick, enabled = enabled) { Text(label) }
            } else {
                OutlinedButton(onClick = onClick, enabled = enabled) { Text(label) }
            }
        }
    }
}

@Composable
private fun Pictures(state: ProfileEditorState, pick: (ImageSlot) -> Unit, remove: (ImageSlot) -> Unit) {
    Box(Modifier.fillMaxWidth().aspectRatio(3f).clip(RoundedCornerShape(Tokens.Radius.card))) {
        Artwork(state.bannerUrl, modifier = Modifier.fillMaxSize())
        if (state.view?.banner?.status == LoadStatus.Loading) CircularProgressIndicator(Modifier.align(Alignment.Center))
    }
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        OutlinedButton(onClick = { pick(ImageSlot.Banner) }) { Text(stringResource(if (state.bannerUrl == null) R.string.profile_banner_upload else R.string.profile_banner_replace)) }
        if (state.bannerUrl != null) TextButton(onClick = { remove(ImageSlot.Banner) }, colors = inkButtonColors()) { Text(stringResource(R.string.profile_banner_remove)) }
    }
    state.view?.banner?.problem?.let { Text(problemMessage(it), color = Tokens.Palette.danger) }
    Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
        Box {
            Avatar(state.avatarUrl, seed = state.user?.username.orEmpty(), size = 72.dp)
            if (state.view?.avatar?.status == LoadStatus.Loading) CircularProgressIndicator(Modifier.align(Alignment.Center))
        }
        OutlinedButton(onClick = { pick(ImageSlot.Avatar) }) { Text(stringResource(R.string.profile_change_picture)) }
        if (state.avatarUrl != null) TextButton(onClick = { remove(ImageSlot.Avatar) }, colors = inkButtonColors()) { Text(stringResource(R.string.profile_remove_picture)) }
    }
    state.view?.avatar?.problem?.let { Text(problemMessage(it), color = Tokens.Palette.danger) }
}
