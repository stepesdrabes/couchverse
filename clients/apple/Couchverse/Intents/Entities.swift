import AppIntents
import CouchverseShared

/// A title Shortcuts and Siri can name: those in Continue Watching and My List, read from the shelf
/// snapshot the app keeps, so a query needs neither the core nor the network.
nonisolated struct TitleEntity: AppEntity {
    static let typeDisplayRepresentation = TypeDisplayRepresentation(name: LocalizedStringResource("intent_title"))
    static let defaultQuery = TitleQuery()

    /// The title's slug.
    let id: String
    let name: String

    var displayRepresentation: DisplayRepresentation {
        DisplayRepresentation(title: LocalizedStringResource(stringLiteral: name))
    }
}

nonisolated struct TitleQuery: EntityStringQuery {
    func entities(for identifiers: [String]) async throws -> [TitleEntity] {
        let titles = ShelfSnapshot.current()?.titles ?? []
        // a saved shortcut keeps opening a title that has left the shelf since
        return identifiers.map { slug in
            TitleEntity(id: slug, name: titles.first { $0.slug == slug }?.name ?? slug)
        }
    }

    func entities(matching string: String) async throws -> [TitleEntity] {
        (ShelfSnapshot.current()?.titles(matching: string) ?? []).map { TitleEntity(id: $0.slug, name: $0.name) }
    }

    func suggestedEntities() async throws -> [TitleEntity] {
        (ShelfSnapshot.current()?.titles ?? []).map { TitleEntity(id: $0.slug, name: $0.name) }
    }
}

/// A movie or an episode in Continue Watching, identified by the `couchverse://play` link that
/// plays it.
nonisolated struct ContinueEntity: AppEntity {
    static let typeDisplayRepresentation = TypeDisplayRepresentation(
        name: LocalizedStringResource("intent_in_progress"))
    static let defaultQuery = ContinueQuery()

    let id: String
    let name: String
    let label: String?

    init(_ item: ShelfSnapshot.ContinueItem) {
        id = item.playLink
        name = item.name
        label = item.label
    }

    var displayRepresentation: DisplayRepresentation {
        DisplayRepresentation(
            title: LocalizedStringResource(stringLiteral: name),
            subtitle: label.map { LocalizedStringResource(stringLiteral: $0) })
    }
}

nonisolated struct ContinueQuery: EntityQuery {
    /// One no longer in progress is gone, and the intent continues the first title instead.
    func entities(for identifiers: [String]) async throws -> [ContinueEntity] {
        let items = ShelfSnapshot.current()?.continueWatching ?? []
        return items.filter { identifiers.contains($0.playLink) }.map(ContinueEntity.init)
    }

    func suggestedEntities() async throws -> [ContinueEntity] {
        (ShelfSnapshot.current()?.continueWatching ?? []).map(ContinueEntity.init)
    }
}
