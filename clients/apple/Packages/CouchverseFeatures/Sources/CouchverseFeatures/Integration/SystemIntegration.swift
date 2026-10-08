import CouchverseCore
import SwiftUI

#if os(iOS)
    import CoreSpotlight
#endif

extension View {
    /// Keeps what the app leaves outside itself in step with the core (plan 10.9): the shelf
    /// snapshot the widget, Top Shelf and the intents read, and on iPhone and iPad Spotlight, whose
    /// results open their title. Only in the running app (`live`): previews and snapshots leave the
    /// system alone.
    func systemIntegration(live: Bool) -> some View {
        modifier(SystemIntegration(live: live))
    }

    /// Runs `action` whenever the titles the shelf names change, e.g. to update the parameters of
    /// the App Shortcuts that offer them.
    public func onShelfChange(_ action: @escaping () -> Void) -> some View {
        environment(\.shelfChanged, RootAction(name: "shelf-changed", perform: action))
    }
}

extension EnvironmentValues {
    @Entry var shelfChanged = RootAction(name: "none") {}
}

private struct SystemIntegration: ViewModifier {
    let live: Bool
    @Environment(CoreRuntime.self) private var core
    @Environment(OpenRequests.self) private var requests: OpenRequests?
    @Environment(\.shelfChanged) private var shelfChanged
    /// Made on first use: reading the snapshot is no work for every time the root is rebuilt.
    @State private var shelf: Shelf?
    #if os(iOS)
        @State private var spotlight = SpotlightIndex()
    #endif

    func body(content: Content) -> some View {
        content
            .onChange(of: ShelfInputs(core), initial: true) {
                guard live else { return }
                let shelf = shelf ?? Shelf()
                self.shelf = shelf
                guard let snapshot = shelf.update(core) else { return }
                #if os(iOS)
                    spotlight.update(snapshot)
                #endif
                shelfChanged()
            }
            #if os(iOS)
                .task(id: listedAccount) {
                    // Spotlight and the intents offer My List, which the core loads only while
                    // something shows it
                    guard listedAccount != nil else { return }
                    core.send(.screenOpened(.myList))
                    try? await Task.sleep(for: .seconds(60 * 60 * 24 * 365))
                    core.send(.screenClosed(.myList))
                }
                .onContinueUserActivity(CSSearchableItemActionType) { activity in
                    if let link = activity.userInfo?[CSSearchableItemActivityIdentifier] as? String,
                        let request = OpenRequest(link: link)
                    {
                        requests?.open(request)
                    }
                }
            #endif
    }

    /// The signed-in account, while the app runs.
    private var listedAccount: String? {
        live && core.app.phase == .ready ? core.app.activeAccount : nil
    }
}
