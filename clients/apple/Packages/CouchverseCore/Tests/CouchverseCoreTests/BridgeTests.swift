import Foundation
import Testing

@testable import CouchverseCore

/// The bridge end to end on the host: the generated message types round-trip through the Rust
/// core exactly as the app's runtime will send and read them.
struct BridgeTests {
    private let encoder = JSONEncoder()
    private let decoder = JSONDecoder()

    private func makeCore(platform: Platform = .tvos) throws -> CoreBridge {
        let config = CoreConfig(
            platform: platform, authMode: .bearer, deviceName: "Living Room", locale: "cs-CZ",
            origin: "")
        return try CoreBridge(config: String(decoding: try encoder.encode(config), as: UTF8.self))
    }

    private func send(_ core: CoreBridge, _ event: Event, at now: UInt64) throws -> [EffectRequest] {
        let message = String(decoding: try encoder.encode(Message(nowMs: now, event: event)), as: UTF8.self)
        return try decoder.decode([EffectRequest].self, from: Data(try core.send(message: message).utf8))
    }

    private func resolve(_ core: CoreBridge, _ id: UInt64, _ output: EffectOutput, at now: UInt64)
        throws -> [EffectRequest]
    {
        let resolution = Resolution(nowMs: now, id: id, output: output)
        let json = String(decoding: try encoder.encode(resolution), as: UTF8.self)
        return try decoder.decode([EffectRequest].self, from: Data(try core.resolve(resolution: json).utf8))
    }

    private func view<T: Decodable>(_ core: CoreBridge, _ surface: Surface, as: T.Type) throws -> T {
        let json = String(decoding: try encoder.encode(surface), as: UTF8.self)
        return try decoder.decode(T.self, from: Data(try core.view(surface: json).utf8))
    }

    @Test func aFirstLaunchReadsTheStoresAndWelcomes() throws {
        let core = try makeCore()
        let effects = try send(core, .appStarted, at: 1)
        let reads = effects.compactMap { request -> String? in
            guard case .store(let store) = request.effect, store.op == .read else { return nil }
            return store.key
        }
        #expect(reads == ["servers", "accounts"])

        var last: [EffectRequest] = []
        for request in effects {
            last = try resolve(core, request.id, .stored(StoredValue(value: nil)), at: 2)
        }
        let render = try #require(last.last)
        guard case .render(let surfaces) = render.effect else {
            Issue.record("expected a render, got \(render.effect)")
            return
        }
        #expect(surfaces.surfaces.contains(.app))
        #expect(try view(core, .app, as: AppView.self).phase == .welcome)
    }

    @Test func addingAServerIssuesItsIdentityRequest() throws {
        let core = try makeCore()
        _ = try send(core, .appStarted, at: 1)
        let effects = try send(
            core, .serverAddressSubmitted(ServerAddress(address: "tv.home:8080")), at: 5)
        guard case .http(let request) = try #require(effects.first).effect else {
            Issue.record("expected an HTTP request")
            return
        }
        #expect(request.method == "GET")
        #expect(request.url == "https://tv.home:8080/api/v1/server")
        let servers = try view(core, .servers, as: ServersView.self)
        #expect(servers.add.status == .loading)
    }

    @Test func markdownRendersToASafeTree() throws {
        let core = try makeCore()
        let doc = try view(core, .markdown("Hi **there** <b>x</b>"), as: MarkdownDoc.self)
        guard case .paragraph(let inlines) = try #require(doc.blocks.first) else {
            Issue.record("expected a paragraph")
            return
        }
        #expect(inlines.count == 3)
    }

    @Test func malformedMessagesThrow() throws {
        let core = try makeCore()
        #expect(throws: CoreError.self) { try core.send(message: "{\"nowMs\":1}") }
    }
}
