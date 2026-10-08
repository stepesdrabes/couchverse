import Foundation

/// What a shell measured about this device's video and audio, once per launch: the facts the
/// device profile (plan 7.6) is built from. Kept separate from measuring, which needs the
/// hardware, so the profile itself is unit-tested.
public struct PlaybackMeasurements: Sendable, Equatable {
    /// `VTIsHardwareDecodeSupported(kCMVideoCodecType_HEVC)`.
    public var hevc: Bool
    /// HEVC Main 10 plays (`isPlayableExtendedMIMEType` with an `hvc1.2` codec string).
    public var hevcMain10: Bool
    /// The HDR formats the display path takes (`AVPlayer.eligibleForHDRPlayback`).
    public var hdr10: Bool
    public var hlg: Bool
    public var dolbyVision: Bool
    /// The largest picture the device should be sent, in pixels.
    public var maxWidth: Int
    public var maxHeight: Int
    public var maxFrameRate: Int
    /// Dolby Atmos (E-AC-3 JOC) reaches the output as Atmos: an Atmos-capable receiver or TV, or
    /// spatial audio rendering.
    public var atmos: Bool

    public init(
        hevc: Bool, hevcMain10: Bool, hdr10: Bool, hlg: Bool, dolbyVision: Bool, maxWidth: Int,
        maxHeight: Int, maxFrameRate: Int, atmos: Bool
    ) {
        self.hevc = hevc
        self.hevcMain10 = hevcMain10
        self.hdr10 = hdr10
        self.hlg = hlg
        self.dolbyVision = dolbyVision
        self.maxWidth = maxWidth
        self.maxHeight = maxHeight
        self.maxFrameRate = maxFrameRate
        self.atmos = atmos
    }
}

extension DeviceProfile {
    /// AVPlayer's profile: MP4 and MOV progressively, H.264 and (with a hardware decoder) HEVC,
    /// the audio Apple decodes or passes through, HLS in both segment formats, and audio tracks
    /// switchable inside a file. No sidecar subtitles: AVPlayer shows text only as HLS renditions,
    /// so files with subtitles are remuxed (D9).
    public static func avPlayer(_ m: PlaybackMeasurements) -> DeviceProfile {
        var video = [VideoSupport(codec: .h264, maxLevel: 5.2)]
        if m.hevc {
            video.append(
                VideoSupport(
                    codec: .hevc, profiles: m.hevcMain10 ? [.main, .main10] : [.main], maxLevel: 5.1,
                    maxBitDepth: m.hevcMain10 ? 10 : 8))
        }
        var hdr: [HdrFormat] = []
        // HDR is only worth sending with a 10-bit decoder to carry it
        if m.hevc && m.hevcMain10 {
            if m.hdr10 { hdr.append(.hdr10) }
            if m.hlg { hdr.append(.hlg) }
            // Apple decodes the HDR10- and HLG-compatible profiles, not dual-layer 7
            if m.dolbyVision { hdr += [.dolbyVision5, .dolbyVision8] }
        }
        return DeviceProfile(
            containers: [.mp4, .mov],
            video: video,
            audio: [
                AudioSupport(codec: .aac, maxChannels: 6),
                AudioSupport(codec: .ac3, maxChannels: 6),
                AudioSupport(codec: .eac3, maxChannels: 8, atmos: m.atmos),
                AudioSupport(codec: .flac, maxChannels: 8),
                AudioSupport(codec: .alac, maxChannels: 8),
            ],
            hdr: hdr,
            maxWidth: UInt32(clamping: m.maxWidth),
            maxHeight: UInt32(clamping: m.maxHeight),
            maxFrameRate: Double(m.maxFrameRate),
            hls: [.ts, .fmp4],
            audioTrackSwitching: true)
    }
}
