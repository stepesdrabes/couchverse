package io.stepes.couchverse.ui

import androidx.annotation.StringRes
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.NoticeRef
import io.stepes.couchverse.core.NoticesView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.theme.Motion
import io.stepes.couchverse.design.tv.TvSafe
import kotlinx.coroutines.delay

private const val NOTICE_MILLIS = 4_000L

/** The core's transient notices, one at a time: above the tab bar on phones, top right on TV. */
@Composable
fun Notices() {
    val send = rememberSend()
    val view by rememberSurface<NoticesView>(Surface.Notices)
    val notice = view?.notices?.firstOrNull()
    if (notice != null) {
        LaunchedEffect(notice.id) {
            delay(NOTICE_MILLIS)
            send(Event.NoticeDismissed(NoticeRef(notice.id)))
        }
    }
    val tv = LocalIsTv.current
    Box(
        Modifier
            .fillMaxSize()
            .then(if (tv) Modifier.padding(horizontal = TvSafe.horizontal, vertical = TvSafe.vertical) else Modifier.navigationBarsPadding().padding(bottom = 96.dp)),
        contentAlignment = if (tv) Alignment.TopEnd else Alignment.BottomCenter,
    ) {
        AnimatedVisibility(notice != null, enter = fadeIn(Motion.standard()), exit = fadeOut(Motion.standard())) {
            Text(
                notice?.let { stringResource(noticeText(it.code)) }.orEmpty(),
                style = MaterialTheme.typography.bodyMedium,
                color = Tokens.Palette.text,
                modifier = Modifier
                    .background(Tokens.Palette.surface2, RoundedCornerShape(12.dp))
                    .padding(horizontal = 16.dp, vertical = 12.dp)
                    .semantics { liveRegion = LiveRegionMode.Polite },
            )
        }
    }
}

@StringRes
private fun noticeText(code: String): Int = when (code) {
    "watchlist_failed" -> R.string.catalog_my_list_update_failed
    "visibility_failed" -> R.string.profiles_privacy_failed
    else -> R.string.problem_generic
}
