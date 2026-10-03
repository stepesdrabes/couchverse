package io.stepes.couchverse.design.components

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.design.theme.Motion

/** Which of a screen's states to show for a view model's load status. */
enum class Showing { Skeleton, Content, NotFound, Failed }

/**
 * Stale beats blank (plan 7.4): whatever the core still holds is shown, even while it reloads
 * or after a refresh failed; a skeleton only stands in before there is anything.
 */
fun showing(status: LoadStatus?): Showing = when (status) {
    LoadStatus.Loaded, LoadStatus.Stale -> Showing.Content
    LoadStatus.NotFound -> Showing.NotFound
    LoadStatus.Failed -> Showing.Failed
    null, LoadStatus.Idle, LoadStatus.Loading -> Showing.Skeleton
}

/** Cross-fades between a screen's skeleton, content and failure states. */
@Composable
fun LoadState(
    status: LoadStatus?,
    skeleton: @Composable () -> Unit,
    failed: @Composable () -> Unit,
    modifier: Modifier = Modifier,
    notFound: @Composable () -> Unit = failed,
    content: @Composable () -> Unit,
) {
    val enter = Motion.standard<Float>()
    AnimatedContent(
        targetState = showing(status),
        modifier = modifier,
        transitionSpec = { fadeIn(enter) togetherWith fadeOut(enter) },
        label = "load state",
    ) { state ->
        when (state) {
            Showing.Skeleton -> skeleton()
            Showing.Content -> content()
            Showing.NotFound -> notFound()
            Showing.Failed -> failed()
        }
    }
}
