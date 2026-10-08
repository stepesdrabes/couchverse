import CouchverseCore
import CouchverseDesign
import SwiftUI

/// What covers the whole app: the player while the core plays something, else the screen that
/// stands in for it on a couch (this device as a remote, a follower waiting for the host, the end
/// of the session for a moment), else a join screen a link, Settings or the welcome screen asked
/// for. One cover shows them all, so moving between them never presents one modal over another
/// being dismissed.
enum AppCover: Equatable {
    case player
    case remote
    case waiting
    case ended
    case join(CouchInvite)
}

/// The cover's presentation state: the join screen asked for, and what this device was on the
/// last couch, which outlives the session in the core's view.
@MainActor
@Observable
final class CoverRequests {
    /// A link's code and server, or empty fields for a join screen opened from Settings or the
    /// welcome screen.
    var joining: CouchInvite?
    private(set) var lastRole: CouchRole?
    /// The end of a session this device watched or steered has been shown long enough.
    var endSeen = false

    func cover(playing: Bool, couch: CouchView, ready: Bool) -> AppCover? {
        if playing {
            return .player
        }
        if couch.isLive && couch.role == .remote {
            return .remote
        }
        if couch.isLive && couch.role == .follower {
            return .waiting
        }
        // leaving is the viewer's own doing; anything else ending the session is worth a word
        let viewer = lastRole == .follower || lastRole == .remote
        if couch.status == .ended && couch.ended != "left" && viewer && !endSeen {
            return .ended
        }
        // a join naming its server needs no account; one without waits for one
        if let joining, ready || joining.server != nil {
            return .join(joining)
        }
        return nil
    }

    /// The core put this device on a couch: whatever join screen led there is done.
    func seated(_ role: CouchRole) {
        lastRole = role
        endSeen = false
        joining = nil
    }
}

extension CoreRuntime {
    /// Closes the player; a couch follower leaves the couch with it, or the host's next title would
    /// bring it back.
    func closePlayer() {
        send(couch.isLive && couch.role == .follower ? .couchLeft : .playerClosed)
    }
}

extension View {
    /// Presents `AppCover` over everything; a swipe or the remote's Back closes the player in the
    /// core, or takes this device off the couch.
    func appCover(player: PlayerController?, requests: CoverRequests) -> some View {
        modifier(AppCoverPresenter(player: player, requests: requests))
    }
}

private struct AppCoverPresenter: ViewModifier {
    /// Absent in previews and snapshots, which never play.
    let player: PlayerController?
    let requests: CoverRequests
    @Environment(CoreRuntime.self) private var core

    private var cover: AppCover? {
        requests.cover(playing: core.player.target != nil, couch: core.couch, ready: core.app.phase == .ready)
    }

    func body(content: Content) -> some View {
        content
            .fullScreenCover(isPresented: presented) {
                if let player {
                    AppCoverContent(requests: requests)
                        .overlay { CelebrationOverlay() }
                        .environment(core)
                        .environment(player)
                        .accent(core.session.accent)
                        .environment(\.locale, L10n.locale)
                }
            }
            .onChange(of: core.couch.role) { _, role in
                if let role {
                    requests.seated(role)
                }
            }
            .task(id: core.couch.status == .ended) {
                guard core.couch.status == .ended else { return }
                try? await Task.sleep(for: .seconds(3))
                if !Task.isCancelled {
                    requests.endSeen = true
                }
            }
    }

    private var presented: Binding<Bool> {
        Binding(
            get: { player != nil && cover != nil },
            set: { presented in
                if !presented, let cover {
                    dismissed(cover)
                }
            })
    }

    private func dismissed(_ cover: AppCover) {
        switch cover {
        case .player:
            core.closePlayer()
        case .remote, .waiting:
            core.send(.couchLeft)
        case .ended:
            requests.endSeen = true
        case .join:
            requests.joining = nil
        }
    }
}

private struct AppCoverContent: View {
    let requests: CoverRequests
    @Environment(CoreRuntime.self) private var core
    /// Kept while the cover goes away, so its last screen slides out rather than a blank one.
    @State private var shown: AppCover?

    var body: some View {
        let cover = requests.cover(
            playing: core.player.target != nil, couch: core.couch, ready: core.app.phase == .ready)
        Group {
            switch cover ?? shown {
            case .player: PlayerScreen()
            case .remote: CouchRemoteScreen()
            case .waiting: CouchWaitingScreen()
            case .ended: CouchEndedScreen { requests.endSeen = true }
            case .join(let invite): JoinCouchScreen(invite: invite) { requests.joining = nil }
            case nil: Color.black.ignoresSafeArea()
            }
        }
        .onChange(of: cover, initial: true) { _, cover in
            if let cover {
                shown = cover
            }
        }
    }
}
