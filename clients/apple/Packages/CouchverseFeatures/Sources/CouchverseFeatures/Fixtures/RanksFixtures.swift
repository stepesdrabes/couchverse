import CouchverseCore
import Foundation

/// Ranks, profiles, leaderboards and the profile editor for previews and snapshots. Avatars and
/// banners point at the same never-resolving host as the catalog's artwork, so every picture
/// shows its placeholder.
extension Fixtures {
    static func tier(_ code: String, _ level: UInt32, _ minXp: UInt64) -> Tier {
        Tier(code: code, level: level, colour: "#34d399", minXp: minXp)
    }

    /// Binge Apprentice (level 4), three quarters of the way to Popcorn Veteran.
    public static let rankBadge = RankBadge(
        tier: tier("binger", 4, 3500), next: tier("popcorn", 5, 7000), xp: 6150, percent: 75)

    public static func rank(celebrating code: String? = nil) -> RankView {
        RankView(
            rank: rankBadge, levelUps: 0, celebration: code.map { achievement($0, unlocked: true) },
            queued: code == nil ? 0 : 1)
    }

    /// One achievement as the server scores it; the targets and medals are the server's own.
    static func achievement(_ code: String, unlocked: Bool, value: UInt64? = nil) -> AchievementCard {
        let (category, tier, target): (String, String, UInt64) =
            switch code {
            case "first_play": ("watching", "bronze", 1)
            case "watch_10h": ("watching", "bronze", 600)
            case "watch_50h": ("watching", "silver", 3000)
            case "movies_25": ("watching", "silver", 25)
            case "streak_3": ("streaks", "bronze", 3)
            case "streak_7": ("streaks", "silver", 7)
            case "night_owl_10": ("streaks", "silver", 10)
            case "genres_10": ("explorer", "silver", 10)
            case "titles_50": ("explorer", "silver", 50)
            case "couch_host_1": ("couch", "bronze", 1)
            case "avatar_set": ("meta", "bronze", 1)
            case "achievements_10": ("meta", "silver", 10)
            case "veteran_365": ("meta", "gold", 365)
            default: ("watching", "gold", 1)
            }
        let reached = unlocked ? target : value ?? 0
        let xp: UInt64 = ["bronze": 50, "silver": 150, "gold": 400, "platinum": 1000][tier] ?? 50
        return AchievementCard(
            code: code, category: category, tier: tier, unlocked: unlocked,
            unlockedAt: unlocked ? "2026-09-14T20:31:00Z" : nil, value: reached, target: target,
            percent: UInt32(min(reached * 100 / max(target, 1), 100)), xp: xp)
    }

    static let achievements: [AchievementCard] = [
        achievement("first_play", unlocked: true),
        achievement("watch_10h", unlocked: true),
        achievement("watch_50h", unlocked: false, value: 1840),
        achievement("movies_25", unlocked: false, value: 9),
        achievement("streak_3", unlocked: true),
        achievement("streak_7", unlocked: false, value: 5),
        achievement("night_owl_10", unlocked: false, value: 2),
        achievement("genres_10", unlocked: true),
        achievement("titles_50", unlocked: false, value: 31),
        achievement("couch_host_1", unlocked: true),
        achievement("avatar_set", unlocked: true),
        achievement("achievements_10", unlocked: false, value: 6),
        achievement("veteran_365", unlocked: false, value: 274),
    ]

    /// A year of evenings ending on `Fixtures.now`: quiet weekdays, busier weekends, a gap in
    /// spring.
    static let heatmap: Heatmap = {
        let days = (0..<365).map { index -> HeatDay in
            let weekend = index % 7 == 1 || index % 7 == 2
            let quiet = (index * 37) % 11 < 4 || (180..<200).contains(index)
            let seconds: UInt64 = quiet ? 0 : UInt64(((index * 53) % 17 + (weekend ? 10 : 1)) * 420)
            let level: UInt8 =
                switch seconds {
                case 0: 0
                case ..<1800: 1
                case ..<3600: 2
                case ..<7200: 3
                default: 4
                }
            return HeatDay(seconds: seconds, level: level)
        }
        return Heatmap(
            from: "2025-10-03", days: days, totalSeconds: days.reduce(0) { $0 + $1.seconds },
            activeDays: UInt32(days.filter { $0.seconds > 0 }.count))
    }()

