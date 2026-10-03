package io.stepes.couchverse.catalog

import io.stepes.couchverse.core.AccentPalette
import io.stepes.couchverse.core.BrowseKey
import io.stepes.couchverse.core.BrowseSort
import io.stepes.couchverse.core.BrowseView
import io.stepes.couchverse.core.Card
import io.stepes.couchverse.core.ContinueCard
import io.stepes.couchverse.core.EpisodeNumber
import io.stepes.couchverse.core.EpisodeView
import io.stepes.couchverse.core.FeaturedCard
import io.stepes.couchverse.core.GenreView
import io.stepes.couchverse.core.GenresView
import io.stepes.couchverse.core.HomeRowKind
import io.stepes.couchverse.core.HomeRowView
import io.stepes.couchverse.core.HomeView
import io.stepes.couchverse.core.Image
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.Logo
import io.stepes.couchverse.core.MyListView
import io.stepes.couchverse.core.PlayAction
import io.stepes.couchverse.core.PlayKind
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.core.Quality
import io.stepes.couchverse.core.SearchView
import io.stepes.couchverse.core.SeasonView
import io.stepes.couchverse.core.TitleDetailView
import io.stepes.couchverse.core.TitleKind
import io.stepes.couchverse.core.TitleView

/** View models as the core hands them out, for screens rendered without a server. */
object Fixtures {
    private const val ART = "https://media.example.com/api/v1/artwork/"

    private fun poster(id: String) = Image("${ART}poster-$id?size=w342", accent = "#3a6ea5")

    private fun backdrop(id: String, size: String? = "w780") =
        Image("${ART}backdrop-$id" + (size?.let { "?size=$it" } ?: ""), accent = "#7a3f2a")

    private val names = listOf(
        "Glass Harbor" to 2024, "Northern Lights" to 2021, "The Long Field" to 2019, "Paper Moons" to 2023,
        "Salt and Iron" to 2018, "Quiet Engines" to 2022, "Lantern Bay" to 2020, "Static Bloom" to 2025,
        "Under Ash" to 2017, "Kestrel" to 2024, "Night Ferry" to 2016, "Low Tide" to 2023,
    )

    val cards = names.mapIndexed { index, (name, year) ->
        val slug = name.lowercase().replace(' ', '-')
        Card(
            titleId = "t$index",
            slug = slug,
            name = name,
            kind = if (index % 3 == 1) TitleKind.Series else TitleKind.Movie,
            year = year,
            // one title without art shows the name on its placeholder
            poster = if (index == 5) null else poster(slug),
            backdrop = backdrop(slug),
        )
    }

    private val continueWatching = listOf(
        ContinueCard(
            titleId = "t1", slug = "northern-lights", name = "Northern Lights", kind = TitleKind.Series,
            episodeLabel = "S1 E3", positionSeconds = 1260u, durationSeconds = 2700u, progress = 0.46,
            play = PlayTarget(PlayKind.Episode, "e3"), backdrop = backdrop("northern-lights-e3"),
        ),
        ContinueCard(
            titleId = "t0", slug = "glass-harbor", name = "Glass Harbor", kind = TitleKind.Movie,
            positionSeconds = 4100u, durationSeconds = 6720u, progress = 0.61,
            play = PlayTarget(PlayKind.Movie, "t0"), backdrop = backdrop("glass-harbor"),
        ),
    )

    val featured = listOf(
        FeaturedCard(
            titleId = "t0", slug = "glass-harbor", name = "Glass Harbor", kind = TitleKind.Movie, year = 2024,
            overview = "A lighthouse keeper finds the harbor's lights answering back, and the town that forgot them starts to remember.",
            genres = listOf("Drama", "Mystery"), contentRating = "PG-13", runtimeMinutes = 112u,
            backdrop = backdrop("glass-harbor", null), logo = Logo("${ART}logo-glass-harbor?size=w780", 4.0), inList = false,
        ),
        FeaturedCard(
            titleId = "t1", slug = "northern-lights", name = "Northern Lights", kind = TitleKind.Series, year = 2021,
            overview = "Three siblings run a remote weather station.", genres = listOf("Drama"),
            backdrop = backdrop("northern-lights", null), inList = true,
        ),
    )

