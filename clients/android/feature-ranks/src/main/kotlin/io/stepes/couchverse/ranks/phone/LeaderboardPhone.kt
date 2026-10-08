package io.stepes.couchverse.ranks.phone

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.LeaderboardKey
import io.stepes.couchverse.core.LeaderboardView
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.phone.PhoneGutter
import io.stepes.couchverse.ranks.LeaderboardHeading
import io.stepes.couchverse.ranks.LeaderboardList
import io.stepes.couchverse.ranks.lineBackground

@Composable
internal fun LeaderboardPhone(view: LeaderboardView?, key: LeaderboardKey, onKey: (LeaderboardKey) -> Unit, onProfile: (String) -> Unit, onBack: (() -> Unit)?) {
    val shape = RoundedCornerShape(Tokens.Radius.card)
    LeaderboardList(
        view,
        key,
        onKey,
        gutter = PhoneGutter,
        contentPadding = PaddingValues(top = 0.dp, bottom = 32.dp),
        standing = Modifier.navigationBarsPadding().padding(horizontal = PhoneGutter, vertical = 8.dp),
        modifier = Modifier.statusBarsPadding(),
        title = {
            Row(Modifier.padding(horizontal = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                if (onBack != null) {
                    IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back)) }
                }
                LeaderboardHeading()
            }
        },
        chip = { selected, label, onClick, _ -> FilterChip(selected = selected, onClick = onClick, label = { Text(label) }) },
        line = { row, content ->
            Box(
                Modifier
                    .padding(horizontal = PhoneGutter)
                    .fillMaxWidth()
                    .clip(shape)
                    .background(lineBackground(row))
                    .clickable(role = Role.Button) { onProfile(row.username) },
            ) { content() }
        },
    )
}
