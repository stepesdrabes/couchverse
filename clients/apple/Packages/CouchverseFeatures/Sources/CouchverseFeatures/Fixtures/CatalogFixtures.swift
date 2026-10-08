import CouchverseCore
import Foundation

/// Catalog and player view models. Artwork points at a host that never resolves, so screens show
/// each image's accent-tinted placeholder, the same on every run; logos are left out for the
/// same reason (a logo that fails shows the name, one still loading shows nothing).
extension Fixtures {
    static let titles: [(name: String, kind: TitleKind, year: Int32, accent: String)] = [
        ("Glass Harbor", .movie, 2025, "#866635"),
        ("Couch Tales", .series, 2024, "#6b8a2e"),
        ("Neon Drift", .movie, 2023, "#a5463a"),
        ("Quiet Hours", .movie, 2019, "#2e8a7a"),
        ("Northern Lights", .movie, 2022, "#3a6ea5"),
        ("Static Bloom", .movie, 2024, "#7aa53a"),
        ("Deep Current", .movie, 2021, "#2e5f8a"),
        ("Prism Profile 8", .movie, 2020, "#3aa55f"),
        ("Test Pattern", .movie, 2026, "#5a3aa5"),
        ("Night Shift", .series, 2023, "#a53a7a"),
        ("Paper Moons", .movie, 2018, "#a58a3a"),
        ("Long Way Round", .series, 2021, "#3a8aa5"),
    ]

    static func slug(_ name: String) -> String {
        name.lowercased().replacingOccurrences(of: " ", with: "-")
    }

    static func artwork(_ role: String, _ index: Int, accent: String) -> CouchverseCore.Image {
        CouchverseCore.Image(url: "https://media.invalid/api/v1/artwork/\(role)\(index)?g=g", accent: accent)
    }

    public static func cards(_ count: Int, from start: Int = 0) -> [Card] {
        (0..<count).map { offset in
            let index = (start + offset) % titles.count
            let title = titles[index]
            return Card(
                titleId: "t\(index)", slug: slug(title.name), name: title.name, kind: title.kind, year: title.year,
                poster: artwork("p", index, accent: title.accent), backdrop: artwork("b", index, accent: title.accent))
        }
    }

    static func featured(_ index: Int, inList: Bool = false) -> FeaturedCard {
        let title = titles[index]
        return FeaturedCard(
            titleId: "t\(index)", slug: slug(title.name), name: title.name, kind: title.kind, year: title.year,
            overview: "A lighthouse keeper finds a letter that was never sent, and follows it across a winter coast.",
            genres: ["Drama", "Mystery"], contentRating: "PG-13", runtimeMinutes: title.kind == .movie ? 112 : nil,
            backdrop: artwork("b", index, accent: title.accent), inList: inList)
    }

    public static func home(_ status: LoadStatus) -> HomeView {
        let shown = status == .loaded || status == .stale
        let continueWatching = [
            ContinueCard(
                titleId: "t1", slug: "couch-tales", name: "Couch Tales", kind: .series, episodeLabel: "S1 E3",
                positionSeconds: 1260, durationSeconds: 2400, progress: 0.52,
                play: PlayTarget(kind: .episode, id: "e3"), backdrop: artwork("b", 1, accent: titles[1].accent)),
            ContinueCard(
                titleId: "t4", slug: "northern-lights", name: "Northern Lights", kind: .movie,
                positionSeconds: 3000, durationSeconds: 6720, progress: 0.45,
                play: PlayTarget(kind: .movie, id: "t4"), backdrop: artwork("b", 4, accent: titles[4].accent)),
        ]
        let rows = [
            HomeRowView(
                id: "continue", kind: .continueWatching, label: "Continue Watching", cards: [],
                continueWatching: continueWatching),
            HomeRowView(
                id: "recent", kind: .recentlyAdded, label: "Up on the Marquee", cards: cards(8), continueWatching: []),
            HomeRowView(
                id: "genre-drama", kind: .genre, label: "Drama", cards: cards(6, from: 3), continueWatching: []),
        ]
        return HomeView(
            status: status, featured: shown ? [featured(0), featured(1, inList: true), featured(2)] : [],
            rows: shown ? rows : [], problem: problem(status))
    }

    static func problem(_ status: LoadStatus) -> Problem? {
        status == .failed || status == .stale ? Problem(code: "offline", detail: "") : nil
    }

    public static let titleAccent = AccentPalette(
        accent: "#866635", strong: "#684f29", soft: "#86663529", onAccent: "#ffffff", ink: "#9b8057")

