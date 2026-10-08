import Foundation
import Testing

@testable import CouchverseCore

struct DeviceProfileTests {
    static let appleTV4K = PlaybackMeasurements(
        hevc: true, hevcMain10: true, hdr10: true, hlg: true, dolbyVision: true, maxWidth: 3840,
        maxHeight: 2160, maxFrameRate: 60, atmos: true)

    /// The fixture the server's decision tests use for an Apple TV 4K.
    static func fixture(_ name: String) throws -> DeviceProfile {
        let root = URL(filePath: #filePath).deletingLastPathComponent().appending(path: "../../../../../..")
        let url = root.appending(path: "contract/fixtures/device-profiles/\(name).json").standardized
        return try JSONDecoder().decode(DeviceProfile.self, from: Data(contentsOf: url))
    }

    @Test func anAppleTV4KMatchesTheSharedFixture() throws {
        #expect(DeviceProfile.avPlayer(Self.appleTV4K) == (try Self.fixture("apple-tv-4k")))
    }

    @Test func withoutAnHevcDecoderOnlyH264AndSdrAreOffered() {
        var m = Self.appleTV4K
        m.hevc = false
        let profile = DeviceProfile.avPlayer(m)
        #expect(profile.video.map(\.codec) == [.h264])
        #expect(profile.hdr == [])
    }

    @Test func an8BitHevcDecoderTakesNoHdr() throws {
        var m = Self.appleTV4K
        m.hevcMain10 = false
        let profile = DeviceProfile.avPlayer(m)
        let hevc = profile.video.first { $0.codec == .hevc }
        #expect(hevc?.profiles == [.main])
        #expect(hevc?.maxBitDepth == 8)
        #expect(profile.hdr == [])
    }

    @Test func anSdrDisplayGetsNoHdrAndTheScreensSize() {
        let phone = PlaybackMeasurements(
            hevc: true, hevcMain10: true, hdr10: false, hlg: false, dolbyVision: false, maxWidth: 1920,
            maxHeight: 1080, maxFrameRate: 30, atmos: false)
        let profile = DeviceProfile.avPlayer(phone)
        #expect(profile.hdr == [])
        #expect(profile.maxWidth == 1920 && profile.maxHeight == 1080 && profile.maxFrameRate == 30)
        #expect(profile.audio.first { $0.codec == .eac3 }?.atmos == false)
        #expect(profile.sidecarSubtitles == nil, "AVPlayer shows text only as HLS renditions")
    }
}