    fun home(status: LoadStatus = LoadStatus.Loaded, problem: Problem? = null) = HomeView(
        status = status,
        featured = featured,
        rows = listOf(
            HomeRowView("0-Continue Watching", HomeRowKind.ContinueWatching, "Continue Watching", emptyList(), continueWatching),
            HomeRowView("1-Up on the Marquee", HomeRowKind.RecentlyAdded, "Up on the Marquee", cards.take(8), emptyList()),
            HomeRowView("2-Late Night", HomeRowKind.Genre, "Late Night Thrillers", cards.drop(4), emptyList()),
        ),
        problem = problem,
    )

    val failedHome = HomeView(LoadStatus.Failed, emptyList(), emptyList(), Problem("offline", "simulated"))
    val loadingHome = HomeView(LoadStatus.Loading, emptyList(), emptyList())

    private fun episode(number: Int, progress: Double = 0.0, completed: Boolean = false) = EpisodeView(
        id = "e$number",
        number = number.toUInt(),
        name = listOf("Arrival", "The Station", "Whiteout", "Signals", "Thaw")[number - 1],
        overview = "The storm cuts the station off, and the radio picks up a voice that should not be there.",
        runtimeMinutes = 45u,
        still = Image("${ART}still-$number?size=w780"),
        progress = progress,
        completed = completed,
    )

    val series = TitleDetailView(
        id = "t1", slug = "northern-lights", name = "Northern Lights", kind = TitleKind.Series, year = 2021,
        overview = "Three siblings run a remote weather station through the longest winter on record.",
        genres = listOf("Drama", "Mystery"), contentRating = "TV-14",
        backdrop = backdrop("northern-lights", null), logo = Logo("${ART}logo-northern-lights?size=w780", 3.6),
        accent = AccentPalette("#2f8f9d", "#25707a", "#2f8f9d29", "#ffffff"),
        quality = Quality.Uhd, hdr = true, inList = true,
        play = PlayAction(PlayTarget(PlayKind.Episode, "e3"), resumeSeconds = 1260u, episode = EpisodeNumber(1u, 3u)),
        shuffle = true,
        seasons = listOf(
            SeasonView("s1", 1u, "Season 1", "", listOf(episode(1, completed = true, progress = 1.0), episode(2, completed = true, progress = 1.0), episode(3, 0.46), episode(4), episode(5))),
            SeasonView("s2", 2u, "Season 2", "", listOf(episode(1), episode(2))),
        ),
    )

    val movie = TitleDetailView(
        id = "t0", slug = "glass-harbor", name = "Glass Harbor", kind = TitleKind.Movie, year = 2024,
        overview = featured.first().overview, genres = listOf("Drama", "Mystery"), contentRating = "PG-13",
        runtimeMinutes = 112u, backdrop = backdrop("glass-harbor", null), quality = Quality.Hd1080, hdr = false,
        inList = false, play = PlayAction(PlayTarget(PlayKind.Movie, "t0")), shuffle = false, seasons = emptyList(),
    )

    fun title(detail: TitleDetailView) = TitleView(detail.slug, LoadStatus.Loaded, detail)

    val movies = BrowseView(
        key = BrowseKey(kind = TitleKind.Movie, sort = BrowseSort.Added),
        status = LoadStatus.Loaded, cards = cards, total = 40u, more = true, loadingMore = false,
    )

    val genres = GenresView(
        LoadStatus.Loaded,
        listOf("Action", "Comedy", "Documentary", "Drama", "Mystery", "Science Fiction", "Thriller", "Animation").map {
            GenreView(it, it)
        },
    )

    val emptyMyList = MyListView(LoadStatus.Loaded, emptyList())

    fun search(query: String, cards: List<Card> = Fixtures.cards.take(5)) = SearchView(query, LoadStatus.Loaded, cards)
}
