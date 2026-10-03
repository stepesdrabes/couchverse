package io.stepes.couchverse.design.phone

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.interaction.collectIsDraggedAsState
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.PagerScope
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.theme.LocalAccent
import io.stepes.couchverse.design.theme.LocalReducedMotion
import io.stepes.couchverse.design.theme.Motion
import kotlinx.coroutines.delay

/**
 * The featured titles as full-width pages that advance every 8 seconds (plan 12.2), holding
 * still while a finger is on them. With reduced motion they only move when swiped.
 */
@Composable
fun HeroPager(
    count: Int,
    modifier: Modifier = Modifier,
    onPageShown: (Int) -> Unit = {},
    page: @Composable PagerScope.(index: Int) -> Unit,
) {
    val state = rememberPagerState { count }
    val dragged by state.interactionSource.collectIsDraggedAsState()
    val pressed by state.interactionSource.collectIsPressedAsState()
    val reducedMotion = LocalReducedMotion.current
    LaunchedEffect(state.settledPage) { onPageShown(state.settledPage) }
    if (count > 1 && !dragged && !pressed && !reducedMotion) {
        LaunchedEffect(state.settledPage) {
            delay(Tokens.Motion.heroIntervalMillis)
            state.animateScrollToPage((state.settledPage + 1) % count)
        }
    }
    Box(modifier) {
        HorizontalPager(state = state, beyondViewportPageCount = 1, pageContent = page)
        if (count > 1) {
            Row(
                Modifier
                    .align(Alignment.BottomCenter)
                    .padding(bottom = 8.dp)
                    .clearAndSetSemantics {},
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                repeat(count) { index ->
                    val active = index == state.currentPage
                    val width by animateDpAsState(if (active) 18.dp else 6.dp, Motion.snappy(), label = "dot")
                    val color by animateColorAsState(
                        if (active) LocalAccent.current.accent else Tokens.Palette.text.copy(alpha = 0.35f),
                        Motion.standard(),
                        label = "dot",
                    )
                    Box(Modifier.height(6.dp).width(width).clip(CircleShape).background(color))
                }
            }
        }
    }
}
