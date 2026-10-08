import CouchverseCore
import Foundation

/// Which screen a link opened in the app needs. The core parses and acts on sign-in links itself
/// (`LinkOpened`); this only decides what the shell presents meanwhile. A couch link fills in the
/// join screen, which sends its code.
enum DeepLink: Equatable {
    /// `couchverse://connect?server=&code=`: sign this device in with a one-time code.
    case connect
    /// `couchverse://pair?code=`, or a server's pairing page scanned from a TV's QR code:
    /// approve another device.
    case approve
    /// `couchverse://couch/123456`, or the join page a host's QR code shows: join a couch session
    /// (`CouchLink` reads the code).
    case couch
    /// `couchverse://title/<slug>`: a title's page.
    case title(slug: String)
    /// `couchverse://play/<movie|episode>/<id>`: play it from where it stopped (Continue Watching
    /// outside the app). The same shapes as on Android.
    case play(PlayTarget)

    init?(_ url: URL) {
        if CouchLink.code(url.absoluteString) != nil {
            self = .couch
            return
        }
        switch url.scheme?.lowercased() {
        case "couchverse":
            let path = url.pathComponents.filter { $0 != "/" }
            switch url.host()?.lowercased() {
            case "connect": self = .connect
            case "pair": self = .approve
            case "title":
                guard path.count == 1 else { return nil }
                self = .title(slug: path[0])
            case "play":
                guard path.count == 2, let kind = PlayKind(rawValue: path[0].lowercased()) else { return nil }
                self = .play(PlayTarget(kind: kind, id: path[1]))
            default: return nil
            }
        case "http", "https":
            let components = URLComponents(url: url, resolvingAgainstBaseURL: false)
            let hasCode = components?.queryItems?.contains { $0.name == "code" && $0.value?.isEmpty == false }
            guard url.path().trimmingCharacters(in: ["/"]) == "pair", hasCode == true else { return nil }
            self = .approve
        default:
            return nil
        }
    }

    init?(_ string: String) {
        guard let url = URL(string: string.trimmingCharacters(in: .whitespacesAndNewlines)) else { return nil }
        self.init(url)
    }
}
