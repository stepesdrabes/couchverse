import CouchverseCore
import CouchverseDesign
import Foundation

// Where a download stands, in the display language: the core hands over states, fractions and
// failure codes, the words are the shell's (the same strings as on Android).

enum DownloadLabels {
    /// The qualities offered, best first.
    static let qualities: [DownloadQuality] = [.original, .hd1080, .hd720, .sd480]

    static func quality(_ quality: DownloadQuality) -> String {
        quality == .original ? L10n.downloadQualityOriginal : quality.rawValue
    }

    /// `Downloading · 42%`, `Downloaded · 1.2 GB`, or why it failed.
    static func state(_ item: DownloadItem) -> String {
        switch item.state {
        case .queued:
            L10n.downloadStateQueued
        case .preparing:
            L10n.downloadStatePreparing(percent: percent(item.progress))
        case .fetching:
            L10n.downloadStateFetching(percent: percent(item.progress))
        case .ready:
            item.sizeBytes > 0
                ? "\(L10n.downloadStateReady) \u{00B7} \(size(bytes: item.sizeBytes))" : L10n.downloadStateReady
        case .failed:
            failure(item.problem?.code)
        }
    }

    /// The core's failure codes (`DownloadItem.problem`) in words.
    static func failure(_ code: String?) -> String {
        switch code {
        case "unsupported": L10n.downloadFailedUnsupported
        case "expired": L10n.downloadFailedExpired
        case "prepare_failed": L10n.downloadFailedPrepare
        case "no_space": L10n.downloadFailedNoSpace
        default: L10n.downloadFailedFetch
        }
    }

    /// `1.2 GB`, in the display language.
    static func size(bytes: UInt64) -> String {
        Int64(clamping: bytes).formatted(.byteCount(style: .file).locale(L10n.locale))
    }

    /// Whole percent, never past 100.
    static func percent(_ fraction: Double) -> String {
        String(Int((min(max(fraction, 0), 1) * 100).rounded(.down)))
    }

    /// The title, with the episode's name after it.
    static func name(_ item: DownloadItem) -> String {
        guard let episode = item.episodeName, !episode.isEmpty else { return item.title }
        return "\(item.title) \u{00B7} \(episode)"
    }
}
