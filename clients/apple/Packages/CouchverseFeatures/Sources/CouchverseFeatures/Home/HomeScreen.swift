import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Home until the catalog slice lands: who is signed in, on which server, in which accent.
struct HomeScreen: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.accent) private var accent
    @Environment(\.showAccountSwitcher) private var showAccountSwitcher
    @Environment(\.showProfilePicker) private var showProfilePicker

    private var session: SessionView { core.session }
    private var card: AccountCard? { core.accounts.accounts.first { $0.id == core.app.activeAccount } }
    private var server: Server? { core.servers.servers.first { $0.id == card?.serverId } }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Idiom.isTV ? Tokens.Spacing.xxxl : Tokens.Spacing.xl) {
                hero
                if let problem = session.problem, session.status == .stale || session.status == .failed {
                    if session.user == nil {
                        ProblemBanner(problem)
                    } else {
                        Label(L10n.homeOffline(server: server?.name ?? ""), systemImage: "wifi.slash")
                            .typeRole(Tokens.TypeRamp.caption)
                            .foregroundStyle(Tokens.Palette.muted)
                    }
                }
                Text(L10n.homePlaceholderMessage)
                    .typeRole(Tokens.TypeRamp.body)
                    .foregroundStyle(Tokens.Palette.muted)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: 640, alignment: .leading)
                AccentSwatches(palette: session.accent)
                Button {
                    if Idiom.isTV {
                        showProfilePicker()
                    } else {
                        showAccountSwitcher()
                    }
                } label: {
                    ActionLabel(L10n.accountsSwitch, systemImage: "person.2.fill")
                }
                .secondaryAction()
                .fixedSize()
            }
            .padding(Tokens.Spacing.xl)
            // clear of the TV sidebar, which floats over the leading edge while it has focus
            .padding(.leading, Idiom.isTV ? 400 : 0)
            .padding(.vertical, Idiom.isTV ? 40 : 0)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .scrollBounceBehavior(.basedOnSize)
        .background { GlowBackdrop(tint: accent.color) }
        #if os(iOS)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button {
                        showAccountSwitcher()
                    } label: {
                        AvatarView(url: card?.avatarUrl, seed: card?.username ?? "", name: card?.displayName ?? "")
                        .frame(width: 32, height: 32)
                    }
                    .accessibilityLabel(L10n.accountsSwitch)
                    .accessibilityIdentifier("account-switcher")
                }
            }
        #endif
    }

    /// Side by side where there is room (TV, iPad), stacked on a phone or at large text sizes.
    @ViewBuilder private var hero: some View {
        let size: CGFloat = Idiom.isTV ? 160 : 88
        ViewThatFits(in: .horizontal) {
            HStack(alignment: .center, spacing: Tokens.Spacing.xl) {
                heroAvatar(size)
                heroText
            }
            VStack(alignment: .leading, spacing: Tokens.Spacing.lg) {
                heroAvatar(size)
                heroText
            }
        }
    }

    @ViewBuilder private func heroAvatar(_ size: CGFloat) -> some View {
        if let user = session.user {
            AvatarView(url: card?.avatarUrl, seed: user.username, name: user.displayName)
                .frame(width: size, height: size)
                .overlay { Circle().strokeBorder(accent.color, lineWidth: 3) }
        } else {
            Skeleton(circle: size)
        }
    }

    @ViewBuilder private var serverLine: some View {
        Text(server?.name ?? card?.serverName ?? "")
            .typeRole(Tokens.TypeRamp.eyebrow)
            .textCase(.uppercase)
            .foregroundStyle(accent.color)
        if server?.insecure == true {
            InsecureBadge()
        }
    }

    @ViewBuilder private var heroText: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
            if let user = session.user {
                ViewThatFits(in: .horizontal) {
                    HStack(spacing: Tokens.Spacing.md) { serverLine }
                    VStack(alignment: .leading, spacing: Tokens.Spacing.xs) { serverLine }
                }
                Text(L10n.homeWelcomeBack(name: user.displayName))
                    .typeRole(Tokens.TypeRamp.hero)
                    .foregroundStyle(Tokens.Palette.text)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityAddTraits(.isHeader)
            } else {
                Skeleton(width: 120, height: 14)
                Skeleton(width: Idiom.isTV ? 640 : 220, height: Idiom.isTV ? 64 : 34)
            }
        }
    }
}

/// The server's accent as the core derived it: the colour, its strong shade and its soft tint.
private struct AccentSwatches: View {
    let palette: AccentPalette

    var body: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
            Text(L10n.homeServerAccent)
                .typeRole(Tokens.TypeRamp.eyebrow)
                .textCase(.uppercase)
                .foregroundStyle(Tokens.Palette.faint)
            HStack(spacing: Tokens.Spacing.md) {
                swatch(palette.accent, text: palette.onAccent)
                swatch(palette.strong, text: palette.onAccent)
                swatch(palette.soft, text: "#f2f4f8")
            }
        }
        .accessibilityElement(children: .combine)
    }

    private func swatch(_ hex: String, text: String) -> some View {
        Text(hex.uppercased())
            .font(.system(.caption, design: .monospaced, weight: .semibold))
            .foregroundStyle(Color(hex: text) ?? .white)
            .padding(.horizontal, Tokens.Spacing.md)
            .padding(.vertical, Tokens.Spacing.sm)
            .background(Color(hex: hex) ?? .clear, in: Capsule())
            .overlay { Capsule().strokeBorder(Tokens.Palette.edge) }
    }
}
