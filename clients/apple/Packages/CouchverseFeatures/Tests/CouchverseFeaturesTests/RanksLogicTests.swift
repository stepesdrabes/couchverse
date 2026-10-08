import CouchverseCore
import CouchverseDesign
import Foundation
import SwiftUI
import Testing
import UIKit

@testable import CouchverseFeatures

@MainActor
@Suite(.serialized)
struct RanksWordsTests {
    // set in each test rather than in an initializer: a snapshot elsewhere switches the language
    // while it renders, and a test only keeps the main actor for as long as it does not suspend

    /// The server's catalogue (`ranks/achievements.go`); it only ever grows.
    nonisolated static let codes = [
        "first_play", "watch_10h", "watch_50h", "watch_200h", "watch_500h", "movies_25", "episodes_100",
        "series_done_1", "series_done_10", "marathon_6h", "streak_3", "streak_7", "streak_30", "night_owl_10",
        "early_bird_10", "active_days_50", "active_days_200", "genres_10", "titles_50", "watchlist_10", "decades_4",
        "couch_host_1", "couch_host_25", "couch_join_10", "couch_party_6", "emoji_100", "avatar_set", "veteran_365",
        "achievements_10", "achievements_20",
    ]

    @Test(arguments: codes)
    func everyAchievementHasWords(code: String) {
        #expect(RanksWords.achievementName(code) != code)
        #expect(RanksWords.achievementDescription(code) != nil)
        #expect(UIImage(systemName: RanksWords.achievementSymbol(code)) != nil)
    }

    @Test func aCodeFromANewerServerStillReads() {
        #expect(RanksWords.achievementName("binge_weekend") == "binge weekend")
        #expect(RanksWords.achievementDescription("binge_weekend") == nil)
        #expect(RanksWords.tierName("diamond") == "diamond")
        #expect(RanksWords.category("seasonal") == "seasonal")
    }

    @Test(arguments: [
        "rookie", "remote", "snack", "binger", "popcorn", "marathoner", "sage", "cinephile", "master", "legend",
    ])
    func everyTierHasAName(code: String) {
        #expect(RanksWords.tierName(code) != code)
    }

    @Test(arguments: ["video", "movies", "episodes", "couchHosted", "couchJoined", "achievements"])
    func theApisXpSourcesHaveWords(key: String) {
        #expect(RanksWords.xpSource(key) != key)
    }

    @Test(arguments: RanksWords.categories)
    func everyCategoryHasAName(code: String) {
        #expect(RanksWords.category(code) != code)
    }

    @Test func theRankReadsAsTierAndLevel() {
        L10n.language = "en"
        #expect(RanksWords.rankLine(Fixtures.rankBadge) == "Binge Apprentice \u{00B7} Level 4")
        let progress = RanksWords.progress(Fixtures.rankBadge)
        #expect(progress.into == "2,650 / 3,500 XP")
        #expect(progress.next == "to level 5")
    }

    @Test func theTopOfTheLadderHasNoNextLevel() {
        L10n.language = "en"
        let legend = Fixtures.tier("legend", 10, 120_000)
        let top = RankBadge(tier: legend, next: legend, xp: 131_000, percent: 100)
        let progress = RanksWords.progress(top)
        #expect(progress.into == "131,000 XP")
        #expect(progress.next == "Max level")
    }

    @Test func boardValuesReadAsTheirMetric() {
        L10n.language = "en"
        #expect(RanksWords.value(6150, metric: .xp) == "6,150 XP")
        #expect(RanksWords.value(110_400, metric: .watch) == "30 hr, 40 min")
        #expect(RanksWords.value(6, metric: .achievements) == "6")
    }

    @Test(
        arguments: [
            (0, "0 min"), (45 * 60, "45 min"), (125 * 60, "2 hr, 5 min"), (15_007 * 60, "250 hr"),
        ] as [(UInt64, String)])
    func watchTimeDropsTheMinutesOfLongTotals(seconds: UInt64, text: String) {
        L10n.language = "en"
        #expect(RanksWords.watchTime(seconds: seconds) == text)
    }

    @Test func datesReadInTheDisplayLanguage() {
        L10n.language = "en"
        #expect(RanksWords.date("2025-01-03T18:20:00Z") == "Jan 3, 2025")
        #expect(RanksWords.date("2025-01-03") == "Jan 3, 2025")
        #expect(RanksWords.date("someday") == nil)
    }

    @Test func aFailedSaveSaysWhyWhenTheAppKnowsTheCode() {
        L10n.language = "en"
        let wrong = Problem(code: "invalid_password", detail: "")
        #expect(
            RanksWords.failure(wrong, fallback: L10n.profilePasswordChangeFailed) == "The current password is wrong.")
        let offline = Problem(code: "offline", detail: "")
        #expect(RanksWords.failure(offline, fallback: "x") == L10n.problemOffline)
        let unknown = Problem(code: "avatar_failed", detail: "unsupported image type")
        #expect(RanksWords.failure(unknown, fallback: L10n.profileAvatarUploadFailed) == "Avatar upload failed")
        #expect(RanksWords.failure(nil, fallback: L10n.profileSaveFailed) == L10n.profileSaveFailed)
    }

    @Test func aNewPasswordIsJudgedOnceThereIsOne() {
        L10n.language = "en"
        #expect(ProfileEditorScreen.passwordProblem(new: "", confirm: "") == nil)
        #expect(
            ProfileEditorScreen.passwordProblem(new: "short", confirm: "") == "Password must be at least 8 characters")
        #expect(
            ProfileEditorScreen.passwordProblem(new: "longenough", confirm: "longenougj") == "Passwords do not match")
        #expect(ProfileEditorScreen.passwordProblem(new: "longenough", confirm: "longenough") == nil)
    }
}

