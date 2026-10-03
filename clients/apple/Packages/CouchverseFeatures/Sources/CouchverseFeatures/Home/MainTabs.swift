import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The signed-in app. Home is a placeholder until the catalog slice; the navigation is already the
/// final shape: a sidebar on TV and iPad, a Liquid Glass tab bar on iPhone (plan 10.3).
struct MainTabs: View {
    enum Destination: Hashable {
        case home
        case settings
    }

    @State private var selection = Destination.home

    var body: some View {
        TabView(selection: $selection) {
            Tab(L10n.navHome, systemImage: "house", value: Destination.home) {
                NavigationStack { HomeScreen() }
            }
            Tab(L10n.navSettings, systemImage: "gearshape", value: Destination.settings) {
                NavigationStack { SettingsScreen() }
            }
        }
        .tabViewStyle(.sidebarAdaptable)
        .tabViewSidebarHeader { AccountHeader() }
    }
}

/// Who is signed in, at the top of the sidebar: the place a chosen profile's avatar lands, and
/// the way to switch profiles.
struct AccountHeader: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(ProfileChoreography.self) private var choreography
    @Environment(\.showProfilePicker) private var showProfilePicker
    @Environment(\.showAccountSwitcher) private var showAccountSwitcher

    private var card: AccountCard? {
        core.accounts.accounts.first { $0.id == core.app.activeAccount }
    }

    var body: some View {
        Button {
            if Idiom.isTV {
                showProfilePicker()
            } else {
                showAccountSwitcher()
            }
        } label: {
            HStack(spacing: Tokens.Spacing.md) {
                ProfileAvatar(card: card, size: Idiom.isTV ? 72 : 40)
                VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) {
                    Text(card?.displayName ?? "")
                        .typeRole(Tokens.TypeRamp.card)
                        .lineLimit(1)
                    Text(L10n.accountsSwitch)
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.muted)
                        .lineLimit(1)
                }
            }
        }
        .buttonStyle(.plain)
        .accessibilityLabel(L10n.accountsSwitch)
        .accessibilityValue(card?.displayName ?? "")
        .accessibilityIdentifier("account-header")
    }
}

/// The active profile's avatar in the sidebar header: where a chosen profile's avatar lands. It
/// tells the choreography where it sits and stays hidden until the flying avatar arrives.
struct ProfileAvatar: View {
    let card: AccountCard?
    let size: CGFloat
    @Environment(ProfileChoreography.self) private var choreography
    @State private var frame: CGRect = .zero

    var body: some View {
        AvatarView(url: card?.avatarUrl, seed: card?.username ?? "", name: card?.displayName ?? "")
            .frame(width: size, height: size)
            .overlay { Circle().strokeBorder(card?.tint ?? .clear, lineWidth: 2) }
            .opacity(choreography.isFlying(card?.id) ? 0 : 1)
            .onGeometryChange(for: CGRect.self) {
                $0.frame(in: .global)
            } action: { frame in
                self.frame = frame
                report()
            }
            .onChange(of: choreography.card?.id) { report() }
    }

    private func report() {
        if choreography.isRunning, choreography.card?.id == card?.id, frame != .zero {
            choreography.target = frame
        }
    }
}
