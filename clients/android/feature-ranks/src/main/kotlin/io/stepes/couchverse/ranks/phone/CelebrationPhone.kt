package io.stepes.couchverse.ranks.phone

import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import io.stepes.couchverse.core.AchievementCard
import io.stepes.couchverse.design.R
import io.stepes.couchverse.ranks.Celebration

@Composable
internal fun CelebrationPhone(card: AchievementCard, onDismiss: () -> Unit) {
    Celebration(card) {
        Button(onClick = onDismiss) { Text(stringResource(R.string.common_done)) }
    }
}
