import CouchverseCore
import CouchverseDesign
import SwiftUI

/// A profile's achievements by category, the unlocked ones first in each: their medal, name and
/// what they ask for, then when they were earned or how far along they are. Categories the
/// server's flags leave out (the couch's, with the couch off) are absent rather than locked.
struct AchievementsSection: View {
    let cards: [AchievementCard]
    let won: UInt32

    var body: some View {
        VStack(alignment: .leading, spacing: Idiom.isTV ? Tokens.Spacing.xxl : Tokens.Spacing.xl) {
            HStack(alignment: .firstTextBaseline, spacing: Tokens.Spacing.md) {
                Text(L10n.achievementHeading)
                    .typeRole(Tokens.TypeRamp.section)
                    .foregroundStyle(Tokens.Palette.text)
                    .accessibilityAddTraits(.isHeader)
                Text(L10n.achievementCount(unlocked: String(won), total: String(cards.count)))
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.mutedText)
                    .monospacedDigit()
            }
            if cards.isEmpty {
                CatalogMessage(
                    systemImage: "trophy", title: L10n.achievementEmptyTitle, message: L10n.achievementEmptyMessage)
            }
            ForEach(Self.groups(cards), id: \.category) { group in
                VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                    Text(RanksWords.category(group.category))
                        .typeRole(Tokens.TypeRamp.card)
                        .foregroundStyle(Tokens.Palette.mutedText)
                        .accessibilityAddTraits(.isHeader)
                    LazyVGrid(
                        columns: [GridItem(.adaptive(minimum: tileWidth), spacing: Tokens.Spacing.md)],
                        alignment: .leading, spacing: Tokens.Spacing.md
                    ) {
                        ForEach(group.cards, id: \.code) { card in
                            AchievementTile(card: card)
                        }
                    }
                }
                .tvFocusSection()
            }
        }
    }

    private var tileWidth: CGFloat { Idiom.isTV ? 340 : 160 }

    struct CategoryGroup: Equatable {
        let category: String
        let cards: [AchievementCard]
    }

    /// The cards by category in the server's order of categories (one it adds later goes last),
    /// the unlocked first within each and otherwise as the server lists them.
    static func groups(_ cards: [AchievementCard]) -> [CategoryGroup] {
        let known = RanksWords.categories
        let extra = cards.map(\.category).reduce(into: [String]()) { seen, category in
            if !known.contains(category) && !seen.contains(category) {
                seen.append(category)
            }
        }
        return (known + extra).compactMap { category in
            let inCategory = cards.filter { $0.category == category }
            guard !inCategory.isEmpty else { return nil }
            return CategoryGroup(
                category: category, cards: inCategory.filter(\.unlocked) + inCategory.filter { !$0.unlocked })
        }
    }
}

private struct AchievementTile: View {
    let card: AchievementCard

    #if os(tvOS)
        @FocusState private var focused: Bool
    #endif

    var body: some View {
        let medal = RanksStyle.medal(card.tier)
        VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
            HStack(alignment: .top, spacing: Tokens.Spacing.sm) {
                MedalBadge(card: card, size: Idiom.isTV ? 72 : 44)
                Spacer(minLength: 0)
                Text(RanksWords.medal(card.tier))
                    .typeRole(Tokens.TypeRamp.eyebrow)
                    .foregroundStyle(card.unlocked ? medal.ring : Tokens.Palette.faintText)
            }
            Text(RanksWords.achievementName(card.code))
                .typeRole(Tokens.TypeRamp.card)
                .foregroundStyle(card.unlocked ? Tokens.Palette.text : Tokens.Palette.mutedText)
                .fixedSize(horizontal: false, vertical: true)
            if let description = RanksWords.achievementDescription(card.code) {
                Text(description)
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.mutedText)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer(minLength: 0)
            footer
        }
        .padding(Idiom.isTV ? Tokens.Spacing.xl : Tokens.Spacing.md)
        .frame(maxWidth: .infinity, minHeight: Idiom.isTV ? 300 : 170, alignment: .topLeading)
        .background(Tokens.Palette.surface, in: RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous)
                .strokeBorder(card.unlocked ? medal.ring.opacity(0.6) : Tokens.Palette.edgeLine)
        }
        .accessibilityElement(children: .combine)
        .accessibilityValue(card.unlocked ? L10n.achievementUnlocked : L10n.achievementLocked)
        #if os(tvOS)
            .focusable()
            .focused($focused)
            .focusLift(focused, scale: 1.05)
        #endif
    }

    @ViewBuilder private var footer: some View {
        if card.unlocked {
            Text(L10n.achievementReward(xp: RanksWords.number(card.xp)))
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.text)
            if let date = card.unlockedAt.flatMap(RanksWords.date) {
                Text(L10n.achievementUnlockedOn(date: date))
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.mutedText)
            }
        } else {
            VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
                ProgressBar(fraction: Double(card.percent) / 100)
                Text(
                    L10n.achievementProgress(
                        current: RanksWords.number(min(card.value, card.target)), target: RanksWords.number(card.target)
                    )
                )
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.mutedText)
                .monospacedDigit()
            }
        }
    }
}
