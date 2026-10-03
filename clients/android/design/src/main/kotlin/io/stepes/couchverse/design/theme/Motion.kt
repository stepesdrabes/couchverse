package io.stepes.couchverse.design.theme

import android.content.Context
import android.provider.Settings
import androidx.compose.animation.core.AnimationSpec
import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.snap
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.staticCompositionLocalOf
import io.stepes.couchverse.design.Tokens

/** Whether the user asked for less motion ("Remove animations" sets the animator scale to 0). */
val LocalReducedMotion = staticCompositionLocalOf { false }

fun systemPrefersReducedMotion(context: Context): Boolean =
    Settings.Global.getFloat(context.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f

/**
 * The shared motion tokens (plan 12.4): short, interruptible springs and one entrance curve.
 * Every spec collapses to a cut when the user prefers reduced motion.
 */
object Motion {
    private fun Tokens.CurveToken.easing() = CubicBezierEasing(x1, y1, x2, y2)

    @Composable
    @ReadOnlyComposable
    fun <T> snappy(): FiniteAnimationSpec<T> = springOf(Tokens.Motion.snappy)

    @Composable
    @ReadOnlyComposable
    fun <T> smooth(): FiniteAnimationSpec<T> = springOf(Tokens.Motion.smooth)

    @Composable
    @ReadOnlyComposable
    fun <T> bouncy(): FiniteAnimationSpec<T> = springOf(Tokens.Motion.bouncy)

    @Composable
    @ReadOnlyComposable
    fun <T> enter(): FiniteAnimationSpec<T> = curveOf(Tokens.Motion.enter)

    @Composable
    @ReadOnlyComposable
    fun <T> standard(): FiniteAnimationSpec<T> = curveOf(Tokens.Motion.standard)

    /** A slow tween for ambient changes (a backdrop's tint following focus). */
    @Composable
    @ReadOnlyComposable
    fun <T> ambient(durationMillis: Int = 700): FiniteAnimationSpec<T> =
        if (LocalReducedMotion.current) snap() else tween(durationMillis, easing = Tokens.Motion.enter.easing())

    @Composable
    @ReadOnlyComposable
    private fun <T> springOf(token: Tokens.SpringToken): FiniteAnimationSpec<T> =
        if (LocalReducedMotion.current) snap() else spring(token.dampingRatio, token.stiffness)

    @Composable
    @ReadOnlyComposable
    private fun <T> curveOf(token: Tokens.CurveToken): FiniteAnimationSpec<T> =
        if (LocalReducedMotion.current) snap() else tween(token.durationMillis, easing = token.easing())
}

/** For APIs that take any [AnimationSpec] but should still honour reduced motion. */
@Composable
@ReadOnlyComposable
fun <T> AnimationSpec<T>.orCut(): AnimationSpec<T> = if (LocalReducedMotion.current) snap() else this
