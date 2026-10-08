import CouchverseCore
import CouchverseDesign
import SwiftUI
import Testing

@testable import CouchverseFeatures

/// Profiles, leaderboards, the profile editor and an unlock celebration, in every state, on the
/// same variants as the other screens (references in `__Snapshots__/RanksSnapshots`).
extension ScreenSnapshots {
    static var ranked: [SurfaceValue] { ready + [.rank(Fixtures.rank())] }

    @Test(arguments: ["own", "private", "other", "newcomer"])
    func profile(kind: String) {
        let detail =
            switch kind {
            case "private": Fixtures.profileDetail(public: false)
            case "other": Fixtures.profileDetail("vera", displayName: "Vera", isSelf: false)
            case "newcomer": Fixtures.newcomer
            default: Fixtures.profileDetail()
            }
        let view = Fixtures.profile(.loaded, detail: detail)
        snapshot("profile-\(kind)", Self.ranked + [.profile(detail.username, view)]) {
            NavigationStack { ProfileScreen(username: detail.username) }
        }
    }

    @Test(arguments: [LoadStatus.loading, .notFound, .failed, .stale])
    func profileStates(status: LoadStatus) {
        snapshot("profile-\(status)", Self.ranked + [.profile("nora", Fixtures.profile(status))]) {
            NavigationStack { ProfileScreen(username: "nora") }
        }
    }

    @Test(arguments: ["xp", "watch-week", "hidden", "empty", "nothing-yet", "loading", "failed"])
    func leaderboard(state: String) {
        let metric: Metric = state == "watch-week" ? .watch : .xp
        let period: Period = state == "watch-week" ? .week : .all
        let view =
            switch state {
            case "hidden": Fixtures.leaderboard(.loaded, hidden: true)
            case "empty": Fixtures.leaderboard(.loaded, empty: true)
            case "nothing-yet": Fixtures.leaderboard(.loaded, allZero: true)
            case "loading": Fixtures.leaderboard(.loading)
            case "failed": Fixtures.leaderboard(.failed)
            default: Fixtures.leaderboard(.loaded, metric: metric, period: period)
            }
        snapshot("leaderboard-\(state)", Self.ranked + [.leaderboard(view.key, view)]) {
            NavigationStack { LeaderboardScreen(metric: metric, period: period) }
        }
    }

    @Test(arguments: ["idle", "saved", "uploading"])
    func profileEditor(state: String) {
        let (editor, showing): (ProfileEditorView, Set<ProfileEditorScreen.Part>) =
            switch state {
            case "saved":
                (
                    Fixtures.profileEditor(details: .loaded, password: .failed, passwordProblem: "invalid_password"),
                    [.details, .password]
                )
            case "uploading": (Fixtures.profileEditor(avatar: .loading), [.avatar])
            default: (Fixtures.profileEditor(), [])
            }
        let own = Fixtures.profile(.loaded)
        snapshot("profile-editor-\(state)", Self.ranked + [.profileEditor(editor), .profile("nora", own)]) {
            NavigationStack { ProfileEditorScreen(showing: showing) }
        }
    }

    @Test func celebration() {
        let values = Self.ready + [.home(Fixtures.home(.loaded)), .rank(Fixtures.rank(celebrating: "genres_10"))]
        snapshot("celebration", values) {
            NavigationStack { HomeScreen() }
                .overlay { CelebrationOverlay() }
        }
    }

    /// Settings with the rank beside your profile, on iPhone and iPad: a TV has it in its sidebar.
    @Test func settingsRanked() {
        let touch = Self.variants.filter { !$0.name.hasPrefix("tv") }
        snapshot("settings-ranked", Self.ranked, variants: touch) { NavigationStack { SettingsScreen() } }
    }
}
