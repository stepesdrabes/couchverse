import CouchverseCore
import CouchverseDesign
import SwiftUI
import Testing

@testable import CouchverseFeatures

/// The couch's screens and states, drawn like the rest of `ScreenSnapshots` (references in
/// `__Snapshots__/CouchSnapshots`).
extension ScreenSnapshots {
    /// The host's code and QR code, a follower's view of the couch, starting one and its end.
    @Test(arguments: ["hosting", "following", "starting", "start-failed", "idle", "ended"])
    func couchPanel(state: String) {
        snapshot("couch-panel-\(state)", Self.ready + [.couch(Fixtures.couch(state))]) {
            CouchPanelScreen()
        }
    }

    @Test(arguments: ["empty", "typed", "joining", "failed", "not-host"])
    func joinCouch(state: String) {
        let (code, couch) =
            switch state {
            case "typed": ("123456", "idle")
            case "joining": ("123456", "starting")
            case "failed": ("123456", "join-failed")
            case "not-host": ("123456", "not-host")
            default: ("", "idle")
            }
        snapshot("couch-join-\(state)", Self.ready + [.couch(Fixtures.couch(couch))]) {
            JoinCouchScreen(invite: CouchInvite(code: code), attempted: state != "empty" && state != "typed") {}
        }
    }

    /// Joining without an account: the server's address too, from a link here.
    @Test func joinCouchAsAGuest() {
        let welcome: [SurfaceValue] = [
            .app(AppView(phase: .welcome, activeAccount: nil)), .couch(Fixtures.couch("idle")),
        ]
        snapshot("couch-join-guest", welcome) {
            JoinCouchScreen(invite: CouchInvite(code: "123456", server: "http://192.168.1.5:8080")) {}
        }
    }

    /// A follower before the host plays anything: connecting, the host choosing, the host away.
    @Test(arguments: ["connecting", "waiting", "away"])
    func couchWaiting(state: String) {
        snapshot("couch-waiting-\(state)", Self.ready + [.couch(Fixtures.couch(state))]) {
            CouchWaitingScreen()
        }
    }

    @Test func couchEnded() {
        snapshot("couch-ended", Self.ready + [.couch(Fixtures.couch("ended"))]) {
            CouchEndedScreen {}
        }
    }

    /// What a follower sees over the picture: reactions and what the host is doing.
    @Test(arguments: ["following", "host-paused", "resynced"])
    func couchPlayer(state: String) {
        snapshot(
            "couch-player-\(state)", Self.ready + [.player(Fixtures.followerPlayer), .couch(Fixtures.couch(state))]
        ) {
            #if os(iOS)
                PlayerScreen().environment(PlayerController())
            #else
                // the TV's picture and transport bar are the system's; only the overlay is ours
                ZStack {
                    Color.black
                    CouchPlayerOverlay()
                }
            #endif
        }
    }

    @Test func settingsOnACouch() {
        snapshot("couch-settings", Self.ready + [.couch(Fixtures.couch("hosting"))]) {
            NavigationStack { SettingsScreen() }
        }
    }

    #if os(iOS)
        @Test(arguments: ["remote", "remote-paused"])
        func couchRemote(state: String) {
            snapshot("couch-\(state)", Self.ready + [.couch(Fixtures.couch(state))]) {
                CouchRemoteScreen()
            }
        }

        @Test func couchReactions() {
            snapshot("couch-reactions", Self.ready) {
                ReactionBar(choices: Reactions.choices(recent: ["🦄", "🔥"])) { _ in }
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .background(Tokens.Palette.bg)
            }
        }
    #endif

    #if os(tvOS)
        /// The TV's Couch tab: joining by code, or the session it hosts.
        @Test(arguments: ["idle", "hosting"])
        func couchHub(state: String) {
            snapshot("couch-hub-\(state)", Self.ready + [.couch(Fixtures.couch(state))]) {
                NavigationStack { CouchHubScreen() }
            }
        }
    #endif
}
