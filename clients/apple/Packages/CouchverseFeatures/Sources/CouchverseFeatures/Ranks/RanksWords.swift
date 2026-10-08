import CouchverseCore
import CouchverseDesign
import Foundation

// The core names achievements, tiers, XP sources and categories by stable codes and hands over
// numbers; these are their words in the display language, as on the web
// (`features/ranks/labels.ts`). A code this build does not know yet still reads as something.

enum RanksWords {
    static func achievementName(_ code: String) -> String {
        switch code {
        case "first_play": L10n.achievementFirstPlayName
        case "watch_10h": L10n.achievementWatch10hName
        case "watch_50h": L10n.achievementWatch50hName
        case "watch_200h": L10n.achievementWatch200hName
        case "watch_500h": L10n.achievementWatch500hName
        case "movies_25": L10n.achievementMovies25Name
        case "episodes_100": L10n.achievementEpisodes100Name
        case "series_done_1": L10n.achievementSeriesDone1Name
        case "series_done_10": L10n.achievementSeriesDone10Name
        case "marathon_6h": L10n.achievementMarathon6hName
        case "streak_3": L10n.achievementStreak3Name
        case "streak_7": L10n.achievementStreak7Name
        case "streak_30": L10n.achievementStreak30Name
        case "night_owl_10": L10n.achievementNightOwl10Name
        case "early_bird_10": L10n.achievementEarlyBird10Name
        case "active_days_50": L10n.achievementActiveDays50Name
        case "active_days_200": L10n.achievementActiveDays200Name
        case "genres_10": L10n.achievementGenres10Name
        case "titles_50": L10n.achievementTitles50Name
        case "watchlist_10": L10n.achievementWatchlist10Name
        case "decades_4": L10n.achievementDecades4Name
        case "couch_host_1": L10n.achievementCouchHost1Name
        case "couch_host_25": L10n.achievementCouchHost25Name
        case "couch_join_10": L10n.achievementCouchJoin10Name
        case "couch_party_6": L10n.achievementCouchParty6Name
        case "emoji_100": L10n.achievementEmoji100Name
        case "avatar_set": L10n.achievementAvatarSetName
        case "veteran_365": L10n.achievementVeteran365Name
        case "achievements_10": L10n.achievementAchievements10Name
        case "achievements_20": L10n.achievementAchievements20Name
        default: code.replacingOccurrences(of: "_", with: " ")
        }
    }

    static func achievementDescription(_ code: String) -> String? {
        switch code {
        case "first_play": L10n.achievementFirstPlayDesc
        case "watch_10h": L10n.achievementWatch10hDesc
        case "watch_50h": L10n.achievementWatch50hDesc
        case "watch_200h": L10n.achievementWatch200hDesc
        case "watch_500h": L10n.achievementWatch500hDesc
        case "movies_25": L10n.achievementMovies25Desc
        case "episodes_100": L10n.achievementEpisodes100Desc
        case "series_done_1": L10n.achievementSeriesDone1Desc
        case "series_done_10": L10n.achievementSeriesDone10Desc
        case "marathon_6h": L10n.achievementMarathon6hDesc
        case "streak_3": L10n.achievementStreak3Desc
        case "streak_7": L10n.achievementStreak7Desc
        case "streak_30": L10n.achievementStreak30Desc
        case "night_owl_10": L10n.achievementNightOwl10Desc
        case "early_bird_10": L10n.achievementEarlyBird10Desc
        case "active_days_50": L10n.achievementActiveDays50Desc
        case "active_days_200": L10n.achievementActiveDays200Desc
        case "genres_10": L10n.achievementGenres10Desc
        case "titles_50": L10n.achievementTitles50Desc
        case "watchlist_10": L10n.achievementWatchlist10Desc
        case "decades_4": L10n.achievementDecades4Desc
        case "couch_host_1": L10n.achievementCouchHost1Desc
        case "couch_host_25": L10n.achievementCouchHost25Desc
        case "couch_join_10": L10n.achievementCouchJoin10Desc
        case "couch_party_6": L10n.achievementCouchParty6Desc
        case "emoji_100": L10n.achievementEmoji100Desc
        case "avatar_set": L10n.achievementAvatarSetDesc
        case "veteran_365": L10n.achievementVeteran365Desc
        case "achievements_10": L10n.achievementAchievements10Desc
        case "achievements_20": L10n.achievementAchievements20Desc
        default: nil
        }
    }

    /// An SF Symbol per achievement, after the web's icons.
    static func achievementSymbol(_ code: String) -> String {
        switch code {
        case "first_play": "movieclapper.fill"
        case "watch_10h": "timer"
        case "watch_50h": "hourglass"
        case "watch_200h": "film.stack.fill"
        case "watch_500h", "achievements_20": "crown.fill"
        case "movies_25": "popcorn.fill"
        case "episodes_100": "list.and.film"
        case "series_done_1": "tv.fill"
        case "series_done_10": "rosette"
        case "marathon_6h": "figure.run"
        case "streak_3", "streak_7", "streak_30": "flame.fill"
        case "night_owl_10": "moon.stars.fill"
        case "early_bird_10": "sunrise.fill"
        case "active_days_50": "calendar"
        case "active_days_200": "calendar.badge.checkmark"
        case "genres_10": "safari.fill"
        case "titles_50": "checkmark.circle.fill"
        case "watchlist_10": "heart.fill"
        case "decades_4": "sparkles"
        case "couch_host_1", "couch_host_25": "sofa.fill"
        case "couch_join_10", "couch_party_6": "person.3.fill"
        case "emoji_100": "face.smiling.inverse"
        case "avatar_set": "person.crop.circle.fill"
        default: "trophy.fill"
        }
    }

