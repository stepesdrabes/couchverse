import CouchverseCore
import CouchverseDesign
import Foundation
import Testing

@testable import CouchverseFeatures

struct DeepLinkTests {
    @Test(arguments: [
        ("couchverse://connect?server=https%3A%2F%2Fmedia.example.com&code=K7PQ", DeepLink.connect),
        ("couchverse://pair?code=WDJB-MJHT", .approve),
        ("COUCHVERSE://Pair?code=WDJB-MJHT", .approve),
        ("http://192.168.1.5:8080/pair?code=WDJB-MJHT", .approve),
        ("https://media.example.com/pair/?code=WDJB-MJHT", .approve),
        ("couchverse://title/glass-harbor-2025", .title(slug: "glass-harbor-2025")),
        ("COUCHVERSE://Title/glass-harbor/", .title(slug: "glass-harbor")),
        ("couchverse://play/episode/e2", .play(PlayTarget(kind: .episode, id: "e2"))),
        ("couchverse://play/Movie/0b9e6c1a-77d4-4c3e", .play(PlayTarget(kind: .movie, id: "0b9e6c1a-77d4-4c3e"))),
    ])
    func linksOpenTheirScreen(url: String, link: DeepLink) {
        #expect(DeepLink(url) == link)
    }

    @Test(arguments: [
        "couchverse://title",
        "couchverse://title/",
        "couchverse://title/glass-harbor/extras",
        "couchverse://play/song/s1",
        "couchverse://play/movie",
        "couchverse://play/episode/e2/again",
        "couchverse://admin",
        "https://media.example.com/title/glass-harbor",
        "https://media.example.com/pair",
        "https://media.example.com/connect?server=x&code=y",
        "https://media.example.com/watch?code=WDJB-MJHT",
        "ftp://media.example.com/pair?code=WDJB-MJHT",
        "not a url",
    ])
    func otherLinksOpenNothing(url: String) {
        #expect(DeepLink(url) == nil)
    }
}

struct UserCodeInputTests {
    @Test(arguments: [
        ("wdjb", "WDJB"),
        ("wdjbm", "WDJB-M"),
        ("wdjbmjht", "WDJB-MJHT"),
        ("WDJB-MJHT", "WDJB-MJHT"),
        (" wd jb-mj ht 42 ", "WDJB-MJHT"),
        ("WDJBMJHTXX", "WDJB-MJHT"),
        ("ččč", ""),
    ])
    func typedCodesAreGrouped(typed: String, formatted: String) {
        #expect(UserCodeInput.format(typed) == formatted)
    }

    @Test func onlyAFullCodeCanBeSubmitted() {
        #expect(UserCodeInput.isComplete("wdjbmjht"))
        #expect(!UserCodeInput.isComplete("WDJB-MJH"))
    }
}

struct PairingCountdownTests {
    @Test(arguments: [
        (UInt64(600_000), UInt64(0), "10:00"),
        (65_500, 0, "1:05"),
        (1_000_000, 999_400, "0:00"),
        (5_000, 9_000, "0:00"),
    ])
    func theCountdownReadsMinutesAndSeconds(deadline: UInt64, now: UInt64, text: String) {
        #expect(PairingPanel.countdown(deadline, now: now) == text)
    }

    @Test func theAddressIsShownWithoutItsScheme() {
        #expect(PairingPanel.displayed("http://192.168.1.5:8080/pair?code=X") == "192.168.1.5:8080/pair?code=X")
    }
}

@MainActor
struct ProfileChoreographyTests {
    @Test func theChosenAvatarFliesToTheHeaderAndLands() async {
        let choreography = ProfileChoreography()
        let card = Fixtures.account("nora", "Nora")
        let run = Task {
            await choreography.run(card: card, accounts: [card], from: CGRect(x: 100, y: 400, width: 240, height: 240))
        }
        await Task.yield()
        #expect(choreography.isRunning)
        #expect(choreography.isFlying(card.id))
        #expect(!choreography.isFlying("someone-else"))

        choreography.target = CGRect(x: 40, y: 40, width: 72, height: 72)
        // the main actor is shared with the snapshot suites, so wait for the stage, not a time
        for _ in 0..<100 where !choreography.flown {
            try? await Task.sleep(for: .milliseconds(20))
        }
        #expect(choreography.dissolved)
        #expect(choreography.flown)
        #expect(!choreography.landed)

        await run.value
        #expect(!choreography.isRunning)
        #expect(!choreography.isFlying(card.id))
        #expect(choreography.card == nil)
    }
}

@MainActor
struct CatalogLabelTests {
    init() { L10n.language = "en" }

    @Test func aMovieResumesAtATimeAndASeriesNamesItsEpisode() {
        let movie = PlayAction(target: PlayTarget(kind: .movie, id: "m"), resumeSeconds: 3725)
        #expect(CatalogLabels.playLabel(movie, kind: .movie) == "Resume from 1:02:05")
        let episode = PlayAction(
            target: PlayTarget(kind: .episode, id: "e"), resumeSeconds: 600,
            episode: EpisodeNumber(season: 2, episode: 4))
        #expect(CatalogLabels.playLabel(episode, kind: .series) == "Play S2 E4")
        #expect(CatalogLabels.playLabel(PlayAction(target: PlayTarget(kind: .movie, id: "m")), kind: .movie) == "Play")
    }

    @Test func theBuiltInRowsAreTranslatedAndCustomOnesKept() {
        let row = { (kind: HomeRowKind, label: String) in
            HomeRowView(id: "r", kind: kind, label: label, cards: [], continueWatching: [])
        }
        L10n.language = "cs"
        #expect(CatalogLabels.row(row(.continueWatching, "Continue Watching")) == L10n.homeRowContinueWatching)
        #expect(CatalogLabels.row(row(.recentlyAdded, "Fresh this week")) == "Fresh this week")
        #expect(CatalogLabels.row(row(.genre, "Drama")) == "Drama")
        L10n.language = "en"
    }

    @Test func factsJoinWhatIsKnown() {
        #expect(
            CatalogLabels.facts(year: 2024, rating: "PG-13", runtime: 112, kind: .movie)
                == "Movie · 2024 · PG-13 · 1 hr, 52 min")
        #expect(CatalogLabels.facts(year: nil, rating: nil, runtime: nil) == "")
    }
}
