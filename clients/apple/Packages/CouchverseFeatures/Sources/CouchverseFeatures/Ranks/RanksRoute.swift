import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Where a profile or leaderboard link leads, pushed onto the tab's navigation stack.
enum RanksRoute: Hashable {
    case profile(username: String)
    case leaderboard
    case editor
}

extension View {
    func ranksDestinations() -> some View {
        navigationDestination(for: RanksRoute.self) { route in
            switch route {
            case .profile(let username): ProfileScreen(username: username)
            case .leaderboard: LeaderboardScreen()
            case .editor: ProfileEditorScreen()
            }
        }
    }

    /// Profiles and leaderboards are absent, not locked, while the server has rankings off: a
    /// pushed one goes back when they are switched off.
    func closesWithoutRankings() -> some View {
        modifier(ClosesWithoutRankings())
    }
}

private struct ClosesWithoutRankings: ViewModifier {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.dismiss) private var dismiss

    func body(content: Content) -> some View {
        content.onChange(of: core.session.features.rankings) { _, on in
            if !on {
                dismiss()
            }
        }
    }
}

/// The viewer's own corner of Settings: their public profile with the rank beside it and the
/// leaderboard while rankings are on, and editing the profile always (a name, a picture and a
/// password are not progression). A TV has the first two in its sidebar instead.
struct ProfileSettingsSection: View {
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        if let me = core.session.user {
            Section {
                if core.session.features.rankings && !Idiom.isTV {
                    NavigationLink(value: RanksRoute.profile(username: me.username)) {
                        PublicProfileRow(user: me)
                    }
                    .accessibilityIdentifier("public-profile")
                    NavigationLink(value: RanksRoute.leaderboard) {
                        Label(L10n.navLeaderboard, systemImage: "trophy")
                    }
                }
                NavigationLink(value: RanksRoute.editor) {
                    Label(L10n.profilesEditProfile, systemImage: "pencil")
                }
                .accessibilityIdentifier("edit-profile")
            }
        }
    }
}

/// The profile row: the avatar in the rank ring, and the rank in words beside it.
private struct PublicProfileRow: View {
    let user: SessionUser
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        let card = core.accounts.accounts.first { $0.id == core.app.activeAccount }
        let badge = core.rank.rank
        HStack(spacing: Tokens.Spacing.md) {
            RankRing(
                color: badge.map { RanksStyle.tier($0.tier.code) } ?? Tokens.Palette.edge,
                progress: Double(badge?.percent ?? 0) / 100, lineWidth: 3, flashes: core.rank.levelUps
            ) {
                AvatarView(url: card?.avatarUrl, seed: user.username, name: user.displayName)
                    .frame(width: 36, height: 36)
            }
            VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) {
                Text(L10n.navPublicProfile)
                    .typeRole(Tokens.TypeRamp.card)
                    .foregroundStyle(Tokens.Palette.text)
                if let badge {
                    Text(RanksWords.rankLine(badge))
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.muted)
                }
            }
        }
        .accessibilityElement(children: .combine)
    }
}
