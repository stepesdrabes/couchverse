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
            http: http, timers: timers, secureStore: secure, store: store ?? self.store)
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
