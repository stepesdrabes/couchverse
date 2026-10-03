import CouchverseCore
import CouchverseDesign
import SwiftUI

extension AccountCard {
    /// The profile's own colour: its identicon hue, the same on every client. It tints the
    /// picker's backdrop and ring until profiles carry a banner accent and a rank tier.
    var tint: Color { Identicon(seed: username).color }
}

/// "Who's watching?": the profiles as large glass tiles over a backdrop tinted by the focused one.
/// A plain view of the accounts it is given, so the choreography can redraw it as it dissolves.
struct ProfilePicker: View {
    let accounts: [AccountCard]
    let active: String?
    /// The profile being flown into the header: hidden here, the rest dissolve away from it.
    var chosen: String?
    var dissolved = false
    let onSelect: (AccountCard, CGRect) -> Void
    let onAdd: () -> Void
    var onSignOut: ((AccountCard) -> Void)?

    @FocusState private var focused: String?
    @Environment(\.horizontalSizeClass) private var sizeClass

    private var tintAccount: AccountCard? {
        let id = focused ?? chosen ?? active
        return accounts.first { $0.id == id } ?? accounts.first
    }

    var body: some View {
        ZStack {
            GlowBackdrop(tint: tintAccount?.tint ?? Tokens.Palette.accent, intensity: 1.2)
            ScrollView {
                VStack(spacing: Idiom.isTV ? Tokens.Spacing.xxxl * 1.5 : Tokens.Spacing.xxl) {
                    Text(L10n.accountsWhosWatching)
                        .typeRole(Tokens.TypeRamp.hero)
                        .foregroundStyle(Tokens.Palette.text)
                        .multilineTextAlignment(.center)
                        .accessibilityAddTraits(.isHeader)
                        .opacity(dissolved ? 0 : 1)
                        .offset(y: dissolved ? -40 : 0)
                    tiles
                }
                .padding(.vertical, Idiom.isTV ? 120 : Tokens.Spacing.xxxl)
                .padding(.horizontal, Tokens.Spacing.xl)
                .frame(maxWidth: .infinity)
                .containerRelativeFrame(.vertical, alignment: .center) { length, _ in length }
            }
            .scrollBounceBehavior(.basedOnSize)
        }
        .defaultFocus($focused, active ?? accounts.first?.id)
    }

    @ViewBuilder private var tiles: some View {
        let chosenIndex = accounts.firstIndex { $0.id == chosen }
        let layout =
            Idiom.isTV
            ? AnyLayout(HStackLayout(alignment: .top, spacing: 72))
            : AnyLayout(GridFlowLayout(columns: sizeClass == .regular ? 4 : 2, spacing: Tokens.Spacing.xl))
        layout {
            ForEach(Array(accounts.enumerated()), id: \.element.id) { index, card in
                ProfileTile(card: card, diameter: diameter) { frame in
                    onSelect(card, frame)
                }
                .focused($focused, equals: card.id)
                .contextMenu {
                    if let onSignOut {
                        Button(role: .destructive) {
                            onSignOut(card)
                        } label: {
                            Label(L10n.navSignOut, systemImage: "rectangle.portrait.and.arrow.right")
                        }
                    }
                }
                .opacity(card.id == chosen ? 0 : 1)
                .modifier(Dissolve(offset: chosenIndex.map { index - $0 } ?? 0, active: dissolved))
            }
            AddProfileTile(diameter: diameter, action: onAdd)
                .modifier(Dissolve(offset: chosenIndex.map { accounts.count - $0 } ?? 1, active: dissolved))
        }
    }

    private var diameter: CGFloat {
        if Idiom.isTV { return 240 }
        return sizeClass == .regular ? 150 : 116
    }
}

/// Moves a profile away from the chosen one, growing and fading as if blown outward.
private struct Dissolve: ViewModifier {
    let offset: Int
    let active: Bool

    func body(content: Content) -> some View {
        let direction = CGFloat(offset.signum())
        content
            .offset(x: active ? direction * (160 + CGFloat(abs(offset)) * 60) : 0)
            .scaleEffect(active ? 1.25 : 1)
            .opacity(active ? 0 : 1)
    }
}

/// Rows of `columns` equal cells, centred: the profile grid on phones and iPads.
private struct GridFlowLayout: Layout {
    let columns: Int
    let spacing: CGFloat

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let cell = cellSize(subviews)
        let rows = (subviews.count + columns - 1) / max(columns, 1)
        let perRow = min(columns, subviews.count)
        return CGSize(
            width: CGFloat(perRow) * cell.width + CGFloat(max(perRow - 1, 0)) * spacing,
            height: CGFloat(rows) * cell.height + CGFloat(max(rows - 1, 0)) * spacing)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let cell = cellSize(subviews)
        for (index, subview) in subviews.enumerated() {
            let row = index / columns
            let inRow = min(columns, subviews.count - row * columns)
            let rowWidth = CGFloat(inRow) * cell.width + CGFloat(inRow - 1) * spacing
            let x = bounds.midX - rowWidth / 2 + CGFloat(index % columns) * (cell.width + spacing)
            let y = bounds.minY + CGFloat(row) * (cell.height + spacing)
            subview.place(
                at: CGPoint(x: x, y: y), anchor: .topLeading,
                proposal: ProposedViewSize(width: cell.width, height: cell.height))
        }
    }

    private func cellSize(_ subviews: Subviews) -> CGSize {
        subviews.reduce(.zero) { size, subview in
            let fit = subview.sizeThatFits(.unspecified)
            return CGSize(width: max(size.width, fit.width), height: max(size.height, fit.height))
        }
    }
}

