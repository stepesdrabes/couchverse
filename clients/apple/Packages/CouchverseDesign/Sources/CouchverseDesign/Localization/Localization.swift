import Foundation
import Observation

/// UI strings in the display language, which follows the signed-in account rather than the
/// system (D21). The generated accessors are static, so the language they read is observable:
/// a view that shows a string re-renders when the language changes, without rebuilding the app.
public enum L10n {
    public static let languages = ["en", "cs"]

    public static var language: String {
        get { state.language }
        set {
            guard languages.contains(newValue), newValue != state.language else { return }
            state.language = newValue
        }
    }

    public static var locale: Locale { Locale(identifier: language) }

    static func string(_ key: String) -> String {
        bundle(for: language).localizedString(forKey: key, value: key, table: "Localizable")
    }

    /// Plural variants come from the compiled catalog's stringsdict and are chosen by the display
    /// language's rules (Czech has one, few and other).
    static func format(_ key: String, _ arguments: [any CVarArg]) -> String {
        String(format: string(key), locale: locale, arguments: arguments)
    }

    @Observable
    final class State {
        var language = "en"
    }

    private static let state = State()
    private static var bundles: [String: Bundle] = [:]

    private static func bundle(for language: String) -> Bundle {
        if let bundle = bundles[language] {
            return bundle
        }
        let bundle = Bundle.module.path(forResource: language, ofType: "lproj").flatMap(Bundle.init(path:))
        bundles[language] = bundle ?? .module
        return bundle ?? .module
    }
}
