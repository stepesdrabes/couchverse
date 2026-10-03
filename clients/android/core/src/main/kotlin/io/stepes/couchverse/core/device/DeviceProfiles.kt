package io.stepes.couchverse.core.device

import io.stepes.couchverse.core.AudioCodec
import io.stepes.couchverse.core.AudioSupport
import io.stepes.couchverse.core.Container
import io.stepes.couchverse.core.DeviceProfile
import io.stepes.couchverse.core.HdrFormat
import io.stepes.couchverse.core.HlsFormat
import io.stepes.couchverse.core.SubtitleFormat
import io.stepes.couchverse.core.VideoCodec
import io.stepes.couchverse.core.VideoSupport

/**
 * What the platform says it can decode and show, as plain data: [measureDevice] reads it from
 * `MediaCodecList`, the display and the audio output, and [deviceProfile] turns it into the
 * profile the core sends with playback requests (plan 7.6).
 */
data class DeviceCaps(
    val decoders: List<Decoder>,
    /** `Display.HdrCapabilities.HDR_TYPE_*` the screen shows. */
    val displayHdr: Set<Int>,
    /** `AudioFormat.ENCODING_*` the audio output takes undecoded (HDMI passthrough). */
    val passthrough: Set<Int>,
)

data class Decoder(
    val mime: String,
    /** `MediaCodecInfo.CodecProfileLevel` profiles the decoder handles. */
    val profiles: Set<Int>,
    val maxWidth: Int = 0,
    val maxHeight: Int = 0,
    /** Channels it decodes, for audio. */
    val maxChannels: Int = 0,
)

/**
 * The profile for Media3's ExoPlayer on this device: what it plays progressively (MP4, Matroska,
 * WebM, both HLS segment formats, sidecar WebVTT, switchable audio tracks), narrowed to the codecs
 * and HDR formats the hardware has.
 */
fun deviceProfile(caps: DeviceCaps): DeviceProfile {
    val byMime = caps.decoders.groupBy { it.mime }
    fun profiles(mime: String) = byMime[mime].orEmpty().flatMapTo(mutableSetOf()) { it.profiles }

    val video = VIDEO.mapNotNull { (mime, codec) ->
        if (mime !in byMime) return@mapNotNull null
        val tenBit = profiles(mime).any { it in TEN_BIT[codec].orEmpty() }
        VideoSupport(codec, maxBitDepth = if (tenBit) 10u else null)
    }

    val hevc = profiles(MIME_HEVC)
    val vp9 = profiles(MIME_VP9)
    val av1 = profiles(MIME_AV1)
    val dolby = profiles(MIME_DOLBY_VISION)
    val hdr = buildList {
        val hdr10 = HEVC_MAIN10_HDR10 in hevc || VP9_PROFILE2_HDR in vp9 || AV1_MAIN10_HDR10 in av1
        if (HDR_TYPE_HDR10 in caps.displayHdr && hdr10) add(HdrFormat.Hdr10)
        val hdr10Plus = HEVC_MAIN10_HDR10_PLUS in hevc || VP9_PROFILE2_HDR10_PLUS in vp9 || AV1_MAIN10_HDR10_PLUS in av1
        if (HDR_TYPE_HDR10_PLUS in caps.displayHdr && hdr10Plus) add(HdrFormat.Hdr10plus)
        // HLG rides on plain 10-bit HEVC or VP9
        val tenBit = HEVC_MAIN10 in hevc || hdr10 || VP9_PROFILE2 in vp9
        if (HDR_TYPE_HLG in caps.displayHdr && tenBit) add(HdrFormat.Hlg)
        if (HDR_TYPE_DOLBY_VISION in caps.displayHdr) {
            DOLBY_VISION.forEach { (profile, format) -> if (profile in dolby) add(format) }
        }
    }

    val decoded = AUDIO.mapNotNull { (mime, codec) ->
        val decoders = byMime[mime] ?: return@mapNotNull null
        val channels = decoders.maxOf { it.maxChannels }.coerceIn(2, MAX_CHANNELS)
        AudioSupport(codec, maxChannels = channels.toUByte(), atmos = if (mime == MIME_EAC3_JOC) true else null)
    }
    val passed = PASSTHROUGH.mapNotNull { (encoding, support) -> support.takeIf { encoding in caps.passthrough } }
    val audio = (decoded + passed)
        .groupBy { it.codec }
        .map { (codec, entries) ->
            AudioSupport(
                codec,
                maxChannels = entries.maxOf { it.maxChannels ?: 2u },
                atmos = if (entries.any { it.atmos == true }) true else null,
            )
        }

    val sizes = caps.decoders.filter { it.mime in VIDEO }
    return DeviceProfile(
        containers = listOf(Container.Mp4, Container.Mkv, Container.Webm),
        video = video,
        audio = audio,
        hdr = hdr,
        maxWidth = sizes.maxOfOrNull { it.maxWidth }?.takeIf { it > 0 }?.toUInt(),
        maxHeight = sizes.maxOfOrNull { it.maxHeight }?.takeIf { it > 0 }?.toUInt(),
        hls = listOf(HlsFormat.Fmp4, HlsFormat.Ts),
        sidecarSubtitles = listOf(SubtitleFormat.Webvtt),
        audioTrackSwitching = true,
    )
}

