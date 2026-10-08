import CouchverseCore
import CouchverseDesign
import SnapshotTesting
import SwiftUI
import Testing
import UIKit

@testable import CouchverseFeatures

/// The key screens rendered from fixture view models, for every load state: on iPhone and TV in
/// English at the default text size and in Czech at the largest Dynamic Type, and on iPad in
/// English (run on an iOS simulator for iPhone and iPad, a tvOS one for TV). The backdrop is drawn
/// flat so references stay small; `designGallery` covers it. References live in `__Snapshots__`;
/// a missing one is recorded and fails the run, so a new screen is reviewed before it is
/// committed.
@MainActor
@Suite(.serialized, .snapshots(record: .missing))
struct ScreenSnapshots {
    struct Variant {
        let name: String
        let config: ViewImageConfig
        let language: String
        let size: UIContentSizeCategory
    }

    static var variants: [Variant] {
        let largest = UIContentSizeCategory.accessibilityExtraExtraExtraLarge
        #if os(tvOS)
            return [
                Variant(name: "tv-en", config: .tv, language: "en", size: .large),
                Variant(name: "tv-cs-largest", config: .tv, language: "cs", size: largest),
            ]
        #else
            return [
                Variant(name: "iphone-en", config: .iPhone13Pro, language: "en", size: .large),
                Variant(name: "iphone-cs-largest", config: .iPhone13Pro, language: "cs", size: largest),
                Variant(name: "ipad-en", config: .iPadPro11(.portrait), language: "en", size: .large),
            ]
        #endif
    }

    static let ready: [SurfaceValue] = [
        .app(AppView(phase: .ready, activeAccount: Fixtures.accounts.active)),
        .accounts(Fixtures.accounts),
        .servers(Fixtures.servers),
        .session(Fixtures.session(.loaded)),
    ]

    @Test func welcome() {
        snapshot("welcome", [.app(AppView(phase: .welcome, activeAccount: nil))]) {
            NavigationStack { WelcomeScreen {} }
        }
    }

    @Test(arguments: [LoadStatus.idle, .loading, .failed])
    func addServer(status: LoadStatus) {
        let servers = Fixtures.addServer(status, problem: status == .failed ? "not_a_server" : nil)
        snapshot("add-server-\(status)", [.servers(servers)]) {
            NavigationStack { AddServerScreen() }
        }
    }

    @Test(arguments: ["idle", "loading", "failed", "pairing", "pairing-expired"])
    func signIn(state: String) {
        let view =
            switch state {
            case "loading": Fixtures.signIn(.loading)
            case "failed": Fixtures.signIn(.failed, problem: "invalid_credentials")
            case "pairing": Fixtures.pairing(.waiting)
            case "pairing-expired": Fixtures.pairing(.expired)
            default: Fixtures.signIn(.idle)
            }
        snapshot("sign-in-\(state)", [.servers(Fixtures.servers), .signIn(view)]) {
            NavigationStack { SignInScreen(serverId: Fixtures.serverId, username: nil) {} }
        }
    }

    @Test func whosWatching() {
        snapshot(
            "whos-watching", [.app(AppView(phase: .chooseAccount, activeAccount: nil)), .accounts(Fixtures.accounts)]
        ) {
            WhosWatchingScreen()
        }
    }

    @Test(arguments: [LoadStatus.loading, .loaded, .failed, .stale])
    func home(status: LoadStatus) {
        snapshot("home-\(status)", Self.ready + [.home(Fixtures.home(status))]) { NavigationStack { HomeScreen() } }
    }

    @Test(arguments: [LoadStatus.loading, .loaded, .notFound, .failed, .stale])
    func movie(status: LoadStatus) {
        let view = Fixtures.title(status)
        snapshot("title-movie-\(status)", Self.ready + [.title(view.slug, view)]) {
            NavigationStack { TitleScreen(slug: view.slug) }
        }
    }

    @Test func series() {
        let view = Fixtures.title(.loaded, series: true)
        snapshot("title-series", Self.ready + [.title(view.slug, view)]) {
            NavigationStack { TitleScreen(slug: view.slug) }
        }
    }

    @Test(arguments: ["loading", "loaded", "failed", "empty", "genre"])
    func browse(state: String) {
        let key = state == "genre" ? BrowseKey(kind: nil, genre: "Drama", sort: .added) : BrowseKey.movies
        let view =
            switch state {
            case "loading": Fixtures.browse(.loading, key: key)
            case "failed": Fixtures.browse(.failed, key: key)
            case "empty": Fixtures.browse(.loaded, key: key, empty: true)
            default: Fixtures.browse(.loaded, key: key)
            }
        snapshot("browse-\(state)", Self.ready + [.browse(key, view), .genres(Fixtures.genres(.loaded))]) {
            NavigationStack { BrowseScreen(key: key) }
        }
    }

    @Test(arguments: [LoadStatus.loading, .loaded])
    func genres(status: LoadStatus) {
        snapshot("genres-\(status)", Self.ready + [.genres(Fixtures.genres(status))]) {
            NavigationStack { GenresScreen() }
        }
    }

