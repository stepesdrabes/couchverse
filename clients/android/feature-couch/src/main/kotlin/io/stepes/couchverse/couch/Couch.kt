package io.stepes.couchverse.couch

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.key
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.stepes.couchverse.core.CouchMember
import io.stepes.couchverse.core.CouchRole
import io.stepes.couchverse.core.CouchStatus
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.Reaction
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.theme.LocalReducedMotion
import kotlin.math.absoluteValue

/** The reactions offered first; what the viewer sent lately comes before them. */
val QuickReactions = listOf("❤️", "😂", "😮", "😢", "👏", "🔥", "🍿", "👍", "🎉", "😱", "🤔", "😴")

/** Recent reactions first, then the quick ones not among them. */
fun reactionChoices(recent: List<String>): List<String> = (recent + QuickReactions).distinct()

/** Whether a couch session is going on (or getting there) for this device. */
val CouchView.active: Boolean get() = role != null && status != CouchStatus.Idle && status != CouchStatus.Ended

/** The one line that says what the session is doing, if it needs saying. */
@Composable
fun couchStatusText(view: CouchView): String? {
    val host = view.members.firstOrNull { it.host }
    return when {
        view.status == CouchStatus.Connecting -> stringResource(R.string.couch_connecting)
        view.status == CouchStatus.Reconnecting -> stringResource(R.string.couch_reconnecting)
        view.status == CouchStatus.Ended -> stringResource(
            if (view.ended == "host_ended") R.string.couch_session_ended_host else R.string.couch_session_ended_title,
        )
        view.role != CouchRole.Follower -> null
        view.hostAway -> stringResource(R.string.couch_host_away)
        view.waiting && view.media == null -> host?.let { stringResource(R.string.couch_host_choosing, it.displayName) }
            ?: stringResource(R.string.couch_host_choosing_generic)
        view.resynced -> stringResource(R.string.couch_resynced)
        !view.playing && !view.localPaused -> stringResource(R.string.couch_host_paused)
        else -> null
    }
}

/** A pill over the video for [couchStatusText]. */
@Composable
fun CouchStatusPill(view: CouchView, modifier: Modifier = Modifier) {
    val text = couchStatusText(view) ?: return
    androidx.compose.material3.Text(
        text,
        style = TextStyle(fontSize = 15.sp),
        color = Tokens.Palette.text,
        modifier = modifier
            .clip(RoundedCornerShape(50))
            .background(Tokens.Palette.surface.copy(alpha = 0.85f))
            .padding(horizontal = 16.dp, vertical = 8.dp)
            .semantics { liveRegion = LiveRegionMode.Polite },
    )
}

/** The members' avatars in a row, the host first. */
@Composable
fun CouchMembers(members: List<CouchMember>, modifier: Modifier = Modifier, size: androidx.compose.ui.unit.Dp = 32.dp) {
    Row(modifier, horizontalArrangement = Arrangement.spacedBy((-8).dp)) {
        members.sortedByDescending { it.host }.forEach { member ->
            Avatar(
                member.avatar?.url,
                seed = member.seed,
                size = size,
                modifier = Modifier.alpha(if (member.paused) 0.5f else 1f),
            )
        }
    }
}

/**
 * Reactions rising from the bottom edge and fading, one per reaction id, at a spot picked from
 * its id so every member's screen has it in the same place.
 */
@Composable
fun ReactionsOverlay(reactions: List<Reaction>, modifier: Modifier = Modifier) {
    val reduced = LocalReducedMotion.current
    BoxWithConstraints(modifier.fillMaxSize()) {
        val width = maxWidth
        val height = maxHeight
        reactions.forEach { reaction ->
            key(reaction.id) {
                val progress = remember { Animatable(0f) }
                LaunchedEffect(Unit) { progress.animateTo(1f, tween(RISE_MS, easing = LinearEasing)) }
                val lane = (reaction.id.toLong() * 37 % 70).absoluteValue / 100f + 0.15f
                androidx.compose.material3.Text(
                    reaction.emoji,
                    fontSize = 40.sp,
                    modifier = Modifier
                        .align(Alignment.BottomStart)
                        .offset(x = width * lane, y = if (reduced) (-80).dp else -(height * 0.6f * progress.value))
                        .alpha(1f - progress.value),
                )
            }
        }
    }
}

/** A row of reactions to send. */
@Composable
fun ReactionRow(recent: List<String>, onReact: (String) -> Unit, modifier: Modifier = Modifier, item: @Composable (String, () -> Unit) -> Unit) {
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(4.dp), verticalAlignment = Alignment.CenterVertically) {
        reactionChoices(recent).forEach { emoji -> item(emoji) { onReact(emoji) } }
    }
}

@Composable
internal fun MemberLine(member: CouchMember, modifier: Modifier = Modifier) {
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
        Avatar(member.avatar?.url, seed = member.seed, size = 36.dp)
        Column {
            androidx.compose.material3.Text(member.displayName, color = Tokens.Palette.text)
            val badges = listOfNotNull(
                stringResource(R.string.couch_host_badge).takeIf { member.host },
                stringResource(R.string.couch_you_badge).takeIf { member.me },
            )
            if (badges.isNotEmpty()) {
                androidx.compose.material3.Text(badges.joinToString(" · "), color = Tokens.Palette.muted, fontSize = 13.sp)
            }
        }
    }
}

private const val RISE_MS = 2_200
