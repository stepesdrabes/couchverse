package io.stepes.couchverse.ranks.phone

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.ProfileDetail
import io.stepes.couchverse.core.ProfileView
import io.stepes.couchverse.core.TopTitle
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Artwork
import io.stepes.couchverse.design.phone.PhoneGutter
import io.stepes.couchverse.ranks.ProfileActions
import io.stepes.couchverse.ranks.ProfileHeader
import io.stepes.couchverse.ranks.ProfileList
import io.stepes.couchverse.ranks.ProfileStates
import io.stepes.couchverse.ranks.watchedFor

@Composable
internal fun ProfilePhone(view: ProfileView?, actions: ProfileActions) {
    Box(Modifier.fillMaxSize()) {
        ProfileStates(view, actions.onRetry) { profile ->
            ProfileList(
                profile,
                PhoneGutter,
                header = { Header(profile, actions) },
                section = { content -> Section(content) },
                topTitle = { title -> TopTitleCard(title, actions.onOpenTitle) },
            )
        }
        actions.onBack?.let { back ->
            IconButton(onClick = back, modifier = Modifier.statusBarsPadding().padding(8.dp)) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back))
            }
        }
    }
}

@Composable
private fun Header(profile: ProfileDetail, actions: ProfileActions) {
    ProfileHeader(profile, PhoneGutter, bannerHeight = 160.dp) {
        OutlinedButton(onClick = actions.onEdit) {
            Icon(Icons.Filled.Edit, contentDescription = null, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(8.dp))
            Text(stringResource(R.string.profiles_edit_profile))
        }
        Spacer(Modifier.weight(1f))
        Text(stringResource(R.string.profiles_public_label), color = Tokens.Palette.text)
        Switch(checked = profile.public, onCheckedChange = actions.onPublic)
    }
}

@Composable
private fun Section(content: @Composable () -> Unit) {
    Column(Modifier.padding(horizontal = PhoneGutter).fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(10.dp)) { content() }
}

@Composable
private fun TopTitleCard(title: TopTitle, onOpen: (String) -> Unit) {
    Column(
        Modifier.width(96.dp).clickable(role = Role.Button) { onOpen(title.slug) },
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        Artwork(
            title.poster?.url,
            accent = title.poster?.accent,
            fallbackName = title.name,
            modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(RoundedCornerShape(10.dp)),
        )
        Text(title.name, style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(watchedFor(title), style = MaterialTheme.typography.labelSmall, color = Tokens.Palette.muted)
    }
}