    /// The categories in the order the profile shows them, the server's own.
    nonisolated static let categories = ["watching", "streaks", "explorer", "couch", "meta"]

    static func category(_ code: String) -> String {
        switch code {
        case "watching": L10n.achievementCategoryWatching
        case "streaks": L10n.achievementCategoryStreaks
        case "explorer": L10n.achievementCategoryExplorer
        case "couch": L10n.achievementCategoryCouch
        case "meta": L10n.achievementCategoryMeta
        default: code
        }
    }

    /// The name of an achievement's medal: bronze, silver, gold or platinum.
    static func medal(_ tier: String) -> String {
        switch tier {
        case "silver": L10n.achievementTierSilver
        case "gold": L10n.achievementTierGold
        case "platinum": L10n.achievementTierPlatinum
        default: L10n.achievementTierBronze
        }
    }

    static func tierName(_ code: String) -> String {
        switch code {
        case "rookie": L10n.rankTierRookie
        case "remote": L10n.rankTierRemote
        case "snack": L10n.rankTierSnack
        case "binger": L10n.rankTierBinger
        case "popcorn": L10n.rankTierPopcorn
        case "marathoner": L10n.rankTierMarathoner
        case "sage": L10n.rankTierSage
        case "cinephile": L10n.rankTierCinephile
        case "master": L10n.rankTierMaster
        case "legend": L10n.rankTierLegend
        default: code
        }
    }

    /// What earned XP; the keys are the API's (`couchHosted`), not the strings' (`couch_hosted`).
    static func xpSource(_ key: String) -> String {
        switch key {
        case "video": L10n.rankSourceVideo
        case "movies": L10n.rankSourceMovies
        case "episodes": L10n.rankSourceEpisodes
        case "couchHosted": L10n.rankSourceCouchHosted
        case "couchJoined": L10n.rankSourceCouchJoined
        case "achievements": L10n.rankSourceAchievements
        default: key
        }
    }

    static func metric(_ metric: Metric) -> String {
        switch metric {
        case .xp: L10n.leaderboardMetricXp
        case .watch: L10n.leaderboardMetricWatch
        case .achievements: L10n.leaderboardMetricAchievements
        }
    }

    static func period(_ period: Period) -> String {
        switch period {
        case .all: L10n.leaderboardPeriodAll
        case .month: L10n.leaderboardPeriodMonth
        case .week: L10n.leaderboardPeriodWeek
        }
    }

    /// `Remote Wrangler · Level 2`.
    static func rankLine(tier code: String, level: UInt32) -> String {
        "\(tierName(code)) \u{00B7} \(L10n.rankLevel(level: String(level)))"
    }

    static func rankLine(_ badge: RankBadge) -> String {
        rankLine(tier: badge.tier.code, level: badge.tier.level)
    }

    /// The XP into this tier out of what it takes, and what it leads to; the top of the ladder
    /// names itself as its next tier.
    static func progress(_ badge: RankBadge) -> (into: String, next: String) {
        guard badge.next.level != badge.tier.level else {
            return (L10n.rankXpValue(xp: number(badge.xp)), L10n.rankMaxLevel)
        }
        let into = badge.xp.saturatingSubtract(badge.tier.minXp)
        let need = badge.next.minXp.saturatingSubtract(badge.tier.minXp)
        return (
            L10n.rankXpProgress(into: number(into), need: number(need)),
            L10n.rankToNextLevel(level: String(badge.next.level))
        )
    }

    /// A board's value as it reads: XP, watch time or a count of achievements.
    static func value(_ value: UInt64, metric: Metric) -> String {
        switch metric {
        case .xp: L10n.rankXpValue(xp: number(value))
        case .watch: watchTime(seconds: value)
        case .achievements: number(value)
        }
    }

    /// `12 hr, 5 min`, `45 min` or `0 min`; past a hundred hours the minutes are noise.
    static func watchTime(seconds: UInt64) -> String {
        let minutes = Int(seconds / 60)
        let units: Set<Duration.UnitsFormatStyle.Unit> = minutes >= 100 * 60 ? [.hours] : [.hours, .minutes]
        return Duration.seconds(minutes * 60).formatted(
            .units(allowed: units, width: .abbreviated, fractionalPart: .hide(rounded: .down)).locale(L10n.locale))
    }

    /// `1,234` with the display language's grouping.
    static func number(_ value: UInt64) -> String {
        value.formatted(.number.locale(L10n.locale))
    }

    /// `3 Oct 2025` for the server's timestamp or date, in the display language; nil when it does
    /// not parse.
    static func date(_ value: String) -> String? {
        let day = String(value.prefix(10))
        guard let date = try? Date(day, strategy: .iso8601.year().month().day()) else { return nil }
        var style = Date.FormatStyle(date: .abbreviated, time: .omitted).locale(L10n.locale)
        style.timeZone = .gmt
        return date.formatted(style)
    }

    /// A failed save in words: the problem's own when the app has words for its code, else what
    /// failed, as the web's `problemMessage(problem, fallback)` does.
    static func failure(_ problem: Problem?, fallback: String) -> String {
        guard let message = problem?.message, message != L10n.problemGeneric else { return fallback }
        return message
    }
}

extension UInt64 {
    fileprivate func saturatingSubtract(_ other: UInt64) -> UInt64 {
        self > other ? self - other : 0
    }
}
