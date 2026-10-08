import AVFoundation
import CouchverseCore
import UIKit
import VideoToolbox

/// Measures what this device plays (plan 7.6), for the profile the core sends with every
/// playback request. Read once the window is up, and again when the audio route changes (a
/// receiver or AirPods can bring or take Atmos).
enum DeviceCapabilities {
    static func measure(screen: UIScreen?) -> PlaybackMeasurements {
        let hevc = VTIsHardwareDecodeSupported(kCMVideoCodecType_HEVC)
        // Main 10, level 5.1, the profile HDR10 and Dolby Vision 8.1 ride on
        let main10 = AVURLAsset.isPlayableExtendedMIMEType(#"video/mp4; codecs="hvc1.2.4.L153.B0""#)
        // every Apple device eligible for HDR takes all three formats, so one answer covers them
        let hdr = AVPlayer.eligibleForHDRPlayback
        let session = AVAudioSession.sharedInstance()
        let spatial = session.currentRoute.outputs.contains { $0.isSpatialAudioEnabled }
        let (width, height) = maxPicture(screen: screen)
        return PlaybackMeasurements(
            hevc: hevc, hevcMain10: hevc && main10,
            hdr10: hdr, hlg: hdr, dolbyVision: hdr,
            maxWidth: width, maxHeight: height,
            maxFrameRate: max(screen?.maximumFramesPerSecond ?? 60, 30),
            atmos: spatial || session.supportsMultichannelContent)
    }

    /// A TV gets what its display shows. A phone or tablet decodes 4K and scales it down, which
    /// costs the server nothing, where a smaller cap would make it transcode.
    private static func maxPicture(screen: UIScreen?) -> (Int, Int) {
        #if os(tvOS)
            guard let bounds = screen?.nativeBounds, bounds.width > 0 else { return (1920, 1080) }
            return (Int(max(bounds.width, bounds.height)), Int(min(bounds.width, bounds.height)))
        #else
            return (3840, 2160)
        #endif
    }

    /// The screen of the app's window scene, once one is connected.
    static var currentScreen: UIScreen? {
        UIApplication.shared.connectedScenes.lazy.compactMap { ($0 as? UIWindowScene)?.screen }.first
    }
}
