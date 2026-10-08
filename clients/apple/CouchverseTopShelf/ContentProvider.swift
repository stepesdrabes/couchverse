import CouchverseShared
import Foundation
import TVServices

/// Continue Watching on the Apple TV's Top Shelf, from the shelf snapshot the app keeps in the App
/// Group: each title plays where it stopped, or opens its page. The artwork URLs carry the
/// account's artwork grant (7 days), so the system loads them without a session (spike S6); with
/// nothing to continue the app's static Top Shelf image shows instead.
nonisolated final class ContentProvider: TVTopShelfContentProvider {
    override func loadTopShelfContent() async -> (any TVTopShelfContent)? {
        guard let snapshot = ShelfSnapshot.current(), !snapshot.continueWatching.isEmpty else { return nil }
        let section = TVTopShelfItemCollection(items: snapshot.continueWatching.map(Self.item))
        section.title = snapshot.words.continueWatching
        return TVTopShelfSectionedContent(sections: [section])
    }

    private static func item(_ entry: ShelfSnapshot.ContinueItem) -> TVTopShelfSectionedItem {
        let item = TVTopShelfSectionedItem(identifier: entry.playLink)
        item.title = [entry.name, entry.label].compactMap { $0 }.joined(separator: " \u{00B7} ")
        item.imageShape = .hdtv
        if let image = (entry.backdrop ?? entry.poster).flatMap(URL.init(string:)) {
            item.setImageURL(image, for: [.screenScale1x, .screenScale2x])
        }
        item.playbackProgress = min(max(entry.progress, 0), 1)
        item.playAction = URL(string: entry.playLink).map(TVTopShelfAction.init(url:))
        item.displayAction = URL(string: entry.titleLink).map(TVTopShelfAction.init(url:))
        return item
    }
}
