import CouchverseCore
import CouchverseDesign
import Observation
import SwiftUI

/// The "Who's watching?" selection (plan 12.3): the other profiles dissolve outward and the
/// backdrop goes dark, the chosen avatar flies into its place in the sidebar header, and home
/// rises from black behind it. Only presentation state lives here; the account switch itself is
/// the core's, sent the moment the profile is chosen.
@Observable
final class ProfileChoreography {
    private(set) var isRunning = false
    /// The other profiles have blown away and the picker's backdrop has faded to black.
    private(set) var dissolved = false
    /// The chosen avatar is at (or on its way to) the header.
    private(set) var flown = false
    /// Home has risen into place.
    private(set) var revealed = false
    /// The avatar has landed; the header shows its own copy again.
    private(set) var landed = false
    private(set) var card: AccountCard?
    /// The picker's profiles at the moment of choosing, redrawn while they dissolve.
    private(set) var accounts: [AccountCard] = []
    /// Where the chosen avatar starts, in global coordinates.
    private(set) var source: CGRect = .zero
    /// Where it settles, in global coordinates; reported by the header once home is laid out.
    var target: CGRect?

    /// Whether `account`'s header avatar should wait for the flying one to land.
    func isFlying(_ account: String?) -> Bool {
        isRunning && !landed && card?.id == account
    }

    func run(card: AccountCard, accounts: [AccountCard], from source: CGRect) async {
        self.card = card
        self.accounts = accounts
        self.source = source
        target = nil
        dissolved = false
        flown = false
        revealed = false
        landed = false
        isRunning = true
        await Task.yield()

        withAnimation(.easeOut(duration: 0.65)) { dissolved = true }
        // home needs a few frames to lay out before its header can say where it is
        for _ in 0..<15 where target == nil {
            try? await Task.sleep(for: .milliseconds(16))
        }
        try? await Task.sleep(for: .milliseconds(150))
        withAnimation(.spring(response: 0.95, dampingFraction: 0.82)) { flown = true }
        withAnimation(.easeOut(duration: 0.9).delay(0.15)) { revealed = true }
        try? await Task.sleep(for: .milliseconds(1100))
        withAnimation(.easeOut(duration: 0.25)) { landed = true }
        try? await Task.sleep(for: .milliseconds(280))

        isRunning = false
        self.card = nil
        self.accounts = []
    }
}

/// Home rising from black behind the dissolving picker.
struct RiseFromBlack: ViewModifier {
    let choreography: ProfileChoreography

    func body(content: Content) -> some View {
        let hidden = choreography.isRunning && !choreography.revealed
        content
            .opacity(hidden ? 0 : 1)
            .scaleEffect(hidden ? 0.94 : 1)
            .offset(y: hidden ? 48 : 0)
    }
}

/// Draws the picker as it was and the flying avatar while a choreography runs.
struct ProfileChoreographyOverlay: View {
    let choreography: ProfileChoreography

    var body: some View {
        if choreography.isRunning, let card = choreography.card {
            GeometryReader { geometry in
                let origin = geometry.frame(in: .global).origin
                ZStack {
                    ProfilePicker(
                        accounts: choreography.accounts, active: nil, chosen: card.id,
                        dissolved: choreography.dissolved, onSelect: { _, _ in }, onAdd: {}
                    )
                    .opacity(choreography.dissolved ? 0 : 1)
                    .allowsHitTesting(false)

                    flyingAvatar(card, origin: origin)
                }
            }
            .ignoresSafeArea()
            .opacity(choreography.landed ? 0 : 1)
            .accessibilityHidden(true)
        }
    }

    private func flyingAvatar(_ card: AccountCard, origin: CGPoint) -> some View {
        let source = choreography.source
        let target = choreography.target ?? CGRect(x: 92, y: 58, width: 72, height: 72)
        let frame = choreography.flown ? target : source
        // it leaves in the tile's ring and lands in the header's
        let ring = choreography.flown ? card.tint : card.rankColor ?? card.tint
        return AvatarView(url: card.avatarUrl, seed: card.username, name: card.displayName)
            .overlay { Circle().strokeBorder(ring, lineWidth: choreography.flown ? 2 : 6) }
            .shadow(color: ring.opacity(choreography.flown ? 0 : 0.7), radius: 36)
            .frame(width: frame.width, height: frame.height)
            .position(x: frame.midX - origin.x, y: frame.midY - origin.y)
    }
}
