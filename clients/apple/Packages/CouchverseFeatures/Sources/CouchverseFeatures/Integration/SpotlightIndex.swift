#if os(iOS)
    import CoreSpotlight
    import CouchverseCore
    import CouchverseShared
    import Foundation
    import os

    nonisolated private let log = Logger(subsystem: "io.stepes.couchverse", category: "spotlight")

    /// The active account's Continue Watching and My List in Spotlight on iPhone and iPad. Each
    /// change replaces the account's items as a whole (they share its domain), signing out drops
    /// them all and a switch drops the previous account's. Changes run in order, off the main
    /// actor.
    @MainActor
    final class SpotlightIndex {
        private var indexed: (account: String?, entries: [SpotlightEntry])?
        private var work: Task<Void, Never>?

        func update(_ snapshot: ShelfSnapshot) {
            let entries = snapshot.spotlightEntries
            let account = snapshot.account
            if let indexed, indexed.account == account, indexed.entries == entries {
                return
            }
            let previous = indexed?.account
            indexed = (account, entries)
            let before = work
            work = Task.detached(priority: .utility) {
                await before?.value
                await Self.replace(previous: previous, account: account, entries: entries)
            }
        }

        private nonisolated static func replace(previous: String?, account: String?, entries: [SpotlightEntry]) async {
            let index = CSSearchableIndex.default()
            do {
                guard let account else {
                    try await index.deleteAllSearchableItems()
                    return
                }
                let domains = [account] + [previous].compactMap { $0 }.filter { $0 != account }
                try await index.deleteSearchableItems(withDomainIdentifiers: domains)
                let items = entries.map { $0.item(domain: account, thumbnail: cachedImage($0.poster)) }
                if !items.isEmpty {
                    try await index.indexSearchableItems(items)
                }
            } catch {
                log.error("could not update Spotlight: \(error)")
            }
        }

        /// The poster as the image cache keeps it from the screens that showed it; nothing is
        /// downloaded for Spotlight.
        private nonisolated static func cachedImage(_ url: String?) -> Data? {
            guard let url = url.flatMap(URL.init(string:)) else { return nil }
            return URLCache.shared.cachedResponse(for: URLRequest(url: url))?.data
        }
    }
#endif
