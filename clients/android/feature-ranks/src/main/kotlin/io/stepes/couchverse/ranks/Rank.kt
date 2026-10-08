package io.stepes.couchverse.ranks

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.AchievementCard
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.RankBadge
import io.stepes.couchverse.core.RankView
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.theme.colorOf
import io.stepes.couchverse.ranks.phone.CelebrationPhone
import io.stepes.couchverse.ranks.tv.CelebrationTv

/** A tier's name in the display language, or its code for a tier this build does not know. */
@Composable
fun tierName(code: String): String = TierNames[code]?.let { stringResource(it) } ?: code

@Composable
fun achievementName(code: String): String = AchievementNames[code]?.let { stringResource(it) } ?: code

@Composable
fun achievementDescription(code: String): String? = AchievementDescriptions[code]?.let { stringResource(it) }

@Composable
fun xpSourceName(key: String): String = XpSources[key]?.let { stringResource(it) } ?: key

/** The tier and level, "Gold · Level 12". */
@Composable
fun rankLine(badge: RankBadge): String = "${tierName(badge.tier.code)} · ${stringResource(R.string.rank_level, badge.tier.level.toString())}"

/** The rank in a line: tier and level, and how far into the tier, in the tier's colour. */
@Composable
fun RankChip(badge: RankBadge, modifier: Modifier = Modifier) {
    val colour = colorOf(badge.tier.colour) ?: Tokens.Palette.accent
    Column(modifier.widthIn(max = 280.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(
            rankLine(badge),
            style = MaterialTheme.typography.labelLarge,
            color = Tokens.Palette.text,
        )
        Box(Modifier.fillMaxWidth().height(6.dp).clip(RoundedCornerShape(3.dp)).background(Tokens.Palette.surface2)) {
            Box(Modifier.fillMaxHeight().fillMaxWidth(badge.percent.toFloat() / 100f).background(colour))
        }
        Text(
            if (badge.next.level == badge.tier.level) {
                stringResource(R.string.rank_max_level)
            } else {
                "${stringResource(R.string.rank_xp_value, badge.xp.toString())} · ${stringResource(R.string.rank_to_next_level, badge.next.level.toString())}"
            },
            style = MaterialTheme.typography.bodySmall,
            color = Tokens.Palette.muted,
        )
    }
}

/**
 * An achievement just unlocked, over whatever is on screen, until it is dismissed; the core
 * queues the next one behind it.
 */
@Composable
fun CelebrationCard(card: AchievementCard, onDismiss: () -> Unit) {
    if (LocalIsTv.current) CelebrationTv(card, onDismiss) else CelebrationPhone(card, onDismiss)
}

/** The card both idioms show; [done] is the idiom's button that dismisses it. */
@Composable
internal fun Celebration(card: AchievementCard, done: @Composable () -> Unit) {
    Box(Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.55f)), contentAlignment = Alignment.Center) {
        Column(
            Modifier
                .padding(24.dp)
                .widthIn(max = 420.dp)
                .clip(RoundedCornerShape(Tokens.Radius.card))
                .background(Tokens.Palette.surface)
                .padding(28.dp)
                .semantics { liveRegion = LiveRegionMode.Polite },
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Icon(CouchverseIcons.Trophy, contentDescription = null, tint = tierColour(card.tier), modifier = Modifier.size(56.dp))
            Text(stringResource(R.string.achievement_unlocked), style = MaterialTheme.typography.labelLarge, color = Tokens.Palette.muted)
            Text(achievementName(card.code), style = MaterialTheme.typography.headlineSmall, color = Tokens.Palette.text, textAlign = TextAlign.Center)
            achievementDescription(card.code)?.let {
                Text(it, style = MaterialTheme.typography.bodyMedium, color = Tokens.Palette.muted, textAlign = TextAlign.Center)
            }
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(stringResource(achievementTier(card.tier)), color = tierColour(card.tier), style = MaterialTheme.typography.labelLarge)
                Text(stringResource(R.string.achievement_reward, card.xp.toString()), color = Tokens.Palette.text, style = MaterialTheme.typography.labelLarge)
            }
            done()
        }
    }
}

/** The celebration queue over the app, with rankings on. */
@Composable
fun CelebrationRoute() {
    val session by rememberSurface<SessionView>(Surface.Session)
    if (session?.features?.rankings != true) return
    val send = rememberSend()
    val rank by rememberSurface<RankView>(Surface.Rank)
    val card = rank?.celebration ?: return
    CelebrationCard(card, onDismiss = { send(Event.CelebrationDismissed) })
}

fun achievementTier(tier: String): Int = when (tier) {
    "silver" -> R.string.achievement_tier_silver
    "gold" -> R.string.achievement_tier_gold
    "platinum" -> R.string.achievement_tier_platinum
    else -> R.string.achievement_tier_bronze
}

/** A medal's colour, from the design tokens. */
fun tierColour(tier: String): Color = when (tier) {
    "silver" -> Tokens.Medal.silver.from
    "gold" -> Tokens.Medal.gold.from
    "platinum" -> Tokens.Medal.platinum.from
    else -> Tokens.Medal.bronze.from
}
