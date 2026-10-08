import CouchverseShared
import Foundation
import Testing

@testable import CouchverseCore

/// The snapshot the app leaves for its extensions and intents, and the Spotlight entries made from
/// it.
struct ShelfSnapshotTests {
    static let words = ShelfSnapshot.Words(
        continueWatching: "Pokračovat ve sledování", myList: "Můj seznam",
        nothingToContinue: "Pusťte si něco v Couchverse a počká to tady.")

    static let episode = ContinueCard(
        titleId: "t1", slug: "couch-tales", name: "Couch Tales", kind: .series, episodeLabel: "S1 E3",
        positionSeconds: 1260, durationSeconds: 2400, progress: 0.52, play: PlayTarget(kind: .episode, id: "e3"),
        poster: Image(url: "https://tv.home/api/v1/artwork/p1?size=w342&g=g1"),
        backdrop: Image(url: "https://tv.home/api/v1/artwork/b1?size=w780&g=g1"))
    static let movie = ContinueCard(
        titleId: "t4", slug: "northern-lights", name: "Northern Lights", kind: .movie, positionSeconds: 3000,
        durationSeconds: 6720, progress: 0.45, play: PlayTarget(kind: .movie, id: "t4"))
    static let listed = [
        Card(titleId: "t1", slug: "couch-tales", name: "Couch Tales", kind: .series, year: 2024),
        Card(
            titleId: "t9", slug: "zluta-ponorka", name: "Žlutá ponorka", kind: .movie, year: 1968,
            poster: Image(url: "https://tv.home/api/v1/artwork/p9?size=w342&g=g1")),
    ]

    static func home(_ status: LoadStatus, _ cards: [ContinueCard] = [episode, movie]) -> HomeView {
        let row = HomeRowView(id: "continue", kind: .continueWatching, label: "", cards: [], continueWatching: cards)
        return HomeView(status: status, featured: [], rows: [row])
    }

    static func myList(_ status: LoadStatus, _ cards: [Card] = listed) -> MyListView {
        MyListView(status: status, cards: cards)
    }

    static let ready = AppView(phase: .ready, activeAccount: "a1")

    func update(
        _ snapshot: ShelfSnapshot = ShelfSnapshot(account: nil, language: "en", words: words, accent: "#e50914"),
        app: AppView = ready, home: HomeView = home(.loaded), myList: MyListView = myList(.loaded)
    ) -> ShelfSnapshot {
        snapshot.updated(app: app, home: home, myList: myList, language: "cs", words: Self.words, accent: "#3a6ea5")
    }

    @Test func linksHaveTheShapesEveryClientUses() {
        #expect(AppLinks.title(slug: "glass-harbor-2025") == "couchverse://title/glass-harbor-2025")
        #expect(AppLinks.play(kind: "episode", id: "e2") == "couchverse://play/episode/e2")
        #expect(PlayTarget(kind: .movie, id: "m 1/2").link == "couchverse://play/movie/m%201%2F2")
    }

    @Test func aLoadedHomeAndListFillIt() throws {
        let snapshot = update()
        #expect(snapshot.account == "a1")
        #expect(snapshot.language == "cs")
        #expect(snapshot.accent == "#3a6ea5")
        #expect(snapshot.words == Self.words)
        let first = try #require(snapshot.continueWatching.first)
        #expect(first.name == "Couch Tales")
        #expect(first.label == "S1 E3")
        #expect(first.progress == 0.52)
        #expect(first.playLink == "couchverse://play/episode/e3")
        #expect(first.titleLink == "couchverse://title/couch-tales")
        #expect(first.backdrop == "https://tv.home/api/v1/artwork/b1?size=w780&g=g1")
        #expect(snapshot.continueWatching.last?.playLink == "couchverse://play/movie/t4")
        #expect(snapshot.myList.map(\.slug) == ["couch-tales", "zluta-ponorka"])
        #expect(snapshot.myList.last?.year == 1968)
    }

    @Test(arguments: [LoadStatus.idle, .loading, .stale, .failed])
    func viewsNotLoadedKeepWhatWasThere(status: LoadStatus) {
        let before = update()
        let after = update(before, home: Self.home(status, []), myList: Self.myList(status, []))
        #expect(after.continueWatching == before.continueWatching)
        #expect(after.myList == before.myList)
    }

    @Test func eachPartFollowsItsOwnView() {
        let before = update()
        let after = update(before, home: Self.home(.loaded, [Self.movie]), myList: Self.myList(.loading, []))
        #expect(after.continueWatching.map(\.playId) == ["t4"])
        #expect(after.myList == before.myList)
    }

    @Test(arguments: [AppPhase.welcome, .signIn])
    func noAccountLeavesItEmpty(phase: AppPhase) {
        let after = update(update(), app: AppView(phase: phase, activeAccount: nil))
        #expect(after.account == nil)
        #expect(after.continueWatching.isEmpty)
        #expect(after.myList.isEmpty)
        #expect(after.spotlightEntries.isEmpty)
    }

