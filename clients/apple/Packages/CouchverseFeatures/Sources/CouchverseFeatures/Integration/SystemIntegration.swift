import CouchverseCore
import SwiftUI

extension View {
    /// Keeps what the app leaves outside itself in step with the core (plan 10.9): the shelf
    /// snapshot the widget, Top Shelf and the intents read. Only in the running app (`live`):
    /// previews and snapshots leave the system alone.
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
    @Environment(\.shelfChanged) private var shelfChanged
    /// Made on first use: reading the snapshot is no work for every time the root is rebuilt.
    @State private var shelf: Shelf?

    func body(content: Content) -> some View {
        content
            .onChange(of: ShelfInputs(core), initial: true) {
                guard live else { return }
                let shelf = shelf ?? Shelf()
                self.shelf = shelf
                if shelf.update(core) != nil {
                    shelfChanged()
                }
            }
    }
}
