package io.stepes.couchverse.design.theme

import androidx.compose.animation.AnimatedVisibilityScope
import androidx.compose.animation.ExperimentalSharedTransitionApi
import androidx.compose.animation.SharedTransitionScope
import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Modifier

/** The app's shared-element scope, provided around the root navigation. */
@OptIn(ExperimentalSharedTransitionApi::class)
val LocalSharedTransitionScope = staticCompositionLocalOf<SharedTransitionScope?> { null }

/** The animated scope of the root destination a composable is in. */
val LocalRootAnimatedScope = staticCompositionLocalOf<AnimatedVisibilityScope?> { null }

/**
 * Lets an account's avatar fly between root screens ("Who's watching?" into the TV sidebar,
 * plan 12.3). Without the scopes (previews, tests) or with reduced motion it stays put.
 */
@OptIn(ExperimentalSharedTransitionApi::class)
@Composable
fun Modifier.sharedAvatar(accountId: String): Modifier {
    val shared = LocalSharedTransitionScope.current ?: return this
    val animated = LocalRootAnimatedScope.current ?: return this
    if (LocalReducedMotion.current) return this
    return with(shared) {
        this@sharedAvatar.sharedElement(
            sharedContentState = rememberSharedContentState("avatar-$accountId"),
            animatedVisibilityScope = animated,
        )
    }
}
