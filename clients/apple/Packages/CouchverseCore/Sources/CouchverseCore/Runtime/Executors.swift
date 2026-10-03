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
    public var secureStore: any KeyValueStore
    public var store: any KeyValueStore

    public init(
        http: any HTTPExecuting,
        timers: any TimerScheduling,
        sockets: any SocketExecuting,
        secureStore: any KeyValueStore,
        store: any KeyValueStore
    ) {
        self.http = http
        self.timers = timers
        self.sockets = sockets
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
