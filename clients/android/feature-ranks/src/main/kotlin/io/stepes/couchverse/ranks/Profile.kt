package io.stepes.couchverse.ranks

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.AchievementCard
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.Heatmap
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.ProfileDetail
import io.stepes.couchverse.core.ProfileView
import io.stepes.couchverse.core.PublicChoice
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.core.TopTitle
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Artwork
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.components.MarkdownView
import io.stepes.couchverse.design.components.StatusMessage
import io.stepes.couchverse.design.phone.PhoneGutter
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.text.displayLocale
import io.stepes.couchverse.design.text.formatRuntime
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.design.theme.LocalAccent
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvSafe
import java.time.LocalDate
import java.time.OffsetDateTime
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.util.Locale

class ProfileActions(
    val onBack: (() -> Unit)?,
    val onEdit: () -> Unit,
    val onPublic: (Boolean) -> Unit,
    val onOpenTitle: (slug: String) -> Unit,
    val onRetry: () -> Unit,
)

/**
 * A member's profile: the banner and avatar, their rank, bio and numbers, when they watch (the
 * year's activity and the hours of the day), what they watch most, achievements and where
 * their XP comes from. Their own adds editing and the switch for being public.
 */
@Composable
fun ProfileScreen(view: ProfileView?, actions: ProfileActions) {
    val profile = view?.profile
    Box(Modifier.fillMaxSize()) {
        when {
            profile != null -> ProfileContent(profile, actions)
            view == null || view.status == LoadStatus.Loading -> CircularProgressIndicator(Modifier.align(Alignment.Center))
            view.status == LoadStatus.NotFound -> StatusMessage(stringResource(R.string.profiles_not_found), Modifier.align(Alignment.Center))
            else -> StatusMessage(
                stringResource(R.string.error_page_title),
                Modifier.align(Alignment.Center),
                message = problemMessage(view.problem),
                action = { OutlinedButton(onClick = actions.onRetry) { Text(stringResource(R.string.common_retry)) } },
            )
        }
        if (!LocalIsTv.current) {
            actions.onBack?.let { back ->
                IconButton(onClick = back, modifier = Modifier.statusBarsPadding().padding(8.dp)) {
                    Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back))
                }
            }
        }
    }
}

