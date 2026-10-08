package io.stepes.couchverse.ranks.tv

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import io.stepes.couchverse.core.AchievementCard
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.focusOnStart
import io.stepes.couchverse.ranks.Celebration

@Composable
internal fun CelebrationTv(card: AchievementCard, onDismiss: () -> Unit) {
    Celebration(card) {
        TvActionButton(stringResource(R.string.common_done), onClick = onDismiss, primary = true, modifier = Modifier.focusOnStart())
    }
}
