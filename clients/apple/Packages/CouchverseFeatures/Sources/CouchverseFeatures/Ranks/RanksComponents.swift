import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Rank and medal colours from the design tokens, so a tier is coloured the same on every client.
enum RanksStyle {
    static func tier(_ code: String) -> Color {
        Tokens.Tier.named(code).color
    }

    static func medal(_ tier: String) -> Tokens.MedalPalette {
        switch tier {
        case "silver": Tokens.Medal.silver
        case "gold": Tokens.Medal.gold
        case "platinum": Tokens.Medal.platinum
        default: Tokens.Medal.bronze
        }
    }

    /// The podium's metals, first place first.
    static let podium = [Tokens.Medal.gold, Tokens.Medal.silver, Tokens.Medal.bronze]
}

/// A block of a reading page under its heading. A TV focuses whole blocks, so the remote can
/// scroll a page that is mostly text and charts; the focused one is outlined. A block whose
/// content has its own focusable items (cards, tiles) leaves the focus to them.
struct ReadingBlock<Content: View>: View {
    let title: String?
    var detail: String?
    var focusable = true
    @ViewBuilder let content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: Idiom.isTV ? Tokens.Spacing.xl : Tokens.Spacing.md) {
            if let title {
                HStack(alignment: .firstTextBaseline, spacing: Tokens.Spacing.md) {
                    Text(title)
                        .typeRole(Tokens.TypeRamp.section)
                        .foregroundStyle(Tokens.Palette.text)
                        .accessibilityAddTraits(.isHeader)
                    if let detail {
                        Text(detail)
                            .typeRole(Tokens.TypeRamp.caption)
                            .foregroundStyle(Tokens.Palette.mutedText)
                            .monospacedDigit()
                    }
                }
            }
            content()
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .modifier(ReadingFocus(enabled: focusable))
    }
}

private struct ReadingFocus: ViewModifier {
    let enabled: Bool

    #if os(tvOS)
        @FocusState private var focused: Bool

        func body(content: Content) -> some View {
            if enabled {
                content
                    .padding(Tokens.Spacing.xl)
                    .background(
                        Tokens.Palette.surface.opacity(focused ? 1 : 0.5),
                        in: RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous)
                    )
                    .overlay {
                        RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous)
                            .strokeBorder(
                                focused ? Tokens.Palette.text : Tokens.Palette.edgeLine, lineWidth: focused ? 3 : 1)
                    }
                    .focusable()
                    .focused($focused)
                    .animation(Tokens.Motion.snappy, value: focused)
            } else {
                content
            }
        }
    #else
        func body(content: Content) -> some View {
            content
        }
    #endif
}

/// How far into the tier, as a bar in the tier's colour.
struct XPBar: View {
    let fraction: Double
    let color: Color

    var body: some View {
        GeometryReader { geometry in
            ZStack(alignment: .leading) {
                Capsule().fill(Tokens.Palette.surface2)
                Capsule().fill(color)
                    .frame(width: geometry.size.width * min(max(fraction, 0), 1))
            }
        }
        .frame(height: Idiom.isTV ? 12 : 8)
        .accessibilityHidden(true)
    }
}

/// An achievement's medal: the metal's gradient behind the achievement's symbol once unlocked,
/// a dim disc ringed with the progress so far while locked.
struct MedalBadge: View {
    let card: AchievementCard
    let size: CGFloat

    @Environment(\.accent) private var accent

    var body: some View {
        let medal = RanksStyle.medal(card.tier)
        ZStack {
            if card.unlocked {
                Circle()
                    .fill(
                        LinearGradient(
                            colors: [medal.from, medal.to], startPoint: .topLeading, endPoint: .bottomTrailing)
                    )
                    .overlay { Circle().strokeBorder(medal.ring, lineWidth: size * 0.04) }
                    .shadow(color: medal.glow, radius: size * 0.2)
            } else {
                Circle().fill(Tokens.Palette.surface2)
                Circle()
                    .trim(from: 0, to: Double(min(card.percent, 100)) / 100)
                    .stroke(accent.color, style: StrokeStyle(lineWidth: size * 0.06, lineCap: .round))
                    .rotationEffect(.degrees(-90))
                    .padding(size * 0.03)
            }
            Image(systemName: RanksWords.achievementSymbol(card.code))
                .font(.system(size: size * 0.42, weight: .semibold))
                .foregroundStyle(card.unlocked ? Tokens.Accent.onAccentDark : Tokens.Palette.faintText)
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}

/// One number of a profile with what it counts.
struct StatTile: View {
    let systemImage: String
    let value: String
    let label: String

    @Environment(\.accent) private var accent

    var body: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
            Image(systemName: systemImage)
                .foregroundStyle(accent.ink)
                .accessibilityHidden(true)
            Text(value)
                .typeRole(Tokens.TypeRamp.section)
                .foregroundStyle(Tokens.Palette.text)
                .monospacedDigit()
                .lineLimit(2)
                .minimumScaleFactor(0.7)
            Text(label)
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.mutedText)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(Idiom.isTV ? Tokens.Spacing.xl : Tokens.Spacing.md)
        .frame(maxWidth: .infinity, minHeight: Idiom.isTV ? 180 : 96, alignment: .topLeading)
        .background(Tokens.Palette.surface, in: RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
        .accessibilityElement(children: .combine)
    }
}

/// The rank as a line: a small ring of the progress in the tier's colour, then the tier and level.
/// Shown under the profile in the sidebar's header.
struct RankCaption: View {
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        if core.session.features.rankings, let badge = core.rank.rank {
            let color = RanksStyle.tier(badge.tier.code)
            HStack(spacing: Tokens.Spacing.xs) {
                RankRing(
                    color: color, progress: Double(badge.percent) / 100, lineWidth: 2, flashes: core.rank.levelUps
                ) {
                    Color.clear.frame(width: 6, height: 6)
                }
                Text(RanksWords.rankLine(badge))
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.mutedText)
                    .lineLimit(1)
            }
            .accessibilityElement(children: .combine)
        }
    }
}