/// One profile: a glass disc holding the avatar inside its ring, and the name below. Focus lifts
/// it, brightens the ring and reveals which server it is on.
struct ProfileTile: View {
    let card: AccountCard
    let diameter: CGFloat
    let action: (CGRect) -> Void
    @State private var avatarFrame: CGRect = .zero

    var body: some View {
        Button {
            action(avatarFrame)
        } label: {
            ProfileTileLabel(card: card, diameter: diameter, avatarFrame: $avatarFrame)
        }
        .buttonStyle(ProfileTileStyle())
        .accessibilityLabel("\(card.displayName), \(card.serverName)")
        .accessibilityValue(card.signedIn ? "" : L10n.accountsSignedOut)
        .accessibilityIdentifier("profile-\(card.username)")
    }
}

private struct ProfileTileStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? 0.95 : 1)
            .animation(Tokens.Motion.snappy, value: configuration.isPressed)
    }
}

private struct ProfileTileLabel: View {
    let card: AccountCard
    let diameter: CGFloat
    @Binding var avatarFrame: CGRect
    @Environment(\.isFocused) private var focused
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(spacing: Tokens.Spacing.lg) {
            ZStack {
                Circle()
                    .fill(.clear)
                    .glassEffect(.regular.tint(card.tint.opacity(focused ? 0.35 : 0.15)), in: Circle())
                AvatarView(url: card.avatarUrl, seed: card.username, name: card.displayName)
                    .onGeometryChange(for: CGRect.self) {
                        $0.frame(in: .global)
                    } action: {
                        avatarFrame = $0
                    }
                    .padding(diameter * 0.07)
                    .saturation(card.signedIn ? 1 : 0)
                    .opacity(card.signedIn ? 1 : 0.6)
                Circle()
                    .strokeBorder(card.tint.opacity(focused ? 1 : 0.55), lineWidth: focused ? 6 : 4)
                    .shadow(color: card.tint.opacity(focused ? 0.8 : 0), radius: 18)
                if !card.signedIn {
                    Image(systemName: "lock.fill")
                        .font(.system(size: diameter * 0.16, weight: .semibold))
                        .foregroundStyle(Tokens.Palette.text)
                        .padding(diameter * 0.06)
                        .background(Tokens.Palette.surface2, in: Circle())
                        .offset(x: diameter * 0.34, y: diameter * 0.34)
                }
            }
            .frame(width: diameter, height: diameter)
            .scaleEffect(focused && !reduceMotion ? 1.12 : 1)
            .shadow(color: .black.opacity(focused ? 0.5 : 0.2), radius: focused ? 30 : 10, y: focused ? 18 : 6)

            VStack(spacing: Tokens.Spacing.xs) {
                Text(card.displayName)
                    .typeRole(Tokens.TypeRamp.card)
                    .foregroundStyle(focused || !Idiom.isTV ? Tokens.Palette.text : Tokens.Palette.muted)
                    .lineLimit(1)
                Group {
                    if card.signedIn {
                        Text(card.serverName)
                    } else {
                        Text(L10n.accountsSignedOut)
                    }
                }
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.muted)
                .lineLimit(1)
                .opacity(focused || !Idiom.isTV ? 1 : 0)
            }
            .frame(width: diameter * 1.3)
        }
        .animation(reduceMotion ? nil : Tokens.Motion.bouncy, value: focused)
    }
}

/// The "+" tile: sign in to another account on any server.
private struct AddProfileTile: View {
    let diameter: CGFloat
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            AddProfileLabel(diameter: diameter)
        }
        .buttonStyle(ProfileTileStyle())
        .accessibilityLabel(L10n.accountsAddAccount)
        .accessibilityIdentifier("add-profile")
    }
}

private struct AddProfileLabel: View {
    let diameter: CGFloat
    @Environment(\.isFocused) private var focused
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(spacing: Tokens.Spacing.lg) {
            Image(systemName: "plus")
                .font(.system(size: diameter * 0.3, weight: .light))
                .foregroundStyle(focused ? Tokens.Palette.text : Tokens.Palette.muted)
                .frame(width: diameter, height: diameter)
                .glassEffect(.regular, in: Circle())
                .scaleEffect(focused && !reduceMotion ? 1.12 : 1)
            Text(L10n.accountsAddAccount)
                .typeRole(Tokens.TypeRamp.card)
                .foregroundStyle(focused || !Idiom.isTV ? Tokens.Palette.text : Tokens.Palette.muted)
                .lineLimit(1)
                .frame(width: diameter * 1.3)
            Text(" ").typeRole(Tokens.TypeRamp.caption).accessibilityHidden(true)
        }
        .animation(reduceMotion ? nil : Tokens.Motion.bouncy, value: focused)
    }
}