    @Test func anotherAccountStartsOver() {
        let after = update(
            update(), app: AppView(phase: .ready, activeAccount: "a2"), home: Self.home(.loading),
            myList: Self.myList(.idle))
        #expect(after.account == "a2")
        #expect(after.continueWatching.isEmpty)
        #expect(after.myList.isEmpty)
    }

    @Test(arguments: [AppPhase.starting, .chooseAccount])
    func choosingAProfileKeepsIt(phase: AppPhase) {
        let before = update()
        #expect(update(before, app: AppView(phase: phase, activeAccount: nil), home: .idle, myList: .idle) == before)
    }

    @Test func titlesAreEachNamedOnceAndFoundIgnoringAccents() {
        let snapshot = update()
        #expect(snapshot.titles.map(\.slug) == ["couch-tales", "northern-lights", "zluta-ponorka"])
        #expect(snapshot.titles(matching: "zluta").map(\.slug) == ["zluta-ponorka"])
        #expect(snapshot.titles(matching: "  LIGHTS ").map(\.slug) == ["northern-lights"])
        #expect(snapshot.titles(matching: "").count == 3)
        #expect(snapshot.titles(matching: "kestrel").isEmpty)
    }

    @Test func itRoundTripsThroughItsFile() throws {
        let directory = FileManager.default.temporaryDirectory.appending(path: "shelf-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: directory) }
        #expect(ShelfSnapshot.read(from: directory) == nil)
        let snapshot = update()
        try snapshot.write(to: directory)
        #expect(ShelfSnapshot.read(from: directory) == snapshot)
    }

    /// A build without `EXTENSIONS_ENABLED` names no App Group, and the snapshot stays in the app's
    /// own caches.
    @Test func withoutAGroupTheSnapshotStaysInTheAppsCaches() {
        #expect(SharedContainer.group(in: .main) == nil)
        #expect(!SharedContainer.extensionsEnabled(in: .main))
        let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first
        #expect(SharedContainer.directory(bundle: .main) == caches)
    }

    /// The format the extensions decode, which may be older or newer than the app writing it.
    @Test func theFileKeepsItsShape() throws {
        let json = """
            {"account":"a1","accent":"#e50914","continueWatching":[{"backdrop":null,"kind":"episode",\
            "label":"S1 E3","name":"Couch Tales","playId":"e3","poster":null,"progress":0.5,"slug":"couch-tales"}],\
            "language":"en","myList":[{"name":"Neon Drift","slug":"neon-drift","year":2023}],\
            "words":{"continueWatching":"Continue Watching","myList":"My List","nothingToContinue":"Nothing yet"}}
            """
        let snapshot = try JSONDecoder().decode(ShelfSnapshot.self, from: Data(json.utf8))
        #expect(snapshot.continueWatching.first?.playLink == "couchverse://play/episode/e3")
        #expect(snapshot.myList.first?.poster == nil)
        let encoded = try #require(String(data: try JSONEncoder().encode(snapshot), encoding: .utf8))
        for key in ["\"continueWatching\"", "\"playId\"", "\"nothingToContinue\"", "\"accent\""] {
            #expect(encoded.contains(key))
        }
    }

    @Test func accentsBecomeComponents() throws {
        let color = try #require(HexColor("#3a6ea5"))
        #expect(color.red == 0x3a / 255.0)
        #expect(color.green == 0x6e / 255.0)
        #expect(color.blue == 0xa5 / 255.0)
        #expect(HexColor("3a6ea5") == color)
        #expect(HexColor("#3a6ea") == nil)
        #expect(HexColor("#zz6ea5") == nil)
    }
}

struct SpotlightEntryTests {
    @Test func continueWatchingComesFirstAndEachTitleOnce() {
        let snapshot = ShelfSnapshotTests().update()
        let entries = snapshot.spotlightEntries
        #expect(
            entries.map(\.link) == [
                "couchverse://title/couch-tales", "couchverse://title/northern-lights",
                "couchverse://title/zluta-ponorka",
            ])
        #expect(entries[0].detail == "Pokračovat ve sledování · S1 E3")
        #expect(entries[1].detail == "Pokračovat ve sledování")
        #expect(entries[2].detail == "Můj seznam")
        #expect(entries[0].poster == "https://tv.home/api/v1/artwork/p1?size=w342&g=g1")
    }

    @Test func anItemCarriesItsLinkTheAccountAndAThumbnail() {
        let entry = SpotlightEntry(
            link: "couchverse://title/neon-drift", name: "Neon Drift", detail: "My List", poster: nil)
        let item = entry.item(domain: "a1", thumbnail: Data([0xFF, 0xD8]))
        #expect(item.uniqueIdentifier == "couchverse://title/neon-drift")
        #expect(item.domainIdentifier == "a1")
        #expect(item.attributeSet.title == "Neon Drift")
        #expect(item.attributeSet.contentDescription == "My List")
        #expect(item.attributeSet.thumbnailData == Data([0xFF, 0xD8]))
    }
}
