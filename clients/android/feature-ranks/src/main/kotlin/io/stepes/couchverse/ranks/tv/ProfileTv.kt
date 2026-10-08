package io.stepes.couchverse.ranks.tv

import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Edit
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.ProfileDetail
import io.stepes.couchverse.core.ProfileView
import io.stepes.couchverse.core.TopTitle
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvPosterCard
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.ranks.ProfileActions
import io.stepes.couchverse.ranks.ProfileHeader
import io.stepes.couchverse.ranks.ProfileList
import io.stepes.couchverse.ranks.ProfileStates
import io.stepes.couchverse.ranks.watchedFor

@Composable
internal fun ProfileTv(view: ProfileView?, actions: ProfileActions) {
    Box(Modifier.fillMaxSize()) {
        ProfileStates(view, actions.onRetry) { profile ->
            ProfileList(
                profile,
                TvSafe.horizontal,
                header = { Header(profile, actions) },
                section = { content -> Section(content) },
                topTitle = { title -> TopTitleCard(title, actions.onOpenTitle) },
            )
        }
    }
}

@Composable
private fun Header(profile: ProfileDetail, actions: ProfileActions) {
    ProfileHeader(profile, TvSafe.horizontal, bannerHeight = 200.dp) {
        TvActionButton(stringResource(R.string.profiles_edit_profile), onClick = actions.onEdit, icon = Icons.Filled.Edit)
        TvActionButton(
            stringResource(R.string.profiles_public_label),
            onClick = { actions.onPublic(!profile.public) },
            icon = if (profile.public) Icons.Filled.Check else Icons.Filled.Close,
        )
    }
}

/**
 * A block of the page that takes focus, so the remote can scroll a page that is mostly
 * reading; the focused block is outlined.
 */
@Composable
private fun Section(content: @Composable () -> Unit) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    Column(
        Modifier
            .padding(horizontal = TvSafe.horizontal)
            .fillMaxWidth()
            .clip(RoundedCornerShape(Tokens.Radius.card))
            .border(2.dp, if (focused) Tokens.Palette.text else Tokens.Palette.edge, RoundedCornerShape(Tokens.Radius.card))
            .focusable(interactionSource = interaction)
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) { content() }
}

@Composable
private fun TopTitleCard(title: TopTitle, onOpen: (String) -> Unit) {
    TvPosterCard(
        name = title.name,
        posterUrl = title.poster?.url,
        accent = title.poster?.accent,
        caption = watchedFor(title),
        onClick = { onOpen(title.slug) },
        width = 120.dp,
    )
}