    static let hours: [UInt64] = [
        1800, 600, 0, 0, 0, 0, 300, 1200, 900, 600, 300, 600,
        1800, 1200, 900, 1500, 2400, 3600, 5400, 7200, 9000, 10_800, 7200, 3600,
    ]

    static let bio = MarkdownDoc(blocks: [
        .paragraph([
            .text("Mostly "), .strong([.text("sci-fi")]), .text(" and the occasional documentary. Ask me about "),
            .link(LinkInline(href: "https://example.com/couch-tales", children: [.text("Couch Tales")])),
            .text("."),
        ])
    ])

    public static func profileDetail(
        _ username: String = "nora", displayName: String = "Nora", isSelf: Bool = true, public: Bool = true
    ) -> ProfileDetail {
        let top = [(1, 43_200), (0, 14_400), (9, 10_800), (2, 7200)].map { index, seconds in
            TopTitle(
                slug: slug(titles[index].name), name: titles[index].name, kind: titles[index].kind,
                seconds: UInt64(seconds), poster: artwork("p", index, accent: titles[index].accent))
        }
        return ProfileDetail(
            username: username, displayName: displayName, bio: bio,
            avatar: CouchverseCore.Image(url: "https://media.invalid/api/v1/artwork/av?g=g"),
            banner: CouchverseCore.Image(url: "https://media.invalid/api/v1/artwork/bn?g=g", accent: "#2e5f8a"),
            memberSince: "2025-01-03T18:20:00Z", isSelf: isSelf, public: `public`, rank: rankBadge, xpTotal: 6150,
            xpSources: [
                XpLine(key: "video", units: 1840, rate: 2, xp: 3680),
                XpLine(key: "movies", units: 9, rate: 100, xp: 900),
                XpLine(key: "episodes", units: 46, rate: 20, xp: 920),
                XpLine(key: "couchHosted", units: 3, rate: 50, xp: 150),
                XpLine(key: "couchJoined", units: 2, rate: 25, xp: 50),
                XpLine(key: "achievements", units: 6, rate: 0, xp: 450),
            ],
            achievements: achievements, achievementsWon: UInt32(achievements.filter(\.unlocked).count),
            recentUnlocks: [achievements[0]],
            totals: ProfileTotals(
                watchSeconds: 110_400, moviesCompleted: 9, episodesCompleted: 46, seriesCompleted: 2,
                distinctTitles: 31, distinctGenres: 10, activeDays: 120, currentStreak: 3, longestStreak: 6,
                bestDayMinutes: 290, couchHosted: 3, couchJoined: 2, biggestCouch: 4, emojiSent: 37),
            topTitles: top, favouriteGenre: "Science Fiction", hours: hours, heatmap: heatmap)
    }

    /// A member who joined today: nothing watched, nothing earned.
    public static let newcomer = ProfileDetail(
        username: "oskar", displayName: "Oskar", bio: MarkdownDoc(blocks: []), memberSince: "2026-10-02T09:00:00Z",
        isSelf: false, public: true,
        rank: RankBadge(tier: tier("rookie", 1, 0), next: tier("remote", 2, 500), xp: 0, percent: 0),
        xpTotal: 0, xpSources: [],
        achievements: achievements.map { achievement($0.code, unlocked: false) }, achievementsWon: 0,
        recentUnlocks: [],
        totals: ProfileTotals(
            watchSeconds: 0, moviesCompleted: 0, episodesCompleted: 0, seriesCompleted: 0, distinctTitles: 0,
            distinctGenres: 0, activeDays: 0, currentStreak: 0, longestStreak: 0, bestDayMinutes: 0, couchHosted: 0,
            couchJoined: 0, biggestCouch: 0, emojiSent: 0),
        topTitles: [], favouriteGenre: "", hours: [UInt64](repeating: 0, count: 24),
        heatmap: Heatmap(
            from: "2025-10-03", days: [HeatDay](repeating: HeatDay(seconds: 0, level: 0), count: 365),
            totalSeconds: 0, activeDays: 0))

