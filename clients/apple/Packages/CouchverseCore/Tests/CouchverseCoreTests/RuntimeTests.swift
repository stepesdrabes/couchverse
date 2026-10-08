import Foundation
import Testing

@testable import CouchverseCore

/// The runtime over the real core, with fake executors and a clock the test controls.
@MainActor
struct RuntimeTests {
    let http = FakeHTTP()
    let timers = ManualTimers()
    let secure = MemoryStore()
    let store = MemoryStore()

    func makeRuntime(
        platform: Platform = .ios, locale: String = "cs-CZ", store: (any KeyValueStore)? = nil,
        now: UInt64 = 1_000
    ) throws -> CoreRuntime {
        let config = CoreConfig(
            platform: platform, authMode: .bearer, deviceName: "Living Room", locale: locale,
            origin: "")
        let executors = Executors(
            http: http, timers: timers, sockets: SocketExecutor(), player: SilentPlayer(), secureStore: secure,
            store: store ?? self.store)
        return try CoreRuntime(config: config, executors: executors, now: { now })
    }

    @Test func theRuntimeStartsFromTheCoresOwnDefaults() throws {
        let runtime = try makeRuntime()
        #expect(runtime.app.phase == .starting)
        #expect(runtime.session.language == "cs")
        #expect(runtime.session.accent.accent == "#e50914")
    }

    @Test func aFirstLaunchReadsTheStoresAndWelcomes() async throws {
        let runtime = try makeRuntime()
        runtime.start()
        await runtime.untilIdle()
        #expect(runtime.app.phase == .welcome)
        #expect(http.requests.isEmpty)
    }

    @Test func unreadableStoresStillLetTheAppStart() async throws {
        let runtime = try makeRuntime(store: FailingStore())
        runtime.start()
        await runtime.untilIdle()
        #expect(runtime.app.phase == .welcome)
    }

    @Test func addingAServerFallsBackToHttpAndPersistsIt() async throws {
        let runtime = try makeRuntime()
        runtime.start()
        await runtime.untilIdle()
        http.route("GET", "http://tv.home/api/v1/server", Payload.server)

        runtime.send(.serverAddressSubmitted(ServerAddress(address: "tv.home")))
        await runtime.untilIdle()

        #expect(http.requests.map(\.url) == ["https://tv.home/api/v1/server", "http://tv.home/api/v1/server"])
        let server = try #require(runtime.servers.servers.first)
        #expect(server.insecure)
        #expect(server.name == "Home Media")
        #expect(runtime.servers.add.added == Payload.serverId)
        #expect(runtime.app.phase == .signIn)
        #expect(store.snapshot["servers"]?.contains("http://tv.home") == true)
    }

    @Test func passwordSignInKeepsTheTokenInTheSecureStore() async throws {
        let store = MemoryStore(["servers": Payload.storedServer])
        let runtime = try makeRuntime(platform: .ipados, locale: "en-GB", store: store)
        runtime.start()
        await runtime.untilIdle()
        #expect(runtime.app.phase == .signIn)

        http.route("POST", "http://tv.home/api/v1/auth/token", Payload.token("tok-n", 2, "nora"))
        Payload.routeSession(http, base: "http://tv.home", user: Payload.user(2, "nora"))
        runtime.send(
            .passwordSignInSubmitted(
                PasswordSignIn(serverId: Payload.serverId, username: "nora", password: "hunter22")))
        await runtime.untilIdle()

        let account = "\(Payload.serverId)/2"
        #expect(runtime.app.phase == .ready)
        #expect(runtime.app.activeAccount == account)
        #expect(secure.snapshot["token.\(account)"] == "tok-n")
        #expect(store.snapshot["accounts"]?.contains("tok-n") == false)
        #expect(runtime.session.user?.username == "nora")
        #expect(runtime.session.language == "cs")
        #expect(runtime.session.accent.accent == "#3a6ea5")
        let me = try #require(http.requests("GET", "http://tv.home/api/v1/auth/me").first)
        #expect(me.headers.contains(HttpHeader(name: "Authorization", value: "Bearer tok-n")))
    }

