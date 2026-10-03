import Foundation
import Observation
import os

let coreLog = Logger(subsystem: "io.stepes.couchverse", category: "core")

/// The app's one stateful service (plan 10.3): owns the core, stamps every message with the
/// monotonic clock, runs the effect loop with one executor per effect kind and publishes the view
/// models the core renders. Screens read these surfaces and send events; they never keep a copy of
/// core state.
@MainActor
@Observable
public final class CoreRuntime {
    public private(set) var app: AppView
    public private(set) var servers: ServersView
    public private(set) var accounts: AccountsView
    public private(set) var signIn: SignInView
    public private(set) var devices: DevicesView
    public private(set) var pairingApproval: PairingApprovalView
    public private(set) var session: SessionView

    public let platform: Platform

    /// What a fixture runtime was sent, for tests and previews; a live runtime forwards instead.
    @ObservationIgnored public private(set) var sentEvents: [Event] = []

    @ObservationIgnored private let live: Live?
    @ObservationIgnored private var renderGeneration: UInt64 = 0
    @ObservationIgnored private var publishedGeneration: [Surface: UInt64] = [:]
    @ObservationIgnored private var markdownCache: [String: MarkdownDoc] = [:]
    @ObservationIgnored private var inFlight = 0
    @ObservationIgnored private var idleWaiters: [CheckedContinuation<Void, Never>] = []

    private struct Live {
        let bridge: CoreBridge
        let executors: Executors
        let now: @MainActor () -> UInt64
        let secureQueue: StoreQueue
        let storeQueue: StoreQueue
    }

    public init(
        config: CoreConfig,
        executors: Executors,
        now: @escaping @MainActor () -> UInt64 = MonotonicClock().nowMs
    ) throws {
        let bridge = try CoreBridge(config: try Self.encode(config))
        live = Live(
            bridge: bridge,
            executors: executors,
            now: now,
            secureQueue: StoreQueue(executors.secureStore, label: "secure-store"),
            storeQueue: StoreQueue(executors.store, label: "store")
        )
        platform = config.platform
        // the core's own defaults (language from the locale, the default accent) before any render
        let initial = try SurfaceValue.published.compactMap { surface in
            try SurfaceValue.decode(surface, try bridge.view(surface: try Self.encode(surface)))
        }
        app = AppView(phase: .starting, activeAccount: nil)
        servers = .empty
        accounts = .empty
        signIn = .idle
        devices = .idle
        pairingApproval = .idle
        session = .signedOut(language: "en")
        initial.forEach(assign)
    }

    /// A runtime without a core, showing fixed view models: previews and snapshot tests.
    public init(fixture platform: Platform, _ values: [SurfaceValue]) {
        live = nil
        self.platform = platform
        app = AppView(phase: .ready, activeAccount: nil)
        servers = .empty
        accounts = .empty
        signIn = .idle
        devices = .idle
        pairingApproval = .idle
        session = .signedOut(language: "en")
        values.forEach(assign)
    }

    /// Lets the core load what it persisted; call once at launch.
    public func start() {
        send(.appStarted)
    }

    public func send(_ event: Event) {
        guard let live else {
            sentEvents.append(event)
            return
        }
        perform {
            try live.bridge.send(message: try Self.encode(Message(nowMs: live.now(), event: event)))
        }
    }

    /// The shell's monotonic clock in the core's terms, e.g. for a pairing code's countdown.
    public func nowMs() -> UInt64 {
        live?.now() ?? 0
    }

    /// A bio or other user markdown as the core's safe document tree.
    public func markdown(_ source: String) -> MarkdownDoc {
        if let cached = markdownCache[source] {
            return cached
        }
        guard let live else { return MarkdownDoc(blocks: []) }
        do {
            let json = try live.bridge.view(surface: try Self.encode(Surface.markdown(source)))
            let doc = try JSONDecoder().decode(MarkdownDoc.self, from: Data(json.utf8))
            if markdownCache.count >= 64 {
                markdownCache.removeAll()
            }
            markdownCache[source] = doc
            return doc
        } catch {
            coreLog.fault("markdown view failed: \(error)")
            return MarkdownDoc(blocks: [])
        }
    }

    /// Replaces a fixture's view model; a live runtime only publishes what the core renders.
    public func showFixture(_ value: SurfaceValue) {
        guard live == nil else { return }
        assign(value)
    }

