package io.stepes.couchverse.catalog

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.hasScrollToIndexAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollToNode
import androidx.compose.ui.test.performTextInput
import io.stepes.couchverse.core.PlayKind
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.Fixture
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import kotlin.test.assertEquals

@Config(qualifiers = RobolectricDeviceQualifiers.Pixel7)
@RunWith(RobolectricTestRunner::class)
class CatalogUiTest {
    @get:Rule
    val compose = createComposeRule()

    private val opened = mutableListOf<String>()
    private val played = mutableListOf<PlayTarget>()
    private val navigation = CatalogNavigation(openTitle = { opened += it }, play = { played += it })

    @Test
    fun `a title page resumes where the viewer stopped and toggles My List`() {
        val listed = mutableListOf<Boolean>()
        compose.setContent {
            Fixture(Device.Phone) {
                TitleScreen(Fixtures.title(Fixtures.series), TitleActions(navigation, onListChange = { listed += it }, onRefresh = {}, onBack = {}))
            }
        }
        compose.onNodeWithText("Resume S1 E3").performClick()
        compose.onNodeWithContentDescription("Remove from My List").performClick()
        compose.onAllNodes(hasScrollToIndexAction()).onFirst().performScrollToNode(hasText("Whiteout", substring = true))
        compose.onNodeWithText("Whiteout", substring = true).performClick()

        assertEquals(listOf(PlayTarget(PlayKind.Episode, "e3"), PlayTarget(PlayKind.Episode, "e3")), played)
        assertEquals(listOf(false), listed)
    }

    @Test
    fun `home cards open titles and continue watching plays`() {
        compose.setContent {
            Fixture(Device.Phone) { HomeScreen(Fixtures.home(), HomeActions(navigation, { _, _ -> }, {})) }
        }
        compose.onNodeWithText("Continue Watching").assertIsDisplayed()
        compose.onNodeWithContentDescription("S1 E3", substring = true).performClick()
        compose.onAllNodes(hasScrollToIndexAction()).onFirst().performScrollToNode(hasText("The Long Field"))
        compose.onNodeWithText("The Long Field").performClick()

        assertEquals(listOf(PlayTarget(PlayKind.Episode, "e3")), played)
        assertEquals(listOf("the-long-field"), opened)
    }

    @Test
    fun `search sends every change to the core`() {
        val queries = mutableListOf<String>()
        compose.setContent {
            Fixture(Device.Phone) { SearchScreen(null, onQuery = { queries += it }, navigation = navigation, autoFocus = false) }
        }
        compose.onNodeWithText("Search your library").assertIsDisplayed()
        compose.onNodeWithText("Search movies and series…").performTextInput("glass")
        assertEquals(listOf("glass"), queries)
    }

    @Test
    fun `the display language changes the words`() {
        compose.setContent {
            Fixture(Device.Phone, language = "cs") {
                TitleScreen(Fixtures.title(Fixtures.movie), TitleActions(navigation, {}, {}, null))
            }
        }
        compose.onNodeWithText("Přehrát").assertIsDisplayed()
    }
}
