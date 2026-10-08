package io.stepes.couchverse.playback

import android.os.Looper
import androidx.media3.common.C
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.test.utils.FakeClock
import androidx.media3.test.utils.robolectric.ShadowMediaCodecConfig
import androidx.media3.test.utils.robolectric.TestPlayerRunHelper
import androidx.test.core.app.ApplicationProvider
import io.stepes.couchverse.core.NowPlaying
import io.stepes.couchverse.core.PlayerCommand
import io.stepes.couchverse.core.PlayerLoad
import io.stepes.couchverse.core.PlayerReport
import io.stepes.couchverse.core.PlayerSeek
import io.stepes.couchverse.core.PlayerSource
import io.stepes.couchverse.core.PlayerSubtitle
import io.stepes.couchverse.core.SubtitleSelection
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import okio.Buffer
import org.junit.After
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** The engine with a real ExoPlayer decoding a real clip, its codecs faked by Robolectric. */
@RunWith(RobolectricTestRunner::class)
class PlaybackEngineTest {
    @get:Rule
    val codecs = ShadowMediaCodecConfig.withAllDefaultSupportedCodecs()

    @get:Rule
    val folder = TemporaryFolder()

    private val context = ApplicationProvider.getApplicationContext<android.content.Context>()
    private val reports = mutableListOf<PlayerReport>()
    private val server = MockWebServer()
    private lateinit var engine: PlaybackEngine

    @Before
    fun setUp() {
        server.start()
        folder.root.resolve("d1.mp4").writeBytes(clip())
        engine = PlaybackEngine(
            context = context,
            downloads = folder.root,
            report = { reports += it },
            build = {
                ExoPlayer.Builder(it)
                    .setClock(FakeClock(true))
                    .setMediaSourceFactory(mediaSources(it, OkHttpClient()))
                    .build()
            },
        )
    }

    @After
    fun tearDown() {
        engine.perform(PlayerCommand.Stop)
        idle()
        server.close()
    }

    @Test
    fun `a download plays from the device and reports until it ends`() {
        engine.perform(PlayerCommand.Load(load("d1.mp4", PlayerSource.Download)))
        idle()
        val player = assertNotNull(engine.player.value)
        advance(player).untilState(Player.STATE_ENDED)
        idle()

        assertTrue(reports.any { it.playing }, "it reported playing")
        val last = reports.last()
        assertEquals(true, last.ended)
        assertEquals(2.0, last.durationSeconds, 0.1)
        assertEquals(null, last.failed)
    }

    @Test
    fun `commands pause, seek and resume the player`() {
        engine.perform(PlayerCommand.Load(load("d1.mp4", PlayerSource.Download, autoplay = false)))
        idle()
        val player = assertNotNull(engine.player.value)
        advance(player).untilState(Player.STATE_READY)
        assertEquals(false, player.playWhenReady)

        engine.perform(PlayerCommand.Seek(PlayerSeek(1.5)))
        idle()
        advance(player).untilPendingCommandsAreFullyHandled()
        idle()
        assertEquals(1.5, reports.last().positionSeconds, 0.05)

        engine.perform(PlayerCommand.Play)
        idle()
        assertTrue(player.playWhenReady)
        engine.perform(PlayerCommand.Pause)
        idle()
        assertEquals(false, player.playWhenReady)
    }

    @Test
    fun `a refused stream is reported with the server's status`() {
        server.enqueue(MockResponse.Builder().code(403).build())
        engine.perform(PlayerCommand.Load(load(server.url("/media/g/stream").toString(), PlayerSource.File)))
        idle()
        val player = assertNotNull(engine.player.value)
        advance(player).untilPlayerError()
        idle()
        assertEquals(1, server.requestCount, "a refusal is not retried")

        assertEquals("http_403", reports.last().failed)
    }

    @Test
    fun `a pinned quality caps the height and stop releases the player`() {
        engine.perform(PlayerCommand.Load(load("d1.mp4", PlayerSource.Download, maxHeight = 720u)))
        idle()
        val player = assertNotNull(engine.player.value)
        assertEquals(720, player.trackSelectionParameters.maxVideoHeight)
        assertEquals("Glass Harbor", player.mediaMetadata.title.toString())

        engine.perform(PlayerCommand.Stop)
        idle()
        assertNull(engine.player.value)
        assertNull(engine.load.value)
    }

    @Test
    fun `sidecar subtitles are offered and shown when chosen`() {
        val vtt = server.url("/subtitles/en.vtt").toString()
        repeat(2) { server.enqueue(MockResponse.Builder().body(Buffer().writeUtf8(VTT)).build()) }
        val subtitles = listOf(PlayerSubtitle("s-en", "en", "English", vtt, forced = false))
        engine.perform(PlayerCommand.Load(load("d1.mp4", PlayerSource.Download, autoplay = false, subtitles = subtitles)))
        idle()
        val player = assertNotNull(engine.player.value)
        advance(player).untilState(Player.STATE_READY)
        assertTrue(player.trackSelectionParameters.disabledTrackTypes.contains(C.TRACK_TYPE_TEXT), "off until chosen")

        engine.perform(PlayerCommand.SelectSubtitles(SubtitleSelection("s-en")))
        idle()
        advance(player).untilPendingCommandsAreFullyHandled()
        assertTrue(player.currentTracks.isTypeSelected(C.TRACK_TYPE_TEXT))

        engine.perform(PlayerCommand.SelectSubtitles(SubtitleSelection(null)))
        idle()
        assertTrue(player.trackSelectionParameters.disabledTrackTypes.contains(C.TRACK_TYPE_TEXT))
    }

    private fun idle() = shadowOf(Looper.getMainLooper()).idle()

    /** A loaded build machine decodes slowly; the helpers' 10 s default is not a statement about the engine. */
    private fun advance(player: ExoPlayer) = TestPlayerRunHelper.advance(player).withTimeoutMs(60_000)

    private fun clip(): ByteArray = javaClass.classLoader!!.getResourceAsStream("clip.mp4")!!.readBytes()

    private fun load(
        url: String,
        source: PlayerSource,
        autoplay: Boolean = true,
        maxHeight: UInt? = null,
        subtitles: List<PlayerSubtitle> = emptyList(),
    ) = PlayerLoad(
        url = url,
        source = source,
        startSeconds = 0.0,
        autoplay = autoplay,
        maxHeight = maxHeight,
        subtitles = subtitles,
        linear = false,
        nowPlaying = NowPlaying("Glass Harbor", durationSeconds = 2.0),
    )

    private companion object {
        const val VTT = "WEBVTT\n\n00:00.000 --> 00:02.000\nHello\n"
    }
}
