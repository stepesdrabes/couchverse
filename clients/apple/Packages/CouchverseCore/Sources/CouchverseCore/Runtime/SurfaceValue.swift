import Foundation

/// A decoded view model, tagged with the surface it belongs to.
public enum SurfaceValue: Sendable, Hashable {
    case app(AppView)
    case servers(ServersView)
    case accounts(AccountsView)
    case signIn(SignInView)
    case devices(DevicesView)
    case pairingApproval(PairingApprovalView)
    case session(SessionView)
    case home(HomeView)
    case title(String, TitleView)
    case browse(BrowseKey, BrowseView)
    case genres(GenresView)
    case myList(MyListView)
    case search(SearchView)
    case notices(NoticesView)
    case player(PlayerView)
    case downloads(DownloadsView)
    case couch(CouchView)

    /// The surfaces with one view each, read at start-up; `Title` and `Browse` exist per slug and
    /// listing once a screen opens them, `Markdown` is read on demand, and the rest get screens
    /// in later slices.
    static let fixed: [Surface] = [
        .app, .servers, .accounts, .signIn, .devices, .pairingApproval, .session, .home, .genres,
        .myList, .search, .notices, .player,
        .downloads,
        .couch,
    ]

    static func publishes(_ surface: Surface) -> Bool {
        switch surface {
        case .title, .browse: true
        default: fixed.contains(surface)
        }
    }

    var surface: Surface {
        switch self {
        case .app: .app
        case .servers: .servers
        case .accounts: .accounts
        case .signIn: .signIn
        case .devices: .devices
        case .pairingApproval: .pairingApproval
        case .session: .session
        case .home: .home
        case .title(let slug, _): .title(slug)
        case .browse(let key, _): .browse(key)
        case .genres: .genres
        case .myList: .myList
        case .search: .search
        case .notices: .notices
        case .player: .player
        case .downloads: .downloads
        case .couch: .couch
        }
    }

    /// Decodes a surface's JSON. S2 measured `JSONDecoder`, not the core, as the dominant cost of a
    /// render, so batches are decoded off the main actor.
    static func decode(_ surface: Surface, _ json: String) throws -> SurfaceValue? {
        let decoder = JSONDecoder()
        let data = Data(json.utf8)
        return switch surface {
        case .app: .app(try decoder.decode(AppView.self, from: data))
        case .servers: .servers(try decoder.decode(ServersView.self, from: data))
        case .accounts: .accounts(try decoder.decode(AccountsView.self, from: data))
        case .signIn: .signIn(try decoder.decode(SignInView.self, from: data))
        case .devices: .devices(try decoder.decode(DevicesView.self, from: data))
        case .pairingApproval: .pairingApproval(try decoder.decode(PairingApprovalView.self, from: data))
        case .session: .session(try decoder.decode(SessionView.self, from: data))
        case .home: .home(try decoder.decode(HomeView.self, from: data))
        case .title(let slug): .title(slug, try decoder.decode(TitleView.self, from: data))
        case .browse(let key): .browse(key, try decoder.decode(BrowseView.self, from: data))
        case .genres: .genres(try decoder.decode(GenresView.self, from: data))
        case .myList: .myList(try decoder.decode(MyListView.self, from: data))
        case .search: .search(try decoder.decode(SearchView.self, from: data))
        case .notices: .notices(try decoder.decode(NoticesView.self, from: data))
        case .player: .player(try decoder.decode(PlayerView.self, from: data))
        case .downloads: .downloads(try decoder.decode(DownloadsView.self, from: data))
        case .couch: .couch(try decoder.decode(CouchView.self, from: data))
        default: nil
        }
    }

    @concurrent
    static func decodeBatch(_ payloads: [RenderedSurface]) async -> [SurfaceValue] {
        payloads.compactMap { payload in
            do {
                return try decode(payload.surface, payload.json)
            } catch {
                coreLog.fault("undecodable \(String(describing: payload.surface)) view: \(error)")
                return nil
            }
        }
    }
}

/// A surface's view model as the core rendered it, not yet decoded.
struct RenderedSurface: Sendable {
    let surface: Surface
    let json: String
}
