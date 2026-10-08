package io.stepes.couchverse.ranks.tv

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.FilterChip
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import io.stepes.couchverse.core.LeaderboardKey
import io.stepes.couchverse.core.LeaderboardView
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.focusOnStart
import io.stepes.couchverse.ranks.LeaderboardHeading
import io.stepes.couchverse.ranks.LeaderboardList
import io.stepes.couchverse.ranks.lineBackground

/** The board for the remote: focus starts on the first metric, and every row opens a profile. */
@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
internal fun LeaderboardTv(view: LeaderboardView?, key: LeaderboardKey, onKey: (LeaderboardKey) -> Unit, onProfile: (String) -> Unit) {
    val shape = RoundedCornerShape(Tokens.Radius.card)
    LeaderboardList(
        view,
        key,
        onKey,
        gutter = TvSafe.horizontal,
        contentPadding = PaddingValues(top = TvSafe.vertical, bottom = 32.dp),
        standing = Modifier.padding(start = TvSafe.horizontal, top = 8.dp, end = TvSafe.horizontal, bottom = TvSafe.vertical),
        title = {
            Row(Modifier.padding(horizontal = TvSafe.horizontal), verticalAlignment = Alignment.CenterVertically) { LeaderboardHeading() }
        },
        chip = { selected, label, onClick, first ->
            FilterChip(selected = selected, onClick = onClick, modifier = if (first) Modifier.focusOnStart() else Modifier) { Text(label) }
        },
        line = { row, content ->
            Surface(
                onClick = { onProfile(row.username) },
                shape = ClickableSurfaceDefaults.shape(shape),
                colors = ClickableSurfaceDefaults.colors(containerColor = lineBackground(row), focusedContainerColor = Tokens.Palette.edge),
                modifier = Modifier.padding(horizontal = TvSafe.horizontal).fillMaxWidth(),
            ) { content() }
        },
    )
}
