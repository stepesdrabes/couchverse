import CouchverseCore
import Foundation
import Testing

@testable import CouchverseFeatures

struct OpenRequestTests {
    @Test func titlesListsAndPlayingNeedNoHome() {
        let home = Fixtures.home(.loading)
        #expect(OpenRequest.title(slug: "couch-tales").step(home: home) == .showTitle(slug: "couch-tales"))
        #expect(OpenRequest.myList.step(home: home) == .showMyList)
        let target = PlayTarget(kind: .movie, id: "t4")
        #expect(OpenRequest.play(target).step(home: home) == .play(target))
    }

    @Test func aCouchCodeIsTheRootsToTake() {
        #expect(OpenRequest.couch(code: "123456").step(home: Fixtures.home(.loaded)) == nil)
    }

    @Test func continueWatchingPlaysTheFirstCardOnceTheHomeSettles() {
        let first = OpenStep.play(PlayTarget(kind: .episode, id: "e3"))
        #expect(OpenRequest.continueWatching.step(home: Fixtures.home(.idle)) == .wait)
        #expect(OpenRequest.continueWatching.step(home: Fixtures.home(.loading)) == .wait)
        #expect(OpenRequest.continueWatching.step(home: Fixtures.home(.loaded)) == first)
        // stale with a problem will not refresh, so it is as good as it gets
        #expect(OpenRequest.continueWatching.step(home: Fixtures.home(.stale)) == first)
    }

    @Test func aRefreshingHomeIsWaitedFor() {
        let refreshing = HomeView(status: .stale, featured: [], rows: Fixtures.home(.loaded).rows)
        #expect(OpenRequest.continueWatching.step(home: refreshing) == .wait)
    }

    @Test func nothingToContinueShowsHome() {
        let empty = HomeView(status: .loaded, featured: [], rows: [])
        #expect(OpenRequest.continueWatching.step(home: empty) == .showHome)
        #expect(OpenRequest.continueWatching.step(home: Fixtures.home(.failed)) == .showHome)
    }

    @MainActor
    @Test func aNewerRequestOutlivesTheOneActedOn() {
        let requests = OpenRequests()
        requests.open(.myList)
        requests.open(.title(slug: "neon-drift"))
        requests.finish(.myList)
        #expect(requests.pending == .title(slug: "neon-drift"))
        requests.finish(.title(slug: "neon-drift"))
        #expect(requests.pending == nil)
    }
}