    public static func title(_ status: LoadStatus, series: Bool = false) -> TitleView {
        let index = series ? 1 : 0
        let source = titles[index]
        let slug = slug(source.name)
        guard status == .loaded || status == .stale else {
            return TitleView(slug: slug, status: status, problem: status == .failed ? problem(status) : nil)
        }
        let episodes = (1...4).map { number in
            EpisodeView(
                id: "e\(number)", number: UInt32(number),
                name: ["Pilot", "The Long Night In", "Remote Control", "Season's End"][number - 1],
                overview: "The friends rearrange the living room for a marathon nobody agreed to.",
                runtimeMinutes: 40, still: artwork("s", number, accent: source.accent), durationSeconds: 2400,
                progress: number == 1 ? 1 : number == 2 ? 0.4 : 0, completed: number == 1)
        }
        let detail = TitleDetailView(
            id: "t\(index)", slug: slug, name: source.name, kind: source.kind, year: source.year,
            overview: "A lighthouse keeper finds a letter that was never sent, and follows it across a winter coast.",
            genres: ["Drama", "Mystery"], contentRating: "PG-13", runtimeMinutes: series ? nil : 112,
            poster: artwork("p", index, accent: source.accent), backdrop: artwork("b", index, accent: source.accent),
            accent: titleAccent, quality: .hd1080, hdr: !series, inList: series,
            play: series
                ? PlayAction(
                    target: PlayTarget(kind: .episode, id: "e2"), resumeSeconds: 960,
                    episode: EpisodeNumber(season: 1, episode: 2))
                : PlayAction(target: PlayTarget(kind: .movie, id: "t0"), resumeSeconds: 2520),
            shuffle: series,
            seasons: series
                ? [
                    SeasonView(id: "s1", number: 1, name: "", overview: "", episodes: episodes),
                    SeasonView(id: "s2", number: 2, name: "", overview: "", episodes: Array(episodes.prefix(2))),
                ] : [])
        return TitleView(slug: slug, status: status, detail: detail, problem: problem(status))
    }

    public static func browse(_ status: LoadStatus, key: BrowseKey, empty: Bool = false) -> BrowseView {
        let shown = (status == .loaded || status == .stale) && !empty
        return BrowseView(
            key: key, genreLabel: key.genre, status: status,
            cards: shown ? cards(12) : [], total: shown ? 30 : 0, more: shown, loadingMore: false,
            problem: problem(status))
    }

    public static func genres(_ status: LoadStatus) -> GenresView {
        let shown = status == .loaded || status == .stale
        let genres = ["Action", "Comedy", "Documentary", "Drama", "Mystery", "Science Fiction", "Thriller"]
        return GenresView(
            status: status, genres: shown ? genres.map { GenreView(name: $0, label: $0) } : [],
            problem: problem(status))
    }

    public static func myList(_ status: LoadStatus, empty: Bool = false) -> MyListView {
        let shown = (status == .loaded || status == .stale) && !empty
        return MyListView(status: status, cards: shown ? cards(5, from: 2) : [], problem: problem(status))
    }

    public static func search(_ query: String, _ status: LoadStatus, empty: Bool = false) -> SearchView {
        let shown = status == .loaded && !empty && !query.isEmpty
        return SearchView(query: query, status: status, cards: shown ? cards(4, from: 4) : [], problem: problem(status))
    }

    public static func player(_ state: String) -> PlayerView {
        let qualities = [
            QualityOption(key: "original", kind: .original), QualityOption(key: "auto", kind: .auto),
            QualityOption(key: "1080p", kind: .rendition, height: 1080),
            QualityOption(key: "720p", kind: .rendition, height: 720),
        ]
        let status: LoadStatus =
            switch state {
            case "preparing", "loading": .loading
            case "failed", "unsupported": .failed
            default: .loaded
            }
        let problem: Problem? =
            switch state {
            case "failed": Problem(code: "playback_failed", detail: "")
            case "unsupported": Problem(code: "unsupported", detail: "")
            default: nil
            }
        return PlayerView(
            status: status, target: PlayTarget(kind: .episode, id: "e2"), title: "Couch Tales",
            subtitle: "S1 E2 \u{00B7} The Long Night In", titleSlug: "couch-tales",
            backdrop: artwork("b", 1, accent: titles[1].accent), preparing: state == "preparing" ? 42 : nil,
            qualities: status == .loaded ? qualities : [], quality: "auto",
            audio: [], subtitles: [TrackOption(id: "s-en", lang: "en", label: "English")],
            seasons: [
                PlayerSeason(
                    number: 1,
                    episodes: (1...4).map {
                        PlayerEpisode(id: "e\($0)", number: UInt32($0), name: "Episode \($0)", current: $0 == 2)
                    })
            ],
            nextUp: state == "next"
                ? NextUp(
                    target: PlayTarget(kind: .episode, id: "e3"), season: 1, episode: 3, name: "Remote Control",
                    countdownSeconds: 12, shuffled: false)
                : nil,
            shuffleAvailable: true, shuffle: false, linear: false, problem: problem)
    }
}
