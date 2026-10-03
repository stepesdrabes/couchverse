package io.stepes.couchverse.catalog

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.StatusMessage
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.focusOnStart

/** A screen that could not load at all, with a way to try again. */
@Composable
fun LoadFailed(problem: Problem?, onRetry: () -> Unit, modifier: Modifier = Modifier) {
    Box(modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        StatusMessage(
            title = stringResource(R.string.catalog_load_failed),
            message = problemMessage(problem),
            action = { RetryButton(onRetry) },
        )
    }
}

/** Nothing to show, with what would make something appear. */
@Composable
fun Empty(title: String, message: String? = null) {
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        StatusMessage(title = title, message = message)
    }
}

@Composable
fun RetryButton(onRetry: () -> Unit) {
    if (LocalIsTv.current) {
        val focus = remember { FocusRequester() }
        TvActionButton(stringResource(R.string.common_retry), onClick = onRetry, primary = true, modifier = Modifier.focusOnStart(focus))
    } else {
        Button(onClick = onRetry) { Text(stringResource(R.string.common_retry)) }
    }
}

/**
 * Shown above content the core kept after a refresh failed (stale beats blank): the page stays
 * usable and says it may be out of date.
 */
@Composable
fun StaleNotice(status: LoadStatus?, problem: Problem?, modifier: Modifier = Modifier) {
    if (status != LoadStatus.Stale || problem == null) return
    Text(
        stringResource(R.string.catalog_stale_notice),
        style = MaterialTheme.typography.bodySmall,
        color = Tokens.Palette.text,
        modifier = modifier
            .background(Tokens.Palette.surface2.copy(alpha = 0.9f), RoundedCornerShape(10.dp))
            .padding(horizontal = 12.dp, vertical = 8.dp)
            .semantics { liveRegion = LiveRegionMode.Polite },
    )
}

/** A reload is running behind content that is still showing. */
fun refreshing(status: LoadStatus?, problem: Problem?): Boolean = status == LoadStatus.Stale && problem == null
