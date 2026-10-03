import Foundation

/// Non-secret state (servers, accounts) as one small file per key.
public struct FileStore: KeyValueStore {
    public let directory: URL

    public init(directory: URL) {
        self.directory = directory
    }

    /// `Application Support/<name>`, created on first write.
    public static func applicationSupport(_ name: String = "Couchverse") -> FileStore {
        let base = URL.applicationSupportDirectory
        return FileStore(directory: base.appending(path: name, directoryHint: .isDirectory))
    }

    public func read(_ key: String) throws -> String? {
        do {
            return String(decoding: try Data(contentsOf: url(for: key)), as: UTF8.self)
        } catch CocoaError.fileReadNoSuchFile {
            return nil
        }
    }

    public func write(_ key: String, value: String) throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try Data(value.utf8).write(to: url(for: key), options: .atomic)
    }

    public func delete(_ key: String) throws {
        do {
            try FileManager.default.removeItem(at: url(for: key))
        } catch CocoaError.fileNoSuchFile {
            return
        }
    }

    /// Keys are the core's own (`servers`, `accounts`); anything else is escaped so a key can
    /// never leave the directory.
    func url(for key: String) -> URL {
        let allowed = CharacterSet(charactersIn: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_")
        let name = key.addingPercentEncoding(withAllowedCharacters: allowed) ?? "invalid"
        return directory.appending(path: "\(name).json", directoryHint: .notDirectory)
    }
}