    public static func profile(_ status: LoadStatus, detail: ProfileDetail = profileDetail()) -> ProfileView {
        let shown = status == .loaded || status == .stale
        return ProfileView(
            username: detail.username, status: status, profile: shown ? detail : nil,
            problem: status == .failed || status == .stale ? Problem(code: "offline", detail: "") : nil)
    }

    static func leaderRow(
        _ position: UInt32, _ username: String, _ name: String, xp: UInt64, watch: UInt64, achievements: UInt64,
        level: UInt32, tier: String, metric: Metric, isSelf: Bool = false
    ) -> LeaderRow {
        let value =
            switch metric {
            case .xp: xp
            case .watch: watch
            case .achievements: achievements
            }
        return LeaderRow(
            position: position, username: username, displayName: name, level: level, tierCode: tier, value: value,
            xp: xp, watchSeconds: watch, achievements: achievements, isSelf: isSelf)
    }

    /// A board of eight in which the viewer (Nora) is fifth, or hidden from it.
    public static func leaderboard(
        _ status: LoadStatus, metric: Metric = .xp, period: Period = .all, hidden: Bool = false, empty: Bool = false,
        allZero: Bool = false
    ) -> LeaderboardView {
        let key = LeaderboardKey(period: period, metric: metric)
        let shown = status == .loaded || status == .stale
        let members: [(String, String, UInt64, UInt64, UInt64, UInt32, String)] = [
            ("stepan", "Štěpán", 41_200, 640_000, 24, 8, "cinephile"),
            ("vera", "Vera", 23_900, 410_000, 19, 7, "sage"),
            ("otto", "Otto", 13_400, 260_000, 15, 6, "marathoner"),
            ("kids", "Kids Corner", 7300, 190_000, 9, 5, "popcorn"),
            ("nora", "Nora", 6150, 110_400, 6, 4, "binger"),
            ("ida", "Ida", 2100, 61_000, 4, 3, "snack"),
            ("max", "Max", 620, 9_000, 2, 2, "remote"),
            ("oskar", "Oskar", 0, 0, 0, 1, "rookie"),
        ]
        let listed = members.filter { !hidden || $0.0 != "nora" }
        let rows = listed.enumerated().map { index, member in
            leaderRow(
                UInt32(index + 1), member.0, member.1, xp: allZero ? 0 : member.2, watch: allZero ? 0 : member.3,
                achievements: allZero ? 0 : member.4, level: member.5, tier: member.6, metric: metric,
                isSelf: member.0 == "nora")
        }
        let me = leaderRow(
            hidden ? 0 : 5, "nora", "Nora", xp: 6150, watch: 110_400, achievements: 6, level: 4, tier: "binger",
            metric: metric, isSelf: true)
        let board = shown && !empty ? rows : []
        return LeaderboardView(
            key: key, status: status, rows: board, myPosition: hidden || board.isEmpty ? nil : 5,
            me: shown && !empty ? me : nil, total: UInt32(board.count), podium: !board.isEmpty && !allZero,
            allZero: !board.isEmpty && allZero, hidden: hidden && shown,
            problem: status == .failed || status == .stale ? Problem(code: "offline", detail: "") : nil)
    }

    public static func profileEditor(
        details: LoadStatus = .idle, password: LoadStatus = .idle, avatar: LoadStatus = .idle,
        passwordProblem: String? = nil
    ) -> ProfileEditorView {
        ProfileEditorView(
            details: SaveState(status: details),
            password: SaveState(status: password, problem: passwordProblem.map { Problem(code: $0, detail: "") }),
            avatar: SaveState(status: avatar), banner: SaveState(status: .idle))
    }
}
