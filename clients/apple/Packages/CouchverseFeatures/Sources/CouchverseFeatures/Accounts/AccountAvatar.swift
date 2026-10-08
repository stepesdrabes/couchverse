import CouchverseCore
import CouchverseDesign
import SwiftUI

extension AccountCard {
    /// The profile's own colour, which tints "Who's watching?" around it: its banner's accent, or
    /// without a banner the hue of its identicon, so it matches the placeholder avatar.
    var tint: Color {
        accent.flatMap { Color(hex: $0.accent) } ?? Identicon(seed: username).color
    }

    /// The colour of its rank ring: the tier's, once a rank is known.
    var rankColor: Color? {
        rank.map { RanksStyle.tier($0.tier.code) }
    }

    /// `Remote Wrangler · Level 2`, for a signed-in profile with a rank.
    var rankLine: String? {
        signedIn ? rank.map(RanksWords.rankLine) : nil
    }
}

/// An account's picture inside its rank ring (the tier's colour, filled to the progress through
/// it) once a rank is known, and on its own until then; `size` is the whole, ring included.
struct AccountAvatar: View {
    let card: AccountCard
    let size: CGFloat

    var body: some View {
        let avatar = AvatarView(url: card.avatarUrl, seed: card.username, name: card.displayName)
        if let rank = card.rank, let color = card.rankColor {
            let lineWidth = max(size / 16, 2)
            RankRing(color: color, progress: Double(rank.percent) / 100, lineWidth: lineWidth) {
                avatar.frame(width: size - lineWidth * 3, height: size - lineWidth * 3)
            }
        } else {
            avatar.frame(width: size, height: size)
        }
    }
}
