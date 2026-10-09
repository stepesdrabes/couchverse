import CouchverseCore
import CouchverseDesign
import SwiftUI

/// An achievement just unlocked, over the app or over the player (plan 10.5), for a few seconds;
/// then the core shows the next it queued (`CelebrationDismissed`). It never takes the focus, so it
/// cannot interrupt what the remote is doing; a tap sends it away early on touch devices. With
/// Reduce Motion it fades instead of springing in and stays a little longer, as nothing moving
/// cues the read.
struct CelebrationOverlay: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    /// The one this overlay sent away, so a second tap before the core answers skips nothing.
    @State private var dismissed: String?

    static func duration(reduceMotion: Bool) -> Duration {
        reduceMotion ? .milliseconds(6500) : .milliseconds(4500)
    }

    private var card: AchievementCard? {
        core.session.features.rankings ? core.rank.celebration : nil
    }

    var body: some View {
        VStack {
            if let card {
                CelebrationCard(card: card)
                    .id(card.code)
                    .transition(
                        reduceMotion
                            ? .opacity : .move(edge: .top).combined(with: .scale(scale: 0.9)).combined(with: .opacity)
                    )
                    .onTapGesture { dismiss(card) }
                    .task(id: card.code) {
                        let announcement = "\(L10n.achievementUnlocked): \(RanksWords.achievementName(card.code))"
                        AccessibilityNotification.Announcement(announcement).post()
                        try? await Task.sleep(for: Self.duration(reduceMotion: reduceMotion))
                        if !Task.isCancelled {
                            dismiss(card)
                        }
                    }
            }
        }
        .padding(Idiom.isTV ? 60 : Tokens.Spacing.lg)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: Idiom.isTV ? .topTrailing : .top)
        .animation(reduceMotion ? .easeInOut(duration: 0.3) : Tokens.Motion.bouncy, value: card?.code)
        #if os(iOS)
            .sensoryFeedback(.success, trigger: card?.code) { _, code in code != nil }
        #endif
    }

    private func dismiss(_ card: AchievementCard) {
        guard dismissed != card.code else { return }
        dismissed = card.code
        core.send(.celebrationDismissed)
    }
}

/// The unlocked achievement: its medal, name and what it asked for, the metal and the XP it paid.
struct CelebrationCard: View {
    let card: AchievementCard

    @Environment(\.ambience) private var ambience
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    var body: some View {
        let medal = RanksStyle.medal(card.tier)
        let reward = "\(RanksWords.medal(card.tier)) \u{00B7} \(L10n.achievementReward(xp: RanksWords.number(card.xp)))"
        // at the largest text sizes on a phone or tablet the medal goes above the words, which
        // need the width
        let layout =
            dynamicTypeSize.isAccessibilitySize && !Idiom.isTV
            ? AnyLayout(VStackLayout(alignment: .leading, spacing: Tokens.Spacing.md))
            : AnyLayout(HStackLayout(alignment: .center, spacing: Idiom.isTV ? Tokens.Spacing.xl : Tokens.Spacing.lg))
        layout {
            MedalBadge(card: card, size: Idiom.isTV ? 104 : 60)
            VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
                Text(L10n.achievementUnlocked)
                    .typeRole(Tokens.TypeRamp.eyebrow)
                    .textCase(.uppercase)
                    .foregroundStyle(medal.ring)
                Text(RanksWords.achievementName(card.code))
                    .typeRole(Tokens.TypeRamp.section)
                    .foregroundStyle(Tokens.Palette.text)
                if let description = RanksWords.achievementDescription(card.code) {
                    Text(description)
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.mutedText)
                }
                Text(reward)
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.text)
            }
            .fixedSize(horizontal: false, vertical: true)
        }
        .padding(Idiom.isTV ? Tokens.Spacing.xxl : Tokens.Spacing.lg)
        .frame(maxWidth: Idiom.isTV ? 760 : 460, alignment: .leading)
        .celebrationSurface(solid: ambience == .flat)
        .overlay {
            RoundedRectangle(cornerRadius: Tokens.Radius.card * 1.5, style: .continuous)
                .strokeBorder(medal.ring.opacity(0.7), lineWidth: Idiom.isTV ? 3 : 1.5)
        }
        .shadow(color: medal.glow, radius: Idiom.isTV ? 40 : 20)
        .accessibilityElement(children: .combine)
        .accessibilityAddTraits(.isStaticText)
    }
}

extension View {
    /// Glass where it renders; solid in snapshots, which have no host app to draw glass in.
    @ViewBuilder fileprivate func celebrationSurface(solid: Bool) -> some View {
        let shape = RoundedRectangle(cornerRadius: Tokens.Radius.card * 1.5, style: .continuous)
        if solid {
            background(Tokens.Palette.surface, in: shape)
        } else {
            glassEffect(.regular, in: shape)
        }
    }
}
