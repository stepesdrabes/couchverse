package io.stepes.couchverse.catalog

import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.ui.res.stringResource
import io.stepes.couchverse.core.Card
import io.stepes.couchverse.core.ContinueCard
import io.stepes.couchverse.core.EpisodeNumber
import io.stepes.couchverse.core.FeaturedCard
import io.stepes.couchverse.core.HomeRowKind
import io.stepes.couchverse.core.HomeRowView
import io.stepes.couchverse.core.PlayKind
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.core.Quality
import io.stepes.couchverse.core.TitleKind
import io.stepes.couchverse.design.R

/** Where catalog screens send the user; the app's navigation implements it. */
class CatalogNavigation(
    val openTitle: (slug: String) -> Unit,
    val play: (PlayTarget) -> Unit,
    val openGenre: (name: String, label: String) -> Unit = { _, _ -> },
)

/** A hero's main button: a movie plays, a series opens on its episodes (as on the web). */
fun FeaturedCard.primaryAction(navigation: CatalogNavigation) {
    if (kind == TitleKind.Movie) navigation.play(PlayTarget(PlayKind.Movie, titleId)) else navigation.openTitle(slug)
}

/**
 * Built-in rows carry English default labels from the server; those are shown in the display
 * language, while a label an admin customised is shown as written (the web does the same).
 */
@Composable
@ReadOnlyComposable
fun rowLabel(row: HomeRowView): String = when {
    row.kind == HomeRowKind.ContinueWatching && row.label == "Continue Watching" ->
        stringResource(R.string.home_row_continue_watching)
    row.kind == HomeRowKind.RecentlyAdded && row.label == "Up on the Marquee" ->
        stringResource(R.string.home_row_recently_added)
    else -> row.label
}

/** A card's second line: its year. */
fun Card.caption(): String? = year?.toString()

@Composable
@ReadOnlyComposable
fun ContinueCard.progressLabel(): String =
    stringResource(R.string.catalog_progress, (progress * 100).toInt().toString())

@Composable
@ReadOnlyComposable
fun episodeLabel(number: EpisodeNumber): String =
    stringResource(R.string.catalog_episode_short, number.season.toString(), number.episode.toString())

/** The badge for a title's best file; these are the same in every language. */
fun Quality.label(): String = when (this) {
    Quality.Uhd -> "4K"
    Quality.Hd1080 -> "1080p"
    Quality.Hd720 -> "720p"
    Quality.Sd -> "SD"
}
