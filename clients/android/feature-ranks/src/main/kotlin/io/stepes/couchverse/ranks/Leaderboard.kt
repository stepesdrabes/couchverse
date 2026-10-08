package io.stepes.couchverse.ranks

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.LeaderRow
import io.stepes.couchverse.core.LeaderboardKey
import io.stepes.couchverse.core.LeaderboardView
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.Metric
import io.stepes.couchverse.core.Period
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.components.StatusMessage
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.text.displayLocale
import io.stepes.couchverse.design.text.formatRuntime
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.ranks.phone.LeaderboardPhone
import io.stepes.couchverse.ranks.tv.LeaderboardTv

/**
 * Who leads, by XP, watch time or achievements, over all time, this month or this week. The
 * top three earn a medal; the viewer's own row is marked.
 */
@Composable
fun LeaderboardScreen(view: LeaderboardView?, key: LeaderboardKey, onKey: (LeaderboardKey) -> Unit, onProfile: (String) -> Unit, onBack: (() -> Unit)?) {
    if (LocalIsTv.current) LeaderboardTv(view, key, onKey, onProfile) else LeaderboardPhone(view, key, onKey, onProfile, onBack)
}

/** A choice of board drawn by the idiom; [first] is the first of them, where a TV's focus starts. */
internal typealias BoardChip = @Composable (selected: Boolean, label: String, onClick: () -> Unit, first: Boolean) -> Unit

/**
 * The board as one list, the same on both idioms: [title], the metric and period drawn with
 * [chip], then the rows, each in the idiom's [line] that opens the member's profile, or why
 * there are none.
 */
@Composable
internal fun LeaderboardList(
    view: LeaderboardView?,
    key: LeaderboardKey,
    onKey: (LeaderboardKey) -> Unit,
    gutter: Dp,
    contentPadding: PaddingValues,
    modifier: Modifier = Modifier,
    title: @Composable () -> Unit,
    chip: BoardChip,
    line: @Composable (row: LeaderRow, content: @Composable () -> Unit) -> Unit,
) {
    LazyColumn(
        Modifier.fillMaxSize().then(modifier),
        contentPadding = contentPadding,
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        item(key = "title") { title() }
        item(key = "metrics") {
            Row(Modifier.padding(horizontal = gutter), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Metric.entries.forEachIndexed { index, metric ->
                    chip(key.metric == metric, metricLabel(metric), { onKey(key.copy(metric = metric)) }, index == 0)
                }
            }
        }
        item(key = "periods") {
            Row(Modifier.padding(horizontal = gutter), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                if (key.metric == Metric.Xp) {
                    Text(stringResource(R.string.leaderboard_period_locked), style = MaterialTheme.typography.bodySmall, color = Tokens.Palette.muted)
                } else {
                    Period.entries.forEach { period ->
                        chip(key.period == period, periodLabel(period), { onKey(key.copy(period = period)) }, false)
                    }
                }
            }
        }
        when {
            view == null || view.status == LoadStatus.Loading -> item(key = "loading") {
                Box(Modifier.fillMaxWidth().padding(48.dp), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            }
            view.rows.isEmpty() || view.allZero -> item(key = "empty") {
                StatusMessage(stringResource(R.string.leaderboard_empty_title), Modifier.fillMaxWidth(), message = stringResource(R.string.leaderboard_empty_message))
            }
            else -> {
                if (view.hidden) {
                    item(key = "hidden") {
                        Text(stringResource(R.string.leaderboard_hidden_notice), color = Tokens.Palette.muted, modifier = Modifier.padding(horizontal = gutter))
                    }
                }
                view.myPosition?.let { position ->
                    item(key = "mine") {
                        Text(
                            stringResource(R.string.leaderboard_your_position, position.toString(), view.total.toString()),
                            color = Tokens.Palette.text,
                            modifier = Modifier.padding(horizontal = gutter),
                        )
                    }
                }
                items(view.rows, key = { it.username }) { row -> line(row) { LeaderLine(row, key.metric, view.podium) } }
            }
        }
    }
}

/** The heading both idioms start the board with. */
@Composable
internal fun LeaderboardHeading() {
    Text(
        stringResource(R.string.leaderboard_heading),
        style = MaterialTheme.typography.headlineSmall,
        color = Tokens.Palette.text,
        modifier = Modifier.semantics { heading() },
    )
}

/** A member's place, picture, name, rank and score. */
@Composable
private fun LeaderLine(row: LeaderRow, metric: Metric, podium: Boolean) {
    val medal = if (podium && row.position <= 3u) listOf("gold", "silver", "bronze")[row.position.toInt() - 1] else null
    Row(Modifier.padding(12.dp), horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(
            "#${row.position}",
            style = MaterialTheme.typography.titleMedium,
            color = medal?.let(::tierColour) ?: Tokens.Palette.muted,
            fontWeight = FontWeight.Bold,
            modifier = Modifier.width(44.dp),
        )
        Avatar(row.avatar?.url, seed = row.username, size = 36.dp)
        Column(Modifier.weight(1f)) {
            Text(row.displayName, color = Tokens.Palette.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(
                "${tierName(row.tierCode)} · ${stringResource(R.string.rank_level, row.level.toString())}",
                style = MaterialTheme.typography.bodySmall,
                color = Tokens.Palette.muted,
            )
        }
        Text(metricValue(row, metric), style = MaterialTheme.typography.titleSmall, color = Tokens.Palette.text)
    }
}

/** The viewer's own row stands out from the others. */
internal fun lineBackground(row: LeaderRow) = if (row.isSelf) Tokens.Palette.surface2 else Tokens.Palette.surface

@Composable
private fun metricLabel(metric: Metric): String = stringResource(
    when (metric) {
        Metric.Xp -> R.string.leaderboard_metric_xp
        Metric.Watch -> R.string.leaderboard_metric_watch
        Metric.Achievements -> R.string.leaderboard_metric_achievements
    },
)

@Composable
private fun periodLabel(period: Period): String = stringResource(
    when (period) {
        Period.All -> R.string.leaderboard_period_all
        Period.Month -> R.string.leaderboard_period_month
        Period.Week -> R.string.leaderboard_period_week
    },
)

@Composable
private fun metricValue(row: LeaderRow, metric: Metric): String = when (metric) {
    Metric.Xp -> stringResource(R.string.rank_xp_value, row.value.toString())
    Metric.Watch -> formatRuntime((row.value / 60u).toInt(), displayLocale())
    Metric.Achievements -> row.value.toString()
}

/** [LeaderboardScreen] over the core; the board chosen is kept across visits. */
@Composable
fun LeaderboardRoute(onProfile: (String) -> Unit, onBack: (() -> Unit)?) {
    var metric by rememberSaveable { mutableStateOf(Metric.Xp) }
    var period by rememberSaveable { mutableStateOf(Period.All) }
    val key = LeaderboardKey(period, metric)
    val view by rememberSurface<LeaderboardView>(Surface.Leaderboard(key), open = true)
    LeaderboardScreen(
        view,
        key,
        onKey = {
            metric = it.metric
            period = it.period
        },
        onProfile = onProfile,
        onBack = onBack,
    )
}
