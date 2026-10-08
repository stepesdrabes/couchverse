import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The signed-in app (plan 10.3): a sidebar on TV (Home, Movies, Series, Genres, My List, Couch,
/// Profile and Leaderboard while rankings are on, Settings, Search), a Liquid Glass tab bar on
/// iPhone with Search as its own tab, and the same tabs as an adaptable sidebar on iPad. Each tab
/// keeps its own navigation stack.
struct MainTabs: View {
    enum Destination: Hashable {
        case home
        case browse
        case movies
        case series
        case genres
        case myList
        case search
        case settings
        case couch
        case profile
        case leaderboard
    }

    @Environment(CoreRuntime.self) private var core
    /// Absent in previews and snapshots.
    @Environment(OpenRequests.self) private var requests: OpenRequests?
    @State private var selection = Destination.home
    @State private var homePath = NavigationPath()

    var body: some View {
        TabView(selection: $selection) {
            Tab(L10n.navHome, systemImage: "house", value: Destination.home) {
                CatalogStack(path: $homePath) { HomeScreen() }
            }
            #if os(tvOS)
                Tab(L10n.navMovies, systemImage: "film", value: Destination.movies) {
                    CatalogStack { BrowseScreen(key: .movies) }
                }
                Tab(L10n.navSeries, systemImage: "tv", value: Destination.series) {
                    CatalogStack { BrowseScreen(key: .series) }
                }
                Tab(L10n.navGenres, systemImage: "square.grid.2x2", value: Destination.genres) {
                    CatalogStack { GenresScreen() }
                }
            #else
                Tab(L10n.navBrowse, systemImage: "square.grid.2x2", value: Destination.browse) {
                    CatalogStack { BrowseHub() }
                }
            #endif
            Tab(L10n.navMyList, systemImage: "bookmark", value: Destination.myList) {
                CatalogStack { MyListScreen() }
            }
            #if os(tvOS)
                if core.session.features.couch {
                    Tab(L10n.couchOpen, systemImage: "sofa", value: Destination.couch) {
                        NavigationStack { CouchHubScreen() }
                    }
                }
                if core.session.features.rankings, let me = core.session.user?.username {
                    Tab(L10n.navProfile, systemImage: "person.crop.circle", value: Destination.profile) {
                        CatalogStack { ProfileScreen(username: me) }
                    }
                    Tab(L10n.navLeaderboard, systemImage: "trophy", value: Destination.leaderboard) {
                        CatalogStack { LeaderboardScreen() }
                    }
                }
            #endif
            Tab(L10n.navSettings, systemImage: "gearshape", value: Destination.settings) {
                CatalogStack { SettingsScreen() }
            }
            Tab(L10n.navSearch, systemImage: "magnifyingglass", value: Destination.search, role: .search) {
                CatalogStack { SearchScreen() }
            }
        }
        .tabViewStyle(.sidebarAdaptable)
        .tabViewSidebarHeader { AccountHeader() }
        .onChange(of: core.session.features.rankings) { _, on in
            if !on && (selection == .profile || selection == .leaderboard) {
                selection = .home
            }
        }
        .offlineDownloads()
        .onChange(of: requests?.pending, initial: true) { openRequested() }
        .onChange(of: requests?.pending == .continueWatching ? core.home : nil) { openRequested() }
    }

    /// Acts on a link or an intent: a title opens over Home, Continue Watching waits for the home.
    private func openRequested() {
        guard let requests, let request = requests.pending, let step = request.step(home: core.home) else { return }
        switch step {
        case .showTitle(let slug):
            selection = .home
            homePath = NavigationPath([CatalogRoute.title(slug: slug)])
        case .showMyList:
            selection = .myList
        case .play(let target):
            core.send(.playRequested(target))
        case .showHome, .wait:
            selection = .home
        }
        if step != .wait {
            requests.finish(request)
        }
    }
}

/// A tab's navigation stack, which titles, listings, profiles and the leaderboard are pushed onto;
/// `path` when something outside the stack pushes onto it too.
private struct CatalogStack<Root: View>: View {
    var path: Binding<NavigationPath>?
    @ViewBuilder let root: () -> Root

    var body: some View {
        if let path {
            NavigationStack(path: path) { content }
        } else {
            NavigationStack { content }
        }
    }

    private var content: some View {
        root().catalogDestinations().ranksDestinations()
    }
}

#if os(iOS)
    /// The phone's and tablet's Browse tab: movies, series or genres, switched at the top.
    private struct BrowseHub: View {
        enum Section: Hashable {
            case movies
            case series
            case genres
        }

        @State private var section = Section.movies

        var body: some View {
            Group {
                switch section {
                case .movies: BrowseScreen(key: .movies).id(Section.movies)
                case .series: BrowseScreen(key: .series).id(Section.series)
                case .genres: GenresScreen()
                }
            }
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .principal) {
                    Picker(L10n.navBrowse, selection: $section) {
                        Text(L10n.navMovies).tag(Section.movies)
                        Text(L10n.navSeries).tag(Section.series)
                        Text(L10n.navGenres).tag(Section.genres)
                    }
                    .pickerStyle(.segmented)
                    .fixedSize()
                }
            }
        }
    }