    @Test(arguments: ["loaded", "empty"])
    func myList(state: String) {
        snapshot("my-list-\(state)", Self.ready + [.myList(Fixtures.myList(.loaded, empty: state == "empty"))]) {
            NavigationStack { MyListScreen() }
        }
    }

    @Test(arguments: ["idle", "loading", "results", "no-results"])
    func search(state: String) {
        let view =
            switch state {
            case "loading": Fixtures.search("glass", .loading)
            case "results": Fixtures.search("glass", .loaded)
            case "no-results": Fixtures.search("zebra", .loaded, empty: true)
            default: Fixtures.search("", .idle)
            }
        snapshot("search-\(state)", Self.ready + [.search(view)]) {
            NavigationStack { SearchScreen() }
        }
    }

    /// The player's own states; the picture itself is the system's and needs a host app.
    @Test(arguments: ["preparing", "failed", "unsupported"])
    func player(state: String) {
        snapshot("player-\(state)", Self.ready + [.player(Fixtures.player(state))]) {
            PlayerScreen().environment(PlayerController())
        }
    }

    #if os(iOS)
        @Test func playerNextUp() {
            snapshot("player-next", Self.ready + [.player(Fixtures.player("next"))]) {
                PlayerScreen().environment(PlayerController())
            }
        }
    #endif

    @Test func settings() {
        snapshot("settings", Self.ready) { NavigationStack { SettingsScreen() } }
    }

    @Test(arguments: [LoadStatus.loading, .loaded, .failed, .stale])
    func devices(status: LoadStatus) {
        snapshot("devices-\(status)", Self.ready + [.devices(Fixtures.devices(status))]) {
            NavigationStack { DevicesScreen() }
        }
    }

    @Test func approveDeviceCode() {
        snapshot("approve-code", Self.ready) { ApproveDeviceScreen() }
    }

    @Test(arguments: ["loading", "loaded", "not-found", "failed", "approved"])
    func approveDevice(state: String) {
        let approval =
            switch state {
            case "loading": Fixtures.approval(.loading)
            case "not-found": Fixtures.approval(.notFound)
            case "failed": Fixtures.approval(.failed)
            case "approved": Fixtures.approval(.loaded, outcome: .approved)
            default: Fixtures.approval(.loaded)
            }
        snapshot("approve-\(state)", Self.ready + [.pairingApproval(approval)]) {
            ApproveDeviceScreen(openedFromLink: true)
        }
    }

    #if os(iOS)
        @Test func accountSwitcher() {
            snapshot("account-switcher", Self.ready) { AccountSwitcherSheet() }
        }
    #endif

    /// With the glow and glass the screens leave out, once: on iPhone at the default text size,
    /// since a prominent glass button at the largest sizes renders nothing without a host app.
    @Test func designGallery() {
        snapshot("design", Self.ready, ambience: .still, variants: Self.variants.filter { $0.name == "iphone-en" }) {
            DesignGallery()
        }
    }

    /// The screen scale the references were recorded at. Text is rasterized at the simulator's
    /// scale even though screens render into a 1x image, so a 1080p Apple TV (1x) or an iPad (2x)
    /// shifts every glyph edge: `make apple-test` runs on the pinned simulators (`make apple-sims`).
    static let referenceScale: CGFloat = Idiom.isTV ? 2 : 3

    private func snapshot(
        _ name: String, _ values: [SurfaceValue], ambience: Ambience = .flat, variants: [Variant] = Self.variants,
        fileID: StaticString = #fileID,
        file: StaticString = #filePath, testName: String = #function, line: UInt = #line,
        column: UInt = #column, @ViewBuilder _ screen: () -> some View
    ) {
        let scale = ScreenRenderer.screenScale
        guard scale == Self.referenceScale else {
            Issue.record(
                """
                Snapshot references are recorded on a \(Int(Self.referenceScale))x simulator and this one is \
                \(Int(scale))x: run them on the simulators `make apple-sims` creates (Apple TV 4K at 4K, \
                iPhone 17), as `make apple-test` does.
                """)
            return
        }
        for variant in variants {
            L10n.language = variant.language
            let core = CoreRuntime(fixture: Idiom.isTV ? .tvos : .ios, values)
            let view = screen()
                .environment(core)
                .environment(ProfileChoreography())
                .accent(core.session.accent)
                .environment(\.locale, L10n.locale)
                .environment(\.ambience, ambience)
                .environment(\.referenceDate, Fixtures.now)
                .preferredColorScheme(.dark)
            // the apps are dark only (UIUserInterfaceStyle in their Info.plist)
            let traits = UITraitCollection {
                $0.preferredContentSizeCategory = variant.size
                $0.userInterfaceStyle = .dark
                $0.displayScale = 1
            }
            assertSnapshot(
                of: ScreenRenderer.image(of: view, config: variant.config, traits: traits),
                as: .image(precision: 0.99, perceptualPrecision: 0.97),
                named: "\(name).\(variant.name)", fileID: fileID, file: file, testName: "screen", line: line,
                column: column)
        }
        L10n.language = "en"
    }
}
