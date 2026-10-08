import Foundation

/// Where the app and its extensions meet. With `EXTENSIONS_ENABLED` the build names an App Group in
/// each Info.plist and the snapshot lives in its container; without it there is no group and the
/// app keeps the snapshot in its own caches, where its intents still read it.
public enum SharedContainer {
    /// The Info.plist key that names the App Group; empty without `EXTENSIONS_ENABLED`.
    public static let groupKey = "CouchverseAppGroup"
    /// The Continue Watching widget's kind, for reloading its timelines.
    public static let continueWidgetKind = "ContinueWatching"

    /// The App Group `bundle` was built with, if any.
    public static func group(in bundle: Bundle = .main) -> String? {
        guard let group = bundle.object(forInfoDictionaryKey: groupKey) as? String, !group.isEmpty else {
            return nil
        }
        return group
    }

    /// Whether the build carries the extensions (the widget and the Live Activity, Top Shelf).
    public static func extensionsEnabled(in bundle: Bundle = .main) -> Bool {
        group(in: bundle) != nil
    }

    /// The directory the snapshot is kept in: the group's caches when the system grants the
    /// group, else this process's own caches.
    public static func directory(bundle: Bundle = .main, files: FileManager = .default) -> URL? {
        if let group = group(in: bundle),
            let container = files.containerURL(forSecurityApplicationGroupIdentifier: group)
        {
            return container.appending(path: "Library/Caches", directoryHint: .isDirectory)
        }
        return files.urls(for: .cachesDirectory, in: .userDomainMask).first
    }
}

/// A `#rrggbb` colour's components from 0 to 1, for extensions that have no design system.
public struct HexColor: Sendable, Hashable {
    public let red: Double
    public let green: Double
    public let blue: Double

    public init?(_ hex: String) {
        let digits = hex.hasPrefix("#") ? hex.dropFirst() : Substring(hex)
        guard digits.count == 6, let value = UInt32(digits, radix: 16) else { return nil }
        red = Double((value >> 16) & 0xFF) / 255
        green = Double((value >> 8) & 0xFF) / 255
        blue = Double(value & 0xFF) / 255
    }
}
