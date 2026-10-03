package io.stepes.couchverse.playback

import io.stepes.couchverse.core.CouchMember
import io.stepes.couchverse.core.CouchRole
import io.stepes.couchverse.core.CouchStatus
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.Image
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.NextUp
import io.stepes.couchverse.core.PlayKind
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.core.PlayerEpisode
import io.stepes.couchverse.core.PlayerSeason
import io.stepes.couchverse.core.PlayerView
import io.stepes.couchverse.core.Problem
import io.stepes.couchverse.core.QualityKind
import io.stepes.couchverse.core.QualityOption
import io.stepes.couchverse.core.TrackOption
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.Languages
import io.stepes.couchverse.testing.screenshot
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** The player's controls over no video, on a phone and a TV, in both languages. */
@RunWith(RobolectricTestRunner::class)
class PlayerScreenshotTest {
    private val actions = PlayerActions(
        onBack = {},
        onTogglePlay = {},
        onSeek = {},
        onQuality = {},
        onAudio = {},
        onSubtitles = {},
        onEpisode = {},
        onNextEpisode = {},
        onCancelNext = {},
        onShuffle = {},
        onRetry = {},
        onPictureInPicture = {},
    )

    @Test
    fun `an episode with its controls and the next one counting down`() = Device.entries.forEach { device ->
        Languages.forEach { language ->
            screenshot("player_episode", device, language) { PlayerScreen(PlayerState(Fixtures.episode, player = null, couchEnabled = true), actions) }
        }
    }

    @Test
    fun `preparing, failed and a couch follower waiting`() = Device.entries.forEach { device ->
        screenshot("player_preparing", device) { PlayerScreen(PlayerState(Fixtures.episode.copy(status = LoadStatus.Loading, preparing = 42u, nextUp = null), null), actions) }
        screenshot("player_failed", device, "cs") {
            PlayerScreen(PlayerState(Fixtures.episode.copy(status = LoadStatus.Failed, nextUp = null, problem = Problem("playback_failed", "")), null), actions)
        }
        screenshot("player_couch", device) {
            PlayerScreen(PlayerState(Fixtures.episode.copy(linear = true, nextUp = null), null, couch = Fixtures.following, couchEnabled = true), actions)
        }
    }
}

internal object Fixtures {
    private fun episode(id: String, number: UInt, name: String, current: Boolean = false) =
        PlayerEpisode(id, number, name, Image("https://media.example.com/api/v1/artwork/still-$id?size=w780"), current)

    val episode = PlayerView(
        status = LoadStatus.Loaded,
        target = PlayTarget(PlayKind.Episode, "e2"),
        title = "Night Shift",
        subtitle = "S1 E2 · The Keeper",
        titleSlug = "night-shift",
        qualities = listOf(
            QualityOption("original", QualityKind.Original),
            QualityOption("auto", QualityKind.Auto),
            QualityOption("1080p", QualityKind.Rendition, 1080u),
            QualityOption("720p", QualityKind.Rendition, 720u),
        ),
        quality = "original",
        audio = listOf(TrackOption("a-en", "en", "English"), TrackOption("a-cs", "cs", "Čeština")),
        audioSelected = "a-en",
        subtitles = listOf(TrackOption("s-en", "en", "English"), TrackOption("s-cs", "cs", "Čeština")),
        subtitleSelected = "s-cs",
        seasons = listOf(
            PlayerSeason(1u, listOf(episode("e1", 1u, "Lights Out"), episode("e2", 2u, "The Keeper", current = true), episode("e3", 3u, "Fog Horn"))),
        ),
        nextUp = NextUp(PlayTarget(PlayKind.Episode, "e3"), 1u, 3u, "Fog Horn", countdownSeconds = 12u, shuffled = false),
        shuffleAvailable = true,
        shuffle = false,
        linear = false,
    )

    val following = CouchView(
        status = CouchStatus.Open,
        role = CouchRole.Follower,
        code = "123456",
        members = listOf(
            CouchMember("p1", "Nora", seed = "nora", host = true, anonymous = false, paused = false, me = false),
            CouchMember("p2", "Otto", seed = "otto", host = false, anonymous = false, paused = false, me = true),
        ),
        media = PlayTarget(PlayKind.Episode, "e2"),
        playing = true,
        positionSeconds = 100.0,
        positionAtMs = 0u,
        hostAway = true,
        waiting = true,
        localPaused = false,
        reactions = emptyList(),
        recentEmojis = emptyList(),
        resynced = false,
    )
}