private const val MAX_CHANNELS = 8
private const val MIME_HEVC = "video/hevc"
private const val MIME_VP9 = "video/x-vnd.on2.vp9"
private const val MIME_AV1 = "video/av01"
private const val MIME_DOLBY_VISION = "video/dolby-vision"
private const val MIME_EAC3_JOC = "audio/eac3-joc"

private val VIDEO = mapOf(
    "video/avc" to VideoCodec.H264,
    MIME_HEVC to VideoCodec.Hevc,
    MIME_VP9 to VideoCodec.Vp9,
    MIME_AV1 to VideoCodec.Av1,
)

private val AUDIO = mapOf(
    "audio/mp4a-latm" to AudioCodec.Aac,
    "audio/mpeg" to AudioCodec.Mp3,
    "audio/ac3" to AudioCodec.Ac3,
    "audio/eac3" to AudioCodec.Eac3,
    MIME_EAC3_JOC to AudioCodec.Eac3,
    "audio/opus" to AudioCodec.Opus,
    "audio/vorbis" to AudioCodec.Vorbis,
    "audio/flac" to AudioCodec.Flac,
    "audio/alac" to AudioCodec.Alac,
    "audio/raw" to AudioCodec.Pcm,
)

// MediaCodecInfo.CodecProfileLevel values, spelled out so the mapping runs on the JVM
private const val HEVC_MAIN10 = 0x2
private const val HEVC_MAIN10_HDR10 = 0x1000
private const val HEVC_MAIN10_HDR10_PLUS = 0x2000
private const val VP9_PROFILE2 = 0x4
private const val VP9_PROFILE2_HDR = 0x1000
private const val VP9_PROFILE2_HDR10_PLUS = 0x4000
private const val AV1_MAIN10 = 0x2
private const val AV1_MAIN10_HDR10 = 0x1000
private const val AV1_MAIN10_HDR10_PLUS = 0x2000

private val TEN_BIT = mapOf(
    VideoCodec.Hevc to setOf(HEVC_MAIN10, HEVC_MAIN10_HDR10, HEVC_MAIN10_HDR10_PLUS),
    VideoCodec.Vp9 to setOf(VP9_PROFILE2, VP9_PROFILE2_HDR, VP9_PROFILE2_HDR10_PLUS, 0x8, 0x2000, 0x8000),
    VideoCodec.Av1 to setOf(AV1_MAIN10, AV1_MAIN10_HDR10, AV1_MAIN10_HDR10_PLUS),
)

/** Dolby Vision profile flags and the profile numbers the server knows them by. */
private val DOLBY_VISION = listOf(
    0x20 to HdrFormat.DolbyVision5,
    0x80 to HdrFormat.DolbyVision7,
    0x100 to HdrFormat.DolbyVision8,
    0x400 to HdrFormat.DolbyVision10,
)

// Display.HdrCapabilities.HDR_TYPE_*
internal const val HDR_TYPE_DOLBY_VISION = 1
internal const val HDR_TYPE_HDR10 = 2
internal const val HDR_TYPE_HLG = 3
internal const val HDR_TYPE_HDR10_PLUS = 4

/** AudioFormat encodings sent undecoded to a receiver, and what each carries. */
private val PASSTHROUGH = listOf(
    5 to AudioSupport(AudioCodec.Ac3, maxChannels = 6u),
    6 to AudioSupport(AudioCodec.Eac3, maxChannels = 8u),
    18 to AudioSupport(AudioCodec.Eac3, maxChannels = 8u, atmos = true),
    7 to AudioSupport(AudioCodec.Dts, maxChannels = 6u),
    14 to AudioSupport(AudioCodec.Truehd, maxChannels = 8u, atmos = true),
)
