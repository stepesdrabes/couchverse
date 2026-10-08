import Foundation

// Placeholders a runtime starts from; a live runtime replaces them with the core's own views
// before anything is shown.

extension ServersView {
    public static let empty = ServersView(
        servers: [], add: AddServerView(status: .idle, address: "", added: nil, problem: nil))
}

extension AccountsView {
    public static let empty = AccountsView(accounts: [], active: nil)
}

extension SignInView {
    public static let idle = SignInView(
        serverId: nil, status: .idle, problem: nil, pairing: nil, signedIn: nil)
}

extension DevicesView {
    public static let idle = DevicesView(status: .idle, devices: [], problem: nil)
}

extension PairingApprovalView {
    public static let idle = PairingApprovalView(
        status: .idle, code: "", deviceName: "", platform: "", outcome: nil, problem: nil)
}

extension AccentPalette {
    /// The design tokens' default red, as the core derives it.
    public static let fallback = AccentPalette(
        accent: "#e50914", strong: "#b30710", soft: "#e5091429", onAccent: "#ffffff", ink: "#ec474f")
}

extension SessionView {
    public static func signedOut(language: String) -> SessionView {
        SessionView(
            status: .idle, accountId: nil, user: nil,
            features: Features(couch: true, rankings: true, downloads: true), language: language,
            accent: .fallback, problem: nil, offline: false)
    }
}

extension HomeView {
    public static let idle = HomeView(status: .idle, featured: [], rows: [])
}

extension TitleView {
    /// A title page before the core rendered it.
    public static func opening(_ slug: String) -> TitleView {
        TitleView(slug: slug, status: .loading)
    }
}

extension BrowseView {
    /// A listing before the core rendered it.
    public static func opening(_ key: BrowseKey) -> BrowseView {
        BrowseView(key: key, status: .loading, cards: [], total: 0, more: false, loadingMore: false)
    }
}

extension GenresView {
    public static let idle = GenresView(status: .idle, genres: [])
}

extension MyListView {
    public static let idle = MyListView(status: .idle, cards: [])
}

extension SearchView {
    public static let idle = SearchView(query: "", status: .idle, cards: [])
}

extension NoticesView {
    public static let empty = NoticesView(notices: [])
}

extension PlayerView {
    public static let closed = PlayerView(
        status: .idle, title: "", subtitle: "", titleSlug: "", qualities: [], quality: "", audio: [],
        subtitles: [], seasons: [], shuffleAvailable: false, shuffle: false, linear: false)
}

extension DownloadsView {
    public static let idle = DownloadsView(status: .idle, items: [], usedBytes: 0)
}
