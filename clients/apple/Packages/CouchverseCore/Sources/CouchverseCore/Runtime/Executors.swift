import Foundation

/// Performs the core's `Http` effects. Never throws: every failure becomes an output the core
/// understands.
public protocol HTTPExecuting: Sendable {
    func perform(_ request: HttpRequest) async -> EffectOutput
}

/// Runs the core's `Timer` effects on the main actor; `fire` resolves the effect.
@MainActor
public protocol TimerScheduling: AnyObject {
    func start(id: UInt64, request: TimerRequest, fire: @escaping @MainActor () -> Void)
    func cancel(id: UInt64)
}

/// Runs the core's `Socket` effects on the main actor; `deliver` resolves the open effect, once
/// per output, ending with `socketClosed`.
@MainActor
public protocol SocketExecuting: AnyObject {
    func open(id: UInt64, request: SocketOpen, deliver: @escaping @MainActor (EffectOutput) -> Void)
    func send(socket: UInt64, text: String)
    func close(socket: UInt64)
}

/// Runs the core's `Player` commands. A player answers with events rather than outputs (its
/// reports, a track the user picked in a system menu), sent through `events`, which the runtime
/// sets.
@MainActor
public protocol PlayerExecuting: AnyObject {
    var events: ((Event) -> Void)? { get set }
    func execute(_ command: PlayerCommand)
}

/// A player that plays nothing: tests, and hosts without video.
@MainActor
public final class SilentPlayer: PlayerExecuting {
    public var events: ((Event) -> Void)?
    public private(set) var commands: [PlayerCommand] = []

    public init() {}

    public func execute(_ command: PlayerCommand) {
        commands.append(command)
    }
}

/// Backs the core's `Store` and `SecureStore` effects. Calls arrive on one serial queue per
/// store, in the order the core issued them.
public protocol KeyValueStore: Sendable {
    func read(_ key: String) throws -> String?
    func write(_ key: String, value: String) throws
    func delete(_ key: String) throws
}

/// One executor per effect kind (plan 7.1).
public struct Executors {
    public var http: any HTTPExecuting
    public var timers: any TimerScheduling
    public var sockets: any SocketExecuting
    public var player: any PlayerExecuting
    public var secureStore: any KeyValueStore
    public var store: any KeyValueStore
    /// Downloads for offline viewing; a device that keeps none fails every start.
    public var downloads: any DownloadExecuting = NoDownloads()

    public init(
        http: any HTTPExecuting,
        timers: any TimerScheduling,
        sockets: any SocketExecuting,
        player: any PlayerExecuting,
        secureStore: any KeyValueStore,
        store: any KeyValueStore
    ) {
        self.http = http
        self.timers = timers
        self.sockets = sockets
        self.player = player
        self.secureStore = secureStore
        self.store = store
    }
}

extension KeyValueStore {
    /// Executes one store effect, turning failures into the core's `StoreFailed` output.
    func execute(_ request: StoreRequest) -> EffectOutput {
        do {
            switch request.op {
            case .read:
                return .stored(StoredValue(value: try read(request.key)))
            case .write(let value):
                try write(request.key, value: value)
                return .storeDone
            case .delete:
                try delete(request.key)
                return .storeDone
            }
        } catch {
            return .storeFailed(StoreFailure(message: String(describing: error)))
        }
    }
}
