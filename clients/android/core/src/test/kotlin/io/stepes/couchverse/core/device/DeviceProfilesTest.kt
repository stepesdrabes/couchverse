package io.stepes.couchverse.core.device

import io.stepes.couchverse.core.AudioCodec
import io.stepes.couchverse.core.AudioSupport
import io.stepes.couchverse.core.CoreJson
import io.stepes.couchverse.core.DeviceProfile
import io.stepes.couchverse.core.HdrFormat
import io.stepes.couchverse.core.VideoCodec
import io.stepes.couchverse.core.VideoSupport
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class DeviceProfilesTest {
    /** A Google TV streamer on an HDR TV with an Atmos soundbar. */
    private val googleTv = DeviceCaps(
        decoders = listOf(
            Decoder("video/avc", setOf(1, 2, 8), 4096, 2176),
            Decoder("video/hevc", setOf(1, 2, 0x1000, 0x2000), 4096, 2176),
            Decoder("video/x-vnd.on2.vp9", setOf(1, 4, 0x1000), 4096, 2176),
            Decoder("video/av01", setOf(1, 2, 0x1000), 3840, 2160),
            Decoder("video/dolby-vision", setOf(0x20, 0x100)),
            Decoder("audio/mp4a-latm", emptySet(), maxChannels = 8),
            Decoder("audio/ac3", emptySet(), maxChannels = 6),
            Decoder("audio/eac3", emptySet(), maxChannels = 6),
            Decoder("audio/opus", emptySet(), maxChannels = 8),
        ),
        displayHdr = setOf(HDR_TYPE_DOLBY_VISION, HDR_TYPE_HDR10, HDR_TYPE_HLG),
        passthrough = setOf(5, 6, 18),
    )

    @Test
    fun `a 4K HDR streamer gets ten bit codecs, its HDR formats and Atmos passthrough`() {
        val profile = deviceProfile(googleTv)

        assertEquals(
            listOf(
                VideoSupport(VideoCodec.H264),
                VideoSupport(VideoCodec.Hevc, maxBitDepth = 10u),
                VideoSupport(VideoCodec.Vp9, maxBitDepth = 10u),
                VideoSupport(VideoCodec.Av1, maxBitDepth = 10u),
            ),
            profile.video,
        )
        // HDR10+ decodes, but the screen does not show it
        assertEquals(
            listOf(HdrFormat.Hdr10, HdrFormat.Hlg, HdrFormat.DolbyVision5, HdrFormat.DolbyVision8),
            profile.hdr,
        )
        assertEquals(AudioSupport(AudioCodec.Eac3, maxChannels = 8u, atmos = true), profile.audio.single { it.codec == AudioCodec.Eac3 })
        assertEquals(AudioSupport(AudioCodec.Aac, maxChannels = 8u), profile.audio.single { it.codec == AudioCodec.Aac })
        assertEquals(4096u, profile.maxWidth)
    }

    @Test
    fun `an SDR phone gets no HDR and only stereo for what it cannot decode`() {
        val phone = DeviceCaps(
            decoders = listOf(
                Decoder("video/avc", setOf(1, 2, 8), 1920, 1088),
                Decoder("video/hevc", setOf(1, 0x1000), 3840, 2160),
                Decoder("audio/mp4a-latm", emptySet(), maxChannels = 2),
            ),
            displayHdr = emptySet(),
            passthrough = emptySet(),
        )
        val profile = deviceProfile(phone)

        assertEquals(emptyList(), profile.hdr)
        assertEquals(listOf(AudioSupport(AudioCodec.Aac, maxChannels = 2u)), profile.audio)
        assertEquals(VideoSupport(VideoCodec.Hevc, maxBitDepth = 10u), profile.video.single { it.codec == VideoCodec.Hevc })
    }

    @Test
    fun `the profile encodes as the core and the fixtures expect`() {
        val json = CoreJson.encodeToString(DeviceProfile.serializer(), deviceProfile(googleTv))
        // the shared fixture parses with the same types, so both sides speak one shape
        val fixture = File("../../../contract/fixtures/device-profiles/android-tv.json").readText()
        CoreJson.decodeFromString(DeviceProfile.serializer(), fixture)
        assertTrue(json.contains(""""hdr":["hdr10","hlg","dolbyVision5","dolbyVision8"]"""))
        assertTrue(json.contains(""""hls":["fmp4","ts"]"""))
    }
}
