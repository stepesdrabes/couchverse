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
        var values = Self.ready
        values.append(.session(Fixtures.session(status)))
        snapshot("home-\(status)", values) { NavigationStack { HomeScreen() } }
    }

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

    private func snapshot(
        _ name: String, _ values: [SurfaceValue], ambience: Ambience = .flat, variants: [Variant] = Self.variants,
        fileID: StaticString = #fileID,
        file: StaticString = #filePath, testName: String = #function, line: UInt = #line,
        column: UInt = #column, @ViewBuilder _ screen: () -> some View
    ) {
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
