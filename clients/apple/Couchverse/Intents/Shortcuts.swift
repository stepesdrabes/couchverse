import AppIntents

/// The phrases Siri and Spotlight offer without any setup; their Czech forms are in
/// `AppShortcuts.xcstrings`.
nonisolated struct CouchverseShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(
            intent: ContinueWatchingIntent(),
            phrases: [
                "Continue watching in \(.applicationName)",
                "Resume \(\.$item) in \(.applicationName)",
            ],
            shortTitle: LocalizedStringResource("intent_continue_watching"),
            systemImageName: "play.circle")
        AppShortcut(
            intent: OpenTitleIntent(),
            phrases: [
                "Open \(\.$title) in \(.applicationName)",
                "Open a title in \(.applicationName)",
            ],
            shortTitle: LocalizedStringResource("intent_open_title"),
            systemImageName: "film")
        AppShortcut(
            intent: OpenMyListIntent(),
            phrases: [
                "Open my list in \(.applicationName)",
                "Show my list in \(.applicationName)",
            ],
            shortTitle: LocalizedStringResource("intent_my_list"),
            systemImageName: "bookmark")
        AppShortcut(
            intent: JoinCouchIntent(),
            phrases: [
                "Join a couch session in \(.applicationName)",
                "Join a couch in \(.applicationName)",
            ],
            shortTitle: LocalizedStringResource("intent_join_couch_short"),
            systemImageName: "sofa")
    }
}