@Composable
private fun ProfileContent(profile: ProfileDetail, actions: ProfileActions) {
    val tv = LocalIsTv.current
    val gutter = if (tv) TvSafe.horizontal else PhoneGutter
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = 32.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
        item(key = "header") { Header(profile, actions, gutter) }
        if (profile.bio.blocks.isNotEmpty()) {
            item(key = "bio") { Section(gutter) { MarkdownView(profile.bio) } }
        }
        item(key = "stats") { Section(gutter) { Stats(profile) } }
        item(key = "activity") {
            Section(gutter) {
                Heading(R.string.profiles_activity_heading)
                if (profile.heatmap.activeDays == 0u) {
                    Text(stringResource(R.string.profiles_no_activity), color = Tokens.Palette.muted)
                } else {
                    HeatmapGrid(profile.heatmap)
                }
            }
        }
        if (profile.hours.any { it > 0u }) {
            item(key = "clock") {
                Section(gutter) {
                    Heading(R.string.profiles_clock_heading)
                    WatchClock(profile.hours)
                }
            }
        }
        if (profile.topTitles.isNotEmpty()) {
            item(key = "top") {
                Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    Box(Modifier.padding(horizontal = gutter)) { Heading(R.string.profiles_top_titles_heading) }
                    LazyRow(contentPadding = PaddingValues(horizontal = gutter), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        items(profile.topTitles, key = { it.slug }) { title -> TopTitleCard(title, actions.onOpenTitle) }
                    }
                }
            }
        }
        item(key = "achievements") {
            Section(gutter) {
                Heading(R.string.achievement_heading)
                Text(
                    stringResource(R.string.achievement_count, profile.achievementsWon.toString(), profile.achievements.size.toString()),
                    color = Tokens.Palette.muted,
                )
                Achievements(profile.achievements)
            }
        }
        if (profile.xpSources.isNotEmpty()) {
            item(key = "xp") {
                Section(gutter) {
                    Heading(R.string.rank_sources_heading)
                    profile.xpSources.forEach { line ->
                        Row(Modifier.fillMaxWidth()) {
                            Text(xpSourceName(line.key), color = Tokens.Palette.text, modifier = Modifier.weight(1f))
                            Text(stringResource(R.string.rank_xp_value, line.xp.toString()), color = Tokens.Palette.muted)
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun Header(profile: ProfileDetail, actions: ProfileActions, gutter: androidx.compose.ui.unit.Dp) {
    val tv = LocalIsTv.current
    Column {
        Box(Modifier.fillMaxWidth().height(if (tv) 200.dp else 160.dp)) {
            Artwork(profile.banner?.url, accent = profile.banner?.accent, modifier = Modifier.fillMaxSize())
        }
        Row(
            Modifier.padding(horizontal = gutter).offset(y = (-36).dp),
            horizontalArrangement = Arrangement.spacedBy(16.dp),
            verticalAlignment = Alignment.Bottom,
        ) {
            Avatar(profile.avatar?.url, seed = profile.username, size = 88.dp, modifier = Modifier.border(3.dp, Tokens.Palette.bg, RoundedCornerShape(50)))
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(
                    profile.displayName,
                    style = MaterialTheme.typography.headlineSmall,
                    color = Tokens.Palette.text,
                    modifier = Modifier.semantics { heading() },
                )
                Text("@${profile.username}", color = Tokens.Palette.muted)
                memberSince(profile.memberSince, displayLocale())?.let {
                    Text(stringResource(R.string.profiles_joined, it), style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.muted)
                }
            }
        }
        Column(Modifier.padding(horizontal = gutter).offset(y = (-20).dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            RankChip(profile.rank)
            if (profile.isSelf) {
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
                    if (tv) {
                        TvActionButton(stringResource(R.string.profiles_edit_profile), onClick = actions.onEdit, icon = Icons.Filled.Edit)
                        TvActionButton(
                            stringResource(R.string.profiles_public_label),
                            onClick = { actions.onPublic(!profile.public) },
                            icon = if (profile.public) Icons.Filled.Check else Icons.Filled.Close,
                        )
                    } else {
                        OutlinedButton(onClick = actions.onEdit) {
                            Icon(Icons.Filled.Edit, contentDescription = null, modifier = Modifier.size(18.dp))
                            Spacer(Modifier.width(8.dp))
                            Text(stringResource(R.string.profiles_edit_profile))
                        }
                        Spacer(Modifier.weight(1f))
                        Text(stringResource(R.string.profiles_public_label), color = Tokens.Palette.text)
                        Switch(checked = profile.public, onCheckedChange = actions.onPublic)
                    }
                }
                if (!profile.public) {
                    Text(stringResource(R.string.profiles_private_self_notice), style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.muted)
                }
            }
        }
    }
}

/**
 * A block of the page. On a TV it takes focus, so the remote can scroll a page that is mostly
 * reading; the focused block is outlined.
 */
@Composable
private fun Section(gutter: androidx.compose.ui.unit.Dp, content: @Composable () -> Unit) {
    val tv = LocalIsTv.current
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    Column(
        Modifier
            .padding(horizontal = gutter)
            .fillMaxWidth()
            .then(
                if (tv) {
                    Modifier
                        .clip(RoundedCornerShape(Tokens.Radius.card))
                        .border(2.dp, if (focused) Tokens.Palette.text else Tokens.Palette.edge, RoundedCornerShape(Tokens.Radius.card))
                        .focusable(interactionSource = interaction)
                        .padding(16.dp)
                } else {
                    Modifier
                },
            ),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) { content() }
}

@Composable
private fun Heading(label: Int) {
    Text(stringResource(label), style = MaterialTheme.typography.titleMedium, color = Tokens.Palette.text, modifier = Modifier.semantics { heading() })
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Stats(profile: ProfileDetail) {
    val totals = profile.totals
    val locale = displayLocale()
    val stats = listOfNotNull(
        stringResource(R.string.profiles_stat_watch_time) to formatRuntime((totals.watchSeconds / 60u).toInt(), locale),
        stringResource(R.string.profiles_stat_titles) to (totals.moviesCompleted + totals.seriesCompleted).toString(),
        stringResource(R.string.profiles_stat_episodes) to totals.episodesCompleted.toString(),
        stringResource(R.string.profiles_stat_current_streak) to pluralStringResource(R.plurals.profiles_streak_days, totals.currentStreak.toInt(), totals.currentStreak.toInt()),
        stringResource(R.string.profiles_stat_longest_streak) to pluralStringResource(R.plurals.profiles_streak_days, totals.longestStreak.toInt(), totals.longestStreak.toInt()),
        stringResource(R.string.profiles_stat_couch) to totals.couchHosted.toString(),
        profile.favouriteGenre.takeIf { it.isNotBlank() }?.let { stringResource(R.string.profiles_stat_favourite_genre) to it },
    )
    FlowRow(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        stats.forEach { (label, value) ->
            Column(
                Modifier.width(150.dp).clip(RoundedCornerShape(Tokens.Radius.card)).background(Tokens.Palette.surface).padding(12.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                Text(value, style = MaterialTheme.typography.titleLarge, color = Tokens.Palette.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(label, style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.muted)
            }
        }
    }
}

/** The year as weeks of seven days, each day as bright as how much was watched. */
@Composable
private fun HeatmapGrid(heatmap: Heatmap) {
    val accent = LocalAccent.current.accent
    val from = runCatching { LocalDate.parse(heatmap.from) }.getOrNull()
    // weeks start on Monday; the first column is padded to the first day's weekday
    val lead = from?.dayOfWeek?.value?.minus(1) ?: 0
    val cells = List(lead) { null } + heatmap.days
    val weeks = cells.chunked(7)
    Row(horizontalArrangement = Arrangement.spacedBy(2.dp)) {
        weeks.takeLast(WEEKS).forEach { week ->
            Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                week.forEach { day ->
                    val level = day?.level?.toInt() ?: -1
                    Box(
                        Modifier
                            .size(7.dp)
                            .clip(RoundedCornerShape(1.dp))
                            .background(if (level <= 0) Tokens.Palette.surface2.copy(alpha = if (level < 0) 0f else 1f) else accent.copy(alpha = 0.25f + 0.1875f * level)),
                    )
                }
            }
        }
    }
    Text(
        pluralStringResource(R.plurals.profiles_activity_summary, heatmap.activeDays.toInt(), (heatmap.totalSeconds / 3600u).toString(), heatmap.activeDays.toInt()),
        style = MaterialTheme.typography.bodySmall,
        color = Tokens.Palette.muted,
    )
}

/** Watch time by the hour of the day, as 24 bars. */
@Composable
private fun WatchClock(hours: List<ULong>) {
    val most = hours.maxOrNull()?.takeIf { it > 0u } ?: return
    val accent = LocalAccent.current.accent
    Row(Modifier.fillMaxWidth().height(72.dp), horizontalArrangement = Arrangement.spacedBy(3.dp), verticalAlignment = Alignment.Bottom) {
        hours.forEach { seconds ->
            Box(
                Modifier
                    .weight(1f)
                    .fillMaxHeight((seconds.toFloat() / most.toFloat()).coerceAtLeast(0.03f))
                    .clip(RoundedCornerShape(topStart = 2.dp, topEnd = 2.dp))
                    .background(accent),
            )
        }
    }
    Row(Modifier.fillMaxWidth()) {
        listOf(0, 6, 12, 18).forEach { hour ->
            Text(stringResource(R.string.profiles_clock_hour, hour.toString()), style = MaterialTheme.typography.labelSmall, color = Tokens.Palette.muted, modifier = Modifier.weight(1f))
        }
    }
}

@Composable
private fun TopTitleCard(title: TopTitle, onOpen: (String) -> Unit) {
    val tv = LocalIsTv.current
    val width = if (tv) 120.dp else 96.dp
    if (tv) {
        io.stepes.couchverse.design.tv.TvPosterCard(
            name = title.name,
            posterUrl = title.poster?.url,
            accent = title.poster?.accent,
            caption = formatRuntime((title.seconds / 60u).toInt(), displayLocale()),
            onClick = { onOpen(title.slug) },
            width = width,
        )
    } else {
        Column(
            Modifier.width(width).clickable(role = Role.Button) { onOpen(title.slug) },
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Artwork(
                title.poster?.url,
                accent = title.poster?.accent,
                fallbackName = title.name,
                modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(RoundedCornerShape(10.dp)),
            )
            Text(title.name, style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(formatRuntime((title.seconds / 60u).toInt(), displayLocale()), style = MaterialTheme.typography.labelSmall, color = Tokens.Palette.muted)
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Achievements(cards: List<AchievementCard>) {
    FlowRow(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        cards.sortedByDescending { it.unlocked }.forEach { card -> AchievementTile(card) }
    }
}

@Composable
private fun AchievementTile(card: AchievementCard) {
    val colour = tierColour(card.tier)
    Column(
        Modifier
            .width(160.dp)
            .clip(RoundedCornerShape(Tokens.Radius.card))
            .background(Tokens.Palette.surface)
            .border(1.dp, if (card.unlocked) colour.copy(alpha = 0.6f) else Tokens.Palette.edge, RoundedCornerShape(Tokens.Radius.card))
            .padding(12.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Icon(io.stepes.couchverse.design.components.CouchverseIcons.Trophy, contentDescription = null, tint = if (card.unlocked) colour else Tokens.Palette.faint)
        Text(achievementName(card.code), style = MaterialTheme.typography.titleSmall, color = Tokens.Palette.text, maxLines = 2, overflow = TextOverflow.Ellipsis)
        achievementDescription(card.code)?.let {
            Text(it, style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.muted, maxLines = 3, overflow = TextOverflow.Ellipsis)
        }
        if (card.unlocked) {
            Text(stringResource(achievementTier(card.tier)), style = MaterialTheme.typography.labelSmall, color = colour)
        } else {
            LinearProgressIndicator(progress = { card.percent.toFloat() / 100f }, modifier = Modifier.fillMaxWidth())
            Text(
                stringResource(R.string.achievement_progress, card.value.toString(), card.target.toString()),
                style = MaterialTheme.typography.labelSmall,
                color = Tokens.Palette.muted,
            )
        }
    }
}

/** "3 Oct 2025" for the server's date or timestamp; `null` when it does not parse. */
fun memberSince(value: String, locale: Locale): String? {
    val date = runCatching { OffsetDateTime.parse(value).toLocalDate() }.getOrNull()
        ?: runCatching { LocalDate.parse(value.take(10)) }.getOrNull()
        ?: return null
    return date.format(DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM).withLocale(locale))
}

/** The weeks of activity a phone has room for; the TV shows the same. */
private const val WEEKS = 26

/** [ProfileScreen] over the core. */
@Composable
fun ProfileRoute(username: String, onEdit: () -> Unit, onOpenTitle: (String) -> Unit, onBack: (() -> Unit)?) {
    val send = rememberSend()
    val view by rememberSurface<ProfileView>(Surface.Profile(username), open = true)
    ProfileScreen(
        view,
        ProfileActions(
            onBack = onBack,
            onEdit = onEdit,
            onPublic = { send(Event.ProfileVisibilityChanged(PublicChoice(it))) },
            onOpenTitle = onOpenTitle,
            onRetry = { send(Event.RefreshRequested(Surface.Profile(username))) },
        ),
    )
}