    @Test func pairingPollsOnTimerTicksUntilTheCodeIsApproved() async throws {
        let store = MemoryStore(["servers": Payload.storedServer])
        let runtime = try makeRuntime(platform: .tvos, store: store, now: 42_000)
        runtime.start()
        await runtime.untilIdle()

        http.route("POST", "http://tv.home/api/v1/auth/pairings", status: 201, Payload.pairing)
        runtime.send(.pairingStarted(ServerRef(serverId: Payload.serverId)))
        await runtime.untilIdle()

        let pairing = try #require(runtime.signIn.pairing)
        #expect(pairing.userCode == "WDJB-MJHT")
        #expect(pairing.verifyUrl == "http://tv.home/pair?code=WDJB-MJHT")
        #expect(pairing.expiresAtMs == 42_000 + 600_000)
        let poll = try #require(timers.timer(afterMs: 5_000, repeats: true))
        let expiry = try #require(timers.timer(afterMs: 600_000, repeats: false))

        http.route("POST", "http://tv.home/api/v1/auth/pairings/poll", #"{"status":"pending"}"#)
        timers.fire(poll.id)
        await runtime.untilIdle()
        #expect(runtime.signIn.pairing?.state == .waiting)

        http.route(
            "POST", "http://tv.home/api/v1/auth/pairings/poll",
            #"{"status":"approved","device":\#(Payload.token("tok-tv", 1, "admin"))}"#)
        Payload.routeSession(http, base: "http://tv.home", user: Payload.user(1, "admin"))
        timers.fire(poll.id)
        await runtime.untilIdle()

        #expect(runtime.app.phase == .ready)
        #expect(timers.cancelled.contains(poll.id))
        #expect(timers.cancelled.contains(expiry.id))
        #expect(timers.running.isEmpty)
        #expect(runtime.signIn.pairing == nil)
    }

    @Test func titlePagesArePublishedBySlugOnceTheirScreenOpens() async throws {
        let runtime = try makeRuntime(store: MemoryStore(Payload.signedIn))
        try secure.write("token.\(Payload.accountId)", value: "tok-1")
        Payload.routeSession(http, base: "http://tv.home", user: Payload.user(1, "admin"))
        runtime.start()
        await runtime.untilIdle()
        #expect(runtime.title("glass-harbor").status == .loading, "not open yet")

        http.route(
            "GET", "http://tv.home/api/v1/titles/glass-harbor?lang=cs",
            #"{"title":{"id":"t1","slug":"glass-harbor","name":"Glass Harbor","kind":"movie","year":2025,"overview":"Lighthouses.","genres":[],"genreLabels":[],"contentRating":"","runtimeMinutes":null,"allowRandomPlayback":false,"addedAt":"2026-09-01T00:00:00Z","metadataLanguages":["en"],"releaseDate":null,"sortName":"Glass Harbor","status":"published","tmdbId":null,"updatedAt":"2026-09-01T00:00:00Z"},"artwork":[],"seasons":[],"mediaFiles":[],"episodeProgress":{},"inWatchlist":true}"#
        )
        runtime.send(.screenOpened(.title("glass-harbor")))
        await runtime.untilIdle()
        let page = runtime.title("glass-harbor")
        #expect(page.status == .loaded, "\(page.problem?.detail ?? "")")
        #expect(page.detail?.name == "Glass Harbor")
        #expect(page.detail?.inList == true)
    }

    @Test func thePlayerTakesTheCoresCommandsAndReportsBack() async throws {
        let player = SilentPlayer()
        let config = CoreConfig(
            platform: .tvos, authMode: .bearer, deviceName: "Living Room", locale: "en-GB", origin: "")
        let runtime = try CoreRuntime(
            config: config,
            executors: Executors(
                http: http, timers: timers, sockets: SocketExecutor(), player: player, secureStore: secure,
                store: MemoryStore(Payload.signedIn)),
            now: { 1_000 })
        try secure.write("token.\(Payload.accountId)", value: "tok-1")
        Payload.routeSession(http, base: "http://tv.home", user: Payload.user(1, "admin"))
        runtime.start()
        await runtime.untilIdle()
        runtime.send(.accountSelected(AccountRef(accountId: Payload.accountId)))
        runtime.send(.capabilitiesReported(.avPlayer(DeviceProfileTests.appleTV4K)))
        await runtime.untilIdle()
        http.route("POST", "http://tv.home/api/v1/playback/movie/t1?lang=cs", Payload.directMovie)

        runtime.send(.playRequested(PlayTarget(kind: .movie, id: "t1")))
        await runtime.untilIdle()

        guard case .load(let load) = try #require(player.commands.last) else {
            Issue.record("expected a load, got \(player.commands)")
            return
        }
        #expect(load.url == "http://tv.home/api/v1/media/gr/stream")
        #expect(load.source == .file)
        #expect(load.startSeconds == 600)
        #expect(load.subtitles.map(\.id) == ["s-en"])
        #expect(runtime.player.status == .loaded)
        let body = try #require(http.requests("POST", "http://tv.home/api/v1/playback/movie/t1?lang=cs").first?.body)
        #expect(body.contains("\"dolbyVision8\""), "the device profile goes with the request")

        player.events?(
            .playerReported(
                PlayerReport(positionSeconds: 600, durationSeconds: 2400, playing: true, buffering: false, ended: false)
            ))
        await runtime.untilIdle()
        #expect(runtime.player.status == .loaded)
    }

    @Test func aLateBatchNeverOverwritesANewerView() {
        let runtime = CoreRuntime(fixture: .ios, [])
        runtime.publish([.app(AppView(phase: .ready, activeAccount: "a"))], generation: 2)
        runtime.publish(
            [.app(AppView(phase: .welcome, activeAccount: nil)), .servers(.empty)], generation: 1)
        #expect(runtime.app.phase == .ready)
    }

    @Test func fixtureRuntimesRecordWhatTheyAreSent() {
        let runtime = CoreRuntime(fixture: .tvos, [.app(AppView(phase: .chooseAccount, activeAccount: nil))])
        runtime.send(.accountSelected(AccountRef(accountId: "a")))
        #expect(runtime.app.phase == .chooseAccount)
        #expect(runtime.sentEvents == [.accountSelected(AccountRef(accountId: "a"))])
    }

    @Test func markdownIsParsedByTheCore() throws {
        let runtime = try makeRuntime()
        let doc = runtime.markdown("Hi **there**")
        guard case .paragraph(let inlines) = try #require(doc.blocks.first) else {
            Issue.record("expected a paragraph, got \(doc.blocks)")
            return
        }
        #expect(inlines == [.text("Hi "), .strong([.text("there")])])
        #expect(runtime.markdown("Hi **there**") == doc)
    }
}