#endif

extension BrowseKey {
    static let movies = BrowseKey(kind: .movie, genre: nil, sort: .added)
    static let series = BrowseKey(kind: .series, genre: nil, sort: .added)
}

/// Who is signed in, at the top of the sidebar: the place a chosen profile's avatar lands, and
/// the way to switch profiles.
struct AccountHeader: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(ProfileChoreography.self) private var choreography
    @Environment(\.showProfilePicker) private var showProfilePicker
    @Environment(\.showAccountSwitcher) private var showAccountSwitcher

    private var card: AccountCard? {
        core.accounts.accounts.first { $0.id == core.app.activeAccount }
    }

    var body: some View {
        Button {
            if Idiom.isTV {
                showProfilePicker()
            } else {
                showAccountSwitcher()
            }
        } label: {
            HStack(spacing: Tokens.Spacing.md) {
                ProfileAvatar(card: card, size: Idiom.isTV ? 72 : 40)
                VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) {
                    Text(card?.displayName ?? "")
                        .typeRole(Tokens.TypeRamp.card)
                        .lineLimit(1)
                    Text(L10n.accountsSwitch)
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.mutedText)
                        .lineLimit(1)
                    RankCaption()
                }
            }
        }
        .buttonStyle(.plain)
        .accessibilityLabel(L10n.accountsSwitch)
        .accessibilityValue([card?.displayName, rankLine].compactMap { $0 }.joined(separator: ", "))
        .accessibilityIdentifier("account-header")
    }

    /// The rank the header shows under the name, while rankings are on.
    private var rankLine: String? {
        core.session.features.rankings ? core.rank.rank.map(RanksWords.rankLine) : nil
    }
}

/// The active profile's avatar in the sidebar header: where a chosen profile's avatar lands. It
/// tells the choreography where it sits and stays hidden until the flying avatar arrives.
struct ProfileAvatar: View {
    let card: AccountCard?
    let size: CGFloat
    @Environment(ProfileChoreography.self) private var choreography
    @State private var frame: CGRect = .zero

    var body: some View {
        AvatarView(url: card?.avatarUrl, seed: card?.username ?? "", name: card?.displayName ?? "")
            .frame(width: size, height: size)
            .overlay { Circle().strokeBorder(card?.tint ?? .clear, lineWidth: 2) }
            .opacity(choreography.isFlying(card?.id) ? 0 : 1)
            .onGeometryChange(for: CGRect.self) {
                $0.frame(in: .global)
            } action: { frame in
                self.frame = frame
                report()
            }
            .onChange(of: choreography.card?.id) { report() }
    }

    private func report() {
        if choreography.isRunning, choreography.card?.id == card?.id, frame != .zero {
            choreography.target = frame
        }
    }
}
