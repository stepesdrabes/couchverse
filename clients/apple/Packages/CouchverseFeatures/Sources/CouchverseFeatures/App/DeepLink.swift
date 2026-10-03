import Foundation

/// Which screen a link opened in the app needs. The core parses and acts on the link itself
/// (`LinkOpened`); this only decides what the shell presents meanwhile.
enum DeepLink: Equatable {
    /// `couchverse://connect?server=&code=`: sign this device in with a one-time code.
    case connect
    /// `couchverse://pair?code=`, or a server's pairing page scanned from a TV's QR code:
    /// approve another device.
    case approve

    init?(_ url: URL) {
        switch url.scheme?.lowercased() {
        case "couchverse":
            switch url.host()?.lowercased() {
            case "connect": self = .connect
            case "pair": self = .approve
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
