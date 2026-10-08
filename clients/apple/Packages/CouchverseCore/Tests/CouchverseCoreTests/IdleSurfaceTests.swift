import Foundation
import Testing

@testable import CouchverseCore

/// Title pages, listings, profiles and boards stay published while a screen holds them open and
/// for a while after, then the runtime lets them go.
extension RuntimeTests {
    func routeTitle(_ slug: String, _ name: String) {
        http.route(
            "GET", "http://tv.home/api/v1/titles/\(slug)?lang=cs",
            #"{"title":{"id":"\#(slug)","slug":"\#(slug)","name":"\#(name)","kind":"movie","year":2025,"overview":"","genres":[],"genreLabels":[],"contentRating":"","runtimeMinutes":null,"allowRandomPlayback":false,"addedAt":"2026-09-01T00:00:00Z","metadataLanguages":["en"],"releaseDate":null,"sortName":"\#(name)","status":"published","tmdbId":null,"updatedAt":"2026-09-01T00:00:00Z"},"artwork":[],"seasons":[],"mediaFiles":[],"episodeProgress":{},"inWatchlist":false}"#
        )
    }

    @Test func aPageNoScreenHoldsIsLetGoOnceItIdledAWhile() async throws {
        let clock = MillisecondClock()
        let runtime = try await signedInRuntime(clock: clock)
        routeTitle("glass-harbor", "Glass Harbor")
        routeTitle("salt-road", "Salt Road")

        runtime.send(.screenOpened(.title("glass-harbor")))
        await runtime.untilIdle()
        runtime.send(.screenClosed(.title("glass-harbor")))
        clock.nowMs += CoreRuntime.keptIdleMs - 1
        runtime.send(.screenOpened(.title("salt-road")))
        await runtime.untilIdle()
        #expect(runtime.title("glass-harbor").detail?.name == "Glass Harbor", "back within a while, it shows at once")

        clock.nowMs += 1
        runtime.send(.screenClosed(.title("salt-road")))

        #expect(runtime.titles["glass-harbor"] == nil)
        #expect(runtime.title("glass-harbor").status == .loading)
        #expect(runtime.title("salt-road").detail?.name == "Salt Road", "only just let go of")
    }

    @Test func aPageAnotherScreenStillHoldsIsKept() async throws {
        let clock = MillisecondClock()
        let runtime = try await signedInRuntime(clock: clock)
        routeTitle("glass-harbor", "Glass Harbor")
        routeTitle("salt-road", "Salt Road")

        // the same title on two tabs' stacks
        runtime.send(.screenOpened(.title("glass-harbor")))
        runtime.send(.screenOpened(.title("glass-harbor")))
        await runtime.untilIdle()
        runtime.send(.screenClosed(.title("glass-harbor")))
        clock.nowMs += CoreRuntime.keptIdleMs * 3
        runtime.send(.screenOpened(.title("salt-road")))
        await runtime.untilIdle()

        #expect(runtime.title("glass-harbor").detail?.name == "Glass Harbor")
    }

    @Test func reopeningAPageStartsItsWhileAgain() async throws {
        let clock = MillisecondClock()
        let runtime = try await signedInRuntime(clock: clock)
        routeTitle("glass-harbor", "Glass Harbor")
        routeTitle("salt-road", "Salt Road")

        runtime.send(.screenOpened(.title("glass-harbor")))
        await runtime.untilIdle()
        runtime.send(.screenClosed(.title("glass-harbor")))
        clock.nowMs += CoreRuntime.keptIdleMs - 1
        runtime.send(.screenOpened(.title("glass-harbor")))
        await runtime.untilIdle()
        runtime.send(.screenClosed(.title("glass-harbor")))
        clock.nowMs += CoreRuntime.keptIdleMs - 1
        runtime.send(.screenOpened(.title("salt-road")))
        await runtime.untilIdle()
        #expect(runtime.title("glass-harbor").detail != nil)

        clock.nowMs += 1
        runtime.send(.screenClosed(.title("salt-road")))
        #expect(runtime.titles["glass-harbor"] == nil)
    }

    @Test func boardsTheCoreRendersWithoutAScreenAgeToo() async throws {
        let clock = MillisecondClock()
        let runtime = try await signedInRuntime(clock: clock)
        http.route("GET", "http://tv.home/api/v1/leaderboard?period=week", Payload.board)
        routeTitle("salt-road", "Salt Road")
        let xp = LeaderboardKey(period: .week, metric: .xp)
        let watch = LeaderboardKey(period: .week, metric: .watch)

        runtime.send(.screenOpened(.leaderboard(xp)))
        await runtime.untilIdle()
        #expect(runtime.leaderboards[watch] != nil, "published with the board's answer")

        clock.nowMs += CoreRuntime.keptIdleMs
        runtime.send(.screenOpened(.title("salt-road")))

        #expect(runtime.leaderboards[watch] == nil)
        #expect(runtime.leaderboard(xp).rows.map(\.username) == ["nora", "admin"], "still on screen")
    }
}
