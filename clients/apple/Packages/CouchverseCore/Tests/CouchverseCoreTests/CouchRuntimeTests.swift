import Foundation
import Testing

@testable import CouchverseCore

/// The couch through the runtime: the core's socket effect, the published `couch` surface and a
/// follower's player, against the real core.
@MainActor
struct CouchRuntimeTests {
    let http = FakeHTTP()
    let sockets = FakeSockets()
    let player = SilentPlayer()
    let secure = MemoryStore()

    /// A phone signed in as the admin on `tv.home`, in Czech.
    func signedIn() async throws -> CoreRuntime {
        let config = CoreConfig(
            platform: .ios, authMode: .bearer, deviceName: "Nora's iPhone", locale: "en-GB", origin: "")
        let executors = Executors(
            http: http, timers: ManualTimers(), sockets: sockets, player: player, secureStore: secure,
            store: MemoryStore(Payload.signedIn))
        let runtime = try CoreRuntime(config: config, executors: executors, now: { 1_000 })
        try secure.write("token.\(Payload.accountId)", value: "tok-1")
        Payload.routeSession(http, base: "http://tv.home", user: Payload.user(1, "admin"))
        runtime.start()
        await runtime.untilIdle()
        return runtime
    }

    @Test func aFollowerPlaysTheHostsTitleOnALinearPlayer() async throws {
        let runtime = try await signedIn()
        #expect(runtime.couch.status == .idle)
        http.route("POST", "http://tv.home/api/v1/couch/123456/join?delivery=body", Couch.session("follower"))
        http.route("GET", "http://tv.home/api/v1/couch/123456/playback?lang=cs", Couch.playback)

        runtime.send(.couchJoinRequested(CouchCode(code: "123456")))
        await runtime.untilIdle()

        #expect(runtime.couch.role == .follower)
        #expect(runtime.couch.status == .connecting)
        let socket = try #require(sockets.opened.first)
        #expect(socket.request.url == "ws://tv.home/api/v1/couch/123456/ws")
        #expect(socket.request.headers.contains(HttpHeader(name: "X-Couch-Token", value: "tok-follower")))
        let load = player.commands.compactMap { command -> PlayerLoad? in
            if case .load(let load) = command { load } else { nil }
        }.first
        #expect(load?.linear == true)
        #expect(runtime.player.linear)
        #expect(runtime.player.target == PlayTarget(kind: .movie, id: "t1"))

        sockets.deliver(socket.id, .socketOpened)
        await runtime.untilIdle()
        #expect(runtime.couch.status == .open)

        sockets.deliver(
            socket.id,
            .socketText(SocketText(text: #"{"type":"emoji","data":{"fromParticipantId":"p-host","emoji":"🍿"}}"#)))
        await runtime.untilIdle()
        #expect(runtime.couch.reactions.map(\.emoji) == ["🍿"])
    }

    @Test func aHostSharesTheJoinPageOfItsServerAndEndsTheSession() async throws {
        let runtime = try await signedIn()
        http.route("GET", "http://tv.home/api/v1/playback/movie/t1?lang=cs", Payload.directMovie)
        runtime.send(.playRequested(PlayTarget(kind: .movie, id: "t1")))
        await runtime.untilIdle()
        http.route("POST", "http://tv.home/api/v1/couch?delivery=body", status: 201, Couch.session("host"))

        runtime.send(.couchStartRequested)
        await runtime.untilIdle()
        let socket = try #require(sockets.opened.first)
        sockets.deliver(socket.id, .socketOpened)
        await runtime.untilIdle()

        #expect(runtime.couch.role == .host)
        #expect(runtime.couch.status == .open)
        #expect(runtime.couch.code == "123456")
        #expect(runtime.couch.shareUrl == "http://tv.home/couch/123456")
        #expect(runtime.couch.members.map(\.displayName) == ["Admin"])

        runtime.send(.couchEndRequested)
        await runtime.untilIdle()
        let end = try #require(http.requests("POST", "http://tv.home/api/v1/couch/123456/end").first)
        #expect(end.headers.contains(HttpHeader(name: "X-Couch-Token", value: "tok-host")))
        #expect(sockets.closed == [socket.id])
        #expect(runtime.couch.status == .ended)
        #expect(runtime.couch.ended == "host_ended")
    }
}

/// Sockets a test opens and feeds by hand.
@MainActor
final class FakeSockets: SocketExecuting {
    struct Opened {
        let id: UInt64
        let request: SocketOpen
    }

    private(set) var opened: [Opened] = []
    private(set) var sent: [(socket: UInt64, text: String)] = []
    private(set) var closed: [UInt64] = []
    private var deliveries: [UInt64: @MainActor (EffectOutput) -> Void] = [:]

    func open(id: UInt64, request: SocketOpen, deliver: @escaping @MainActor (EffectOutput) -> Void) {
        opened.append(Opened(id: id, request: request))
        deliveries[id] = deliver
    }

    func send(socket: UInt64, text: String) {
        sent.append((socket, text))
    }

    func close(socket: UInt64) {
        closed.append(socket)
        deliveries[socket] = nil
    }

    func deliver(_ socket: UInt64, _ output: EffectOutput) {
        deliveries[socket]?(output)
    }
}

/// Couch payloads in the shape the API returns them.
private enum Couch {
    static func session(_ role: String) -> String {
        """
        {"sessionId":"s1","shareToken":"123456","myParticipantId":"p-\(role)","role":"\(role)",\
        "isAnonymous":false,"participantToken":"tok-\(role)","artworkGrant":"g-couch",\
        "state":{"media":{"kind":"movie","titleId":"t1"},"playing":false,"positionSeconds":0,\
        "serverTimestamp":1000,"seq":1,"away":false},\
        "participants":[{"id":"p-host","displayName":"Admin","seed":"admin","isHost":true,\
        "isAnonymous":false,"paused":false}]}
        """
    }

    static let playback = #"{"media":{"kind":"movie","titleId":"t1"},"player":\#(Payload.directMovie)}"#
}
