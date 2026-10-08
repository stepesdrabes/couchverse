import CouchverseCore
import CouchverseDesign
import CouchverseShared
import Foundation
import os

#if os(iOS)
    import WidgetKit
#else
    import TVServices
#endif

private let log = Logger(subsystem: "io.stepes.couchverse", category: "shelf")

/// Keeps the shelf snapshot in step with the core (FEATURES.md, Apple clients): read once at
/// launch, written again whenever what it holds changes, after which the widget or Top Shelf
/// reloads and the change goes on to `changed` (Spotlight, the intents' shortcuts).
@MainActor
final class Shelf {
    private(set) var snapshot: ShelfSnapshot?
    private let directory: URL?
    private let extensions: Bool

    init(directory: URL? = SharedContainer.directory(), extensions: Bool = SharedContainer.extensionsEnabled()) {
        self.directory = directory
        self.extensions = extensions
        snapshot = directory.flatMap(ShelfSnapshot.read(from:))
    }

    /// Takes in the core's views; returns the snapshot when it changed.
    func update(_ core: CoreRuntime) -> ShelfSnapshot? {
        let session = core.session
        // the words are the session's, which the root also gives L10n
        L10n.language = session.language
        let words = ShelfSnapshot.Words(
            continueWatching: L10n.homeRowContinueWatching, myList: L10n.navMyList,
            nothingToContinue: L10n.widgetContinueEmpty)
        let base =
            snapshot
            ?? ShelfSnapshot(account: nil, language: session.language, words: words, accent: session.accent.accent)
        let next = base.updated(
            app: core.app, home: core.home, myList: core.myList, language: session.language, words: words,
            accent: session.accent.accent)
        guard next != snapshot else { return nil }
        snapshot = next
        if let directory {
            do {
                try next.write(to: directory)
            } catch {
                log.error("could not keep the shelf snapshot: \(error)")
            }
        }
        if extensions {
            #if os(iOS)
                WidgetCenter.shared.reloadTimelines(ofKind: SharedContainer.continueWidgetKind)
            #else
                TVTopShelfContentProvider.topShelfContentDidChange()
            #endif
        }
        return next
    }
}

/// What the shelf is made from, so it is looked at again only when one of them changes.
struct ShelfInputs: Equatable {
    let app: AppView
    let home: HomeView
    let myList: MyListView
    let language: String
    let accent: String

    init(_ core: CoreRuntime) {
        app = core.app
        home = core.home
        myList = core.myList
        language = core.session.language
        accent = core.session.accent.accent
    }
}
