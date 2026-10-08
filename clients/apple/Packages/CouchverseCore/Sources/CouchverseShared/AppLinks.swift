import Foundation

/// The `couchverse://` links that open the app on a title or play something, the same shapes on
/// every client: what the widget, Top Shelf, Spotlight and the intents hand back to the app.
public enum AppLinks {
    /// `couchverse://title/<slug>`: the title's page.
    public static func title(slug: String) -> String {
        "couchverse://title/\(component(slug))"
    }

    /// `couchverse://play/<movie|episode>/<id>`: play it from where it stopped.
    public static func play(kind: String, id: String) -> String {
        "couchverse://play/\(component(kind))/\(component(id))"
    }

    private static func component(_ value: String) -> String {
        value.addingPercentEncoding(withAllowedCharacters: pathComponent) ?? value
    }

    private static let pathComponent = CharacterSet.urlPathAllowed.subtracting(CharacterSet(charactersIn: "/"))
}