    /// Waits until every effect and render issued so far has finished (timers excepted).
    func untilIdle() async {
        guard inFlight > 0 else { return }
        await withCheckedContinuation { idleWaiters.append($0) }
    }

    private func perform(_ call: () throws -> String) {
        let requests: [EffectRequest]
        do {
            requests = try JSONDecoder().decode([EffectRequest].self, from: Data(try call().utf8))
        } catch {
            // a malformed message is a shell bug; the core itself never fails a well-formed one
            coreLog.fault("core call failed: \(error)")
            assertionFailure("core call failed: \(error)")
            return
        }
        requests.forEach(execute)
    }

    private func resolve(_ id: UInt64, _ output: EffectOutput) {
        guard let live else { return }
        perform {
            let resolution = Resolution(nowMs: live.now(), id: id, output: output)
            return try live.bridge.resolve(resolution: try Self.encode(resolution))
        }
    }

    private func execute(_ request: EffectRequest) {
        guard let live else { return }
        let id = request.id
        switch request.effect {
        case .http(let http):
            track {
                let output = await live.executors.http.perform(http)
                self.resolve(id, output)
            }
        case .timer(let timer):
            live.executors.timers.start(id: id, request: timer) { [weak self] in
                self?.resolve(id, .timerFired)
            }
        case .cancelTimer(let timer):
            live.executors.timers.cancel(id: timer.id)
        case .secureStore(let store):
            track { self.resolve(id, await live.secureQueue.run(store)) }
        case .store(let store):
            track { self.resolve(id, await live.storeQueue.run(store)) }
        case .render(let render):
            self.render(render.surfaces)
        case .upload:
            // nothing can hand the core a picked file until profile editing lands (Phase 8)
            track {
                self.resolve(id, .httpFailed(HttpFailure(kind: .other, message: "uploads are not supported yet")))
            }
        }
    }

    /// Re-reads only the named surfaces. Batches decode concurrently, so each surface keeps the
    /// generation it was last published at and an older batch finishing late never wins.
    private func render(_ surfaces: [Surface]) {
        guard let live else { return }
        renderGeneration += 1
        let generation = renderGeneration
        let payloads = surfaces.compactMap { surface -> RenderedSurface? in
            guard SurfaceValue.published.contains(surface) else { return nil }
            do {
                let json = try live.bridge.view(surface: try Self.encode(surface))
                return RenderedSurface(surface: surface, json: json)
            } catch {
                coreLog.fault("view \(String(describing: surface)) failed: \(error)")
                return nil
            }
        }
        track {
            let values = await SurfaceValue.decodeBatch(payloads)
            self.publish(values, generation: generation)
        }
    }

    func publish(_ values: [SurfaceValue], generation: UInt64) {
        for value in values {
            let surface = value.surface
            if let published = publishedGeneration[surface], published >= generation {
                continue
            }
            publishedGeneration[surface] = generation
            assign(value)
        }
    }

    /// Assigns only what changed: every assignment invalidates the views that read the surface.
    private func assign(_ value: SurfaceValue) {
        switch value {
        case .app(let view): if app != view { app = view }
        case .servers(let view): if servers != view { servers = view }
        case .accounts(let view): if accounts != view { accounts = view }
        case .signIn(let view): if signIn != view { signIn = view }
        case .devices(let view): if devices != view { devices = view }
        case .pairingApproval(let view): if pairingApproval != view { pairingApproval = view }
        case .session(let view): if session != view { session = view }
        }
    }

    private func track(_ work: @escaping @MainActor () async -> Void) {
        inFlight += 1
        Task {
            await work()
            inFlight -= 1
            if inFlight == 0 {
                let waiters = idleWaiters
                idleWaiters = []
                waiters.forEach { $0.resume() }
            }
        }
    }

    private static func encode<T: Encodable>(_ value: T) throws -> String {
        String(decoding: try JSONEncoder().encode(value), as: UTF8.self)
    }
}

/// Runs one store's effects in the order the core issued them, off the main actor.
private final class StoreQueue: Sendable {
    private let store: any KeyValueStore
    private let queue: DispatchQueue

    init(_ store: any KeyValueStore, label: String) {
        self.store = store
        queue = DispatchQueue(label: "io.stepes.couchverse.\(label)", qos: .userInitiated)
    }

    func run(_ request: StoreRequest) async -> EffectOutput {
        await withCheckedContinuation { continuation in
            queue.async { [store] in
                continuation.resume(returning: store.execute(request))
            }
        }
    }
}
