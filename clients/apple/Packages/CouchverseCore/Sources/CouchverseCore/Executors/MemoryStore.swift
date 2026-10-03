import Foundation
import Synchronization

/// A store that forgets everything on exit: UI tests start every launch from a clean install.
public final class MemoryStore: KeyValueStore {
    private let values: Mutex<[String: String]>

    public init(_ values: [String: String] = [:]) {
        self.values = Mutex(values)
    }

    public var snapshot: [String: String] { values.withLock { $0 } }

    public func read(_ key: String) throws -> String? {
        values.withLock { $0[key] }
    }

    public func write(_ key: String, value: String) throws {
        values.withLock { $0[key] = value }
    }

    public func delete(_ key: String) throws {
        _ = values.withLock { $0.removeValue(forKey: key) }
    }
}
