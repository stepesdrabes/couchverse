#if os(iOS) || os(macOS)
    import CoreSpotlight
    import CouchverseShared
    import Foundation
    import UniformTypeIdentifiers

    /// A title as Spotlight finds it on iPhone and iPad; choosing it opens the title's page.
    public struct SpotlightEntry: Hashable, Sendable {
        /// `couchverse://title/<slug>`, also the item's identifier, so the app opens it as a link.
        public let link: String
        public let name: String
        /// Where it comes from: "Continue Watching · S1 E3", or "My List".
        public let detail: String
        /// The poster, shown when the image cache already has it: Spotlight takes no remote
        /// images, and indexing should not download any.
        public let poster: String?

        public init(link: String, name: String, detail: String, poster: String?) {
            self.link = link
            self.name = name
            self.detail = detail
            self.poster = poster
        }

        /// The item in the account's domain, so an account's items can be dropped together.
        public func item(domain: String, thumbnail: Data?) -> CSSearchableItem {
            let attributes = CSSearchableItemAttributeSet(contentType: .movie)
            attributes.title = name
            attributes.displayName = name
            attributes.contentDescription = detail
            attributes.thumbnailData = thumbnail
            return CSSearchableItem(uniqueIdentifier: link, domainIdentifier: domain, attributeSet: attributes)
        }
    }

    extension ShelfSnapshot {
        /// One entry per title, Continue Watching first; none without an account.
        public var spotlightEntries: [SpotlightEntry] {
            guard account != nil else { return [] }
            var seen = Set<String>()
            let continuing = continueWatching.map { item in
                SpotlightEntry(
                    link: item.titleLink, name: item.name,
                    detail: [words.continueWatching, item.label].compactMap { $0 }.joined(separator: " \u{00B7} "),
                    poster: item.poster)
            }
            let listed = myList.map { title in
                SpotlightEntry(link: title.link, name: title.name, detail: words.myList, poster: title.poster)
            }
            return (continuing + listed).filter { seen.insert($0.link).inserted }
        }
    }
#endif
