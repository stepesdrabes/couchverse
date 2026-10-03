package io.stepes.couchverse.downloads

import io.stepes.couchverse.core.DownloadItem
import io.stepes.couchverse.core.DownloadQuality
import io.stepes.couchverse.core.DownloadState
import io.stepes.couchverse.core.DownloadsView
import io.stepes.couchverse.core.EpisodeNumber
import io.stepes.couchverse.core.Image
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.PlayKind
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.Languages
import io.stepes.couchverse.testing.screenshot
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import kotlin.test.assertEquals

/** The phone's downloads in every state, online and off. */
@RunWith(RobolectricTestRunner::class)
class DownloadsScreenshotTest {
    private val actions = DownloadsActions(onPlay = {}, onRetry = {}, onRemove = {}, onBack = {})

    private fun item(id: String, state: DownloadState, progress: Double = 0.0, problem: String? = null, episode: Boolean = false) = DownloadItem(
        id = id,
        target = PlayTarget(if (episode) PlayKind.Episode else PlayKind.Movie, "t-$id"),
        title = if (episode) "Night Shift" else "Glass Harbor",
        titleSlug = "glass-harbor",
        episode = if (episode) EpisodeNumber(1u, 2u) else null,
        episodeName = if (episode) "The Keeper" else null,
        quality = DownloadQuality.Hd720,
        state = state,
        progress = progress,
        sizeBytes = if (state == DownloadState.Ready) 734_003_200u else 0u,
        image = Image("https://media.example.com/api/v1/artwork/b-$id?size=w780"),
        problem = problem?.let { Problem(it, "") },
    )

    private val view = DownloadsView(
        status = LoadStatus.Loaded,
        items = listOf(
            item("d1", DownloadState.Fetching, 0.42, episode = true),
            item("d2", DownloadState.Preparing, 0.18),
            item("d3", DownloadState.Queued),
            item("d4", DownloadState.Ready),
            item("d5", DownloadState.Failed, problem = "no_space"),
        ),
        usedBytes = 734_003_200u,
    )

    @Test
    fun `downloads in every state`() = Languages.forEach { language ->
        screenshot("downloads", Device.Phone, language) { DownloadsScreen(view, offline = false, directory = null, actions = actions) }
    }

    @Test
    fun `offline and empty`() {
        screenshot("downloads_offline", Device.Phone) { DownloadsScreen(view, offline = true, directory = null, actions = actions) }
        screenshot("downloads_empty", Device.Phone, "cs") {
            DownloadsScreen(DownloadsView(LoadStatus.Loaded, emptyList(), 0u), offline = false, directory = null, actions = actions)
        }
    }

    @Test
    fun `every failure has its words`() {
        val codes = listOf("unsupported", "expired", "prepare_failed", "fetch_failed", "no_space")
        assertEquals(codes.size, codes.map(::failureText).toSet().size)
    }
}
