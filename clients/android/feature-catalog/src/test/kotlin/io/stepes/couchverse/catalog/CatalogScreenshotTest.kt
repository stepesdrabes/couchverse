package io.stepes.couchverse.catalog

import io.stepes.couchverse.core.BrowseSort
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.core.TitleView
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.Languages
import io.stepes.couchverse.testing.screenshot
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** The browse and title screens in every state, on a phone and a TV, in both languages. */
@RunWith(RobolectricTestRunner::class)
class CatalogScreenshotTest {
    private val navigation = CatalogNavigation(openTitle = {}, play = {})
    private val home = HomeActions(navigation, onListChange = { _, _ -> }, onRefresh = {})
    private val title = TitleActions(navigation, onListChange = {}, onRefresh = {}, onBack = {})
    private val browse = BrowseActions(navigation, onSort = {}, onGenre = {}, onMore = {}, onRefresh = {})

    @Test
    fun `home loaded`() = everywhere("home") { HomeScreen(Fixtures.home(), home) }

    @Test
    fun `home loading and failed`() = Device.entries.forEach { device ->
        screenshot("home_loading", device) { HomeScreen(Fixtures.loadingHome, home) }
        screenshot("home_failed", device) { HomeScreen(Fixtures.failedHome, home) }
    }

    @Test
    fun `home kept after a failed refresh`() =
        screenshot("home_stale", Device.Phone) { HomeScreen(Fixtures.home(LoadStatus.Stale, Problem("timeout", "")), home) }

    @Test
    fun `series title`() = everywhere("title_series") { TitleScreen(Fixtures.title(Fixtures.series), title) }

    @Test
    fun `movie title, loading and missing`() {
        Device.entries.forEach { device ->
            screenshot("title_movie", device) { TitleScreen(Fixtures.title(Fixtures.movie), title) }
            screenshot("title_loading", device) { TitleScreen(TitleView("x", LoadStatus.Loading), title) }
        }
        screenshot("title_not_found", Device.Phone, "cs") {
            TitleScreen(TitleView("x", LoadStatus.NotFound, problem = Problem("not_found", "")), title)
        }
    }

    @Test
    fun `movies with genre filters`() = everywhere("browse_movies") {
        BrowseScreen(BrowseState("Movies", Fixtures.movies, BrowseSort.Added, Fixtures.genres, genre = "Drama"), browse)
    }

    @Test
    fun `genres, my list and search`() = Device.entries.forEach { device ->
        screenshot("genres", device) { GenresScreen(Fixtures.genres, navigation, onRefresh = {}) }
        screenshot("my_list_empty", device) { MyListScreen(Fixtures.emptyMyList, navigation, onRefresh = {}) }
        screenshot("search", device) { SearchScreen(Fixtures.search("har"), onQuery = {}, navigation = navigation, autoFocus = false) }
        screenshot("search_empty", device, "cs") {
            SearchScreen(Fixtures.search("xyz", emptyList()), onQuery = {}, navigation = navigation, autoFocus = false)
        }
    }

    private fun everywhere(name: String, content: @androidx.compose.runtime.Composable () -> Unit) {
        Device.entries.forEach { device -> Languages.forEach { language -> screenshot(name, device, language, content = content) } }
    }
}
