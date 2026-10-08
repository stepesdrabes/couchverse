import CouchverseCore
import CouchverseDesign
import Foundation

// The core hands over codes and numbers, never UI words; these are their labels in the display
// language, as on the web (`features/catalog/labels.ts`).

enum CatalogLabels {
    static func kind(_ kind: TitleKind) -> String {
        kind == .series ? L10n.catalogKindSeries : L10n.catalogKindMovie
    }

    static func quality(_ quality: Quality) -> String {
        switch quality {
        case .uhd: L10n.catalogQualityUhd
        case .hd1080: L10n.catalogQualityHd1080
        case .hd720: L10n.catalogQualityHd720
        case .sd: L10n.catalogQualitySd
        }
    }

    /// The built-in rows keep their English default labels on the server; a label the admin
    /// customized is shown as it is.
    static func row(_ row: HomeRowView) -> String {
        switch (row.kind, row.label) {
        case (.continueWatching, "Continue Watching"): L10n.homeRowContinueWatching
        case (.recentlyAdded, "Up on the Marquee"): L10n.homeRowRecentlyAdded
        default: row.label
        }
    }

    static func sort(_ sort: BrowseSort) -> String {
        switch sort {
        case .added: L10n.catalogSortRecentlyAdded
        case .name: L10n.catalogSortName
        case .year: L10n.catalogSortYear
        }
    }

    /// `1 h 52 min`, in the display language.
    static func runtime(minutes: UInt32) -> String {
        Duration.seconds(Int(minutes) * 60).formatted(
            .units(allowed: [.hours, .minutes], width: .abbreviated).locale(L10n.locale))
    }

    /// `1:02:03` or `12:34`.
    static func clock(seconds: UInt64) -> String {
        let pattern: Duration.TimeFormatStyle.Pattern =
            seconds >= 3600 ? .hourMinuteSecond : .minuteSecond
        return Duration.seconds(Int(seconds)).formatted(.time(pattern: pattern))
    }

    /// `S1 E3`, the same short form the web shows on its play button.
    static func episode(_ number: EpisodeNumber) -> String {
        "S\(number.season) E\(number.episode)"
    }

    /// A series names the episode it plays rather than a time, as on the web.
    static func playLabel(_ action: PlayAction, kind: TitleKind) -> String {
        if let resume = action.resumeSeconds, kind == .movie {
            return L10n.catalogResumeFrom(time: clock(seconds: resume))
        }
        if let episode = action.episode {
            return L10n.catalogPlayEpisode(label: self.episode(episode))
        }
        return L10n.commonPlay
    }

    /// Year, rating and length, joined by middle dots.
    static func facts(year: Int32?, rating: String?, runtime: UInt32?, kind: TitleKind? = nil) -> String {
        var parts: [String] = []
        if let kind { parts.append(self.kind(kind)) }
        if let year { parts.append(String(year)) }
        if let rating { parts.append(rating) }
        if let runtime { parts.append(self.runtime(minutes: runtime)) }
        return parts.joined(separator: " \u{00B7} ")
    }

    static func notice(_ code: String) -> String {
        switch code {
        case "watchlist_failed": L10n.catalogMyListUpdateFailed
        case "visibility_failed": L10n.profilesPrivacyFailed
        case "download_unsupported": L10n.downloadFailedUnsupported
        case "downloads_disabled": L10n.downloadsDisabled
        default: L10n.problemGeneric
        }
    }
}