struct HeatmapLayoutTests {
    static func day(_ string: String) -> Date {
        (try? Date(string, strategy: .iso8601.year().month().day())) ?? .distantPast
    }

    @Test func aRunStartsOnItsWeekdayWithMondayOnTop() {
        // 2026-09-30 is a Wednesday
        let heatmap = Heatmap(
            from: "2026-09-30",
            days: [HeatDay(seconds: 0, level: 0), HeatDay(seconds: 1800, level: 2), HeatDay(seconds: 3600, level: 4)],
            totalSeconds: 5400, activeDays: 2)
        let layout = HeatmapLayout(heatmap, weeks: 26)

        #expect(layout.columns.count == 1)
        let column = layout.columns[0]
        #expect(column.count == 7)
        #expect(column[0] == nil && column[1] == nil)
        #expect(column[2]?.date == Self.day("2026-09-30"))
        #expect(column[3]?.level == 2)
        #expect(column[4]?.seconds == 3600)
        #expect(column[5] == nil && column[6] == nil)
        #expect(layout.months == [HeatmapLayout.Month(column: 0, date: Self.day("2026-09-30"))])
    }

    @Test func aYearShowsItsLastWeeksEndingToday() {
        let layout = HeatmapLayout(Fixtures.heatmap, weeks: 26)

        #expect(layout.columns.count == 26)
        #expect(layout.columns.allSatisfy { $0.count == 7 })
        // the first column shown starts on a Monday, the last ends with today, a Friday
        #expect(layout.columns[0][0]?.date == Self.day("2026-04-06"))
        let last = layout.columns[25]
        #expect(last[4]?.date == Self.day("2026-10-02"))
        #expect(last[5] == nil && last[6] == nil)
        #expect(layout.months.map(\.column) == [0, 4, 8, 13, 17, 22])
        #expect(layout.months.last?.date == Self.day("2026-09-07"))
    }

    @Test func theFirstMonthGivesWayToANewOneRightAfterIt() {
        let days = [HeatDay](repeating: HeatDay(seconds: 60, level: 1), count: 14)
        // 2026-09-28 is a Monday: a column of September, then October's
        let layout = HeatmapLayout(
            Heatmap(from: "2026-09-28", days: days, totalSeconds: 840, activeDays: 14), weeks: 26)
        #expect(layout.columns.count == 2)
        #expect(layout.months == [HeatmapLayout.Month(column: 1, date: Self.day("2026-10-05"))])
    }

    @Test func anUnreadableStartShowsNothing() {
        let layout = HeatmapLayout(Heatmap(from: "soon", days: [], totalSeconds: 0, activeDays: 0), weeks: 26)
        #expect(layout.columns.isEmpty)
        #expect(layout.months.isEmpty)
    }
}

struct WatchClockLayoutTests {
    @Test func theDayRunsClockwiseFromMidnightAtTheTop() {
        var hours = [UInt64](repeating: 0, count: 24)
        hours[21] = 3600
        hours[7] = 1800
        let wedges = WatchClockLayout.wedges(hours)

        #expect(wedges.count == 24)
        #expect(wedges[0].start == -89 && wedges[0].end == -76)
        #expect(wedges[23].start == 256 && wedges[23].end == 269)
        #expect(wedges[21].fraction == 1)
        #expect(wedges[7].fraction == 0.5)
        #expect(wedges[0].fraction == 0)
        #expect(WatchClockLayout.busiest(hours) == 21)
    }

    @Test func nothingWatchedHasNoBusiestHour() {
        let hours = [UInt64](repeating: 0, count: 24)
        #expect(WatchClockLayout.wedges(hours).allSatisfy { $0.fraction == 0 })
        #expect(WatchClockLayout.busiest(hours) == nil)
        #expect(WatchClockLayout.busiest([0, 5, 5]) == 1)
    }

    @Test func labelsSitInTheMiddleOfTheirHour() {
        let center = CGPoint(x: 100, y: 100)
        let midnight = WatchClockLayout.labelPoint(hour: 0, center: center, radius: 50)
        #expect(midnight.y < 51 && midnight.x > 100)
        let six = WatchClockLayout.labelPoint(hour: 6, center: center, radius: 50)
        #expect(six.x > 149 && six.y > 100)
    }
}

struct AchievementGroupTests {
    static func card(_ code: String, _ category: String, unlocked: Bool) -> AchievementCard {
        AchievementCard(
            code: code, category: category, tier: "bronze", unlocked: unlocked, value: 0, target: 1, percent: 0, xp: 50)
    }

    @Test func categoriesKeepTheServersOrderWithTheUnlockedFirst() {
        let groups = AchievementsSection.groups(Fixtures.achievements)
        #expect(groups.map(\.category) == ["watching", "streaks", "explorer", "couch", "meta"])
        #expect(groups[0].cards.map(\.code) == ["first_play", "watch_10h", "watch_50h", "movies_25"])
        #expect(groups[4].cards.map(\.code) == ["avatar_set", "achievements_10", "veteran_365"])
    }

    @Test func aCategoryWithNothingInItIsAbsentAndANewOneComesLast() {
        let cards = [
            Self.card("a", "watching", unlocked: false), Self.card("x", "seasonal", unlocked: true),
            Self.card("b", "watching", unlocked: true), Self.card("c", "meta", unlocked: false),
        ]
        let groups = AchievementsSection.groups(cards)
        #expect(groups.map(\.category) == ["watching", "meta", "seasonal"])
        #expect(groups[0].cards.map(\.code) == ["b", "a"])
    }
}

struct CelebrationTests {
    @Test func reduceMotionKeepsACelebrationUpLonger() {
        #expect(CelebrationOverlay.duration(reduceMotion: true) > CelebrationOverlay.duration(reduceMotion: false))
    }
}
