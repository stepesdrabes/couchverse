import Foundation

/// Non-secret state in user defaults. tvOS guarantees an app only ~500 KB of persistent local
/// storage, and that is user defaults; files anywhere else may be purged, which would make the TV
/// forget its servers and accounts while their tokens survive in the Keychain.
public struct DefaultsStore: KeyValueStore {
    public let suiteName: String

    public init(suiteName: String) {
        self.suiteName = suiteName
    }

    public func read(_ key: String) throws -> String? {
        try defaults().string(forKey: key)
    }

    public func write(_ key: String, value: String) throws {
        try defaults().set(value, forKey: key)
    }

    public func delete(_ key: String) throws {
        try defaults().removeObject(forKey: key)
    }

    private func defaults() throws -> UserDefaults {
        guard let defaults = UserDefaults(suiteName: suiteName) else {
            throw CocoaError(.featureUnsupported)
        }
        return defaults
    }
}
