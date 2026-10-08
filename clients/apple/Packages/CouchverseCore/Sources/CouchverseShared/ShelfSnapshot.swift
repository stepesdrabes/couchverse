import Foundation

/// What the app leaves outside itself for its widget, Top Shelf, intents and Spotlight: the active
/// account's Continue Watching and My List, with the few words the extensions show, already in the
/// display language (Android keeps the same in `widget/continue.json`). Extensions only read it;
/// they never run the core.
public struct ShelfSnapshot: Codable, Sendable, Hashable {
    /// The account it was taken from; `nil` once no account is signed in.
    public var account: String?
    /// The display language its names and words are in.
    public var language: String
    public var words: Words
    /// The session's accent, `#rrggbb`.
    public var accent: String
    public var continueWatching: [ContinueItem]
    public var myList: [TitleItem]

    public init(
        account: String?, language: String, words: Words, accent: String, continueWatching: [ContinueItem] = [],
        myList: [TitleItem] = []
    ) {
        self.account = account
        self.language = language
        self.words = words
        self.accent = accent
        self.continueWatching = continueWatching
        self.myList = myList
    }

    /// Words in the display language.
    public struct Words: Codable, Sendable, Hashable {
        /// Continue Watching's heading.
        public var continueWatching: String
        public var myList: String
        /// What the widget says with nothing to continue.
        public var nothingToContinue: String

        public init(continueWatching: String, myList: String, nothingToContinue: String) {
            self.continueWatching = continueWatching
            self.myList = myList
            self.nothingToContinue = nothingToContinue
        }
    }

    /// A movie or an episode in Continue Watching.
    public struct ContinueItem: Codable, Sendable, Hashable, Identifiable {
        /// The title's slug.
        public var slug: String
        public var name: String
        /// The episode's label (`S1 E3`); absent for a movie.
        public var label: String?
        /// How far in, from 0 to 1.
        public var progress: Double
        /// `movie` or `episode`, and the id `couchverse://play` takes.
        public var kind: String
        public var playId: String
        /// Artwork URLs; they carry the account's artwork grant, so no session is needed to load
        /// them while it lasts (7 days).
        public var backdrop: String?
        public var poster: String?

        public init(
            slug: String, name: String, label: String?, progress: Double, kind: String, playId: String,
            backdrop: String?, poster: String?
        ) {
            self.slug = slug
            self.name = name
            self.label = label
            self.progress = progress
            self.kind = kind
            self.playId = playId
            self.backdrop = backdrop
            self.poster = poster
        }

        public var id: String { playLink }
        public var playLink: String { AppLinks.play(kind: kind, id: playId) }
        public var titleLink: String { AppLinks.title(slug: slug) }
    }

    /// A title in My List.
    public struct TitleItem: Codable, Sendable, Hashable, Identifiable {
        public var slug: String
        public var name: String
        public var year: Int?
        public var poster: String?

        public init(slug: String, name: String, year: Int?, poster: String?) {
            self.slug = slug
            self.name = name
            self.year = year
            self.poster = poster
        }

        public var id: String { slug }
        public var link: String { AppLinks.title(slug: slug) }
    }

    /// Every title it names, each once, Continue Watching's first: what the intents can open.
    public var titles: [TitleItem] {
        var seen = Set<String>()
        let continuing = continueWatching.map { TitleItem(slug: $0.slug, name: $0.name, year: nil, poster: $0.poster) }
        return (continuing + myList).filter { seen.insert($0.slug).inserted }
    }

    /// The titles whose name contains `text`, ignoring case and accents.
    public func titles(matching text: String) -> [TitleItem] {
        let wanted = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !wanted.isEmpty else { return titles }
        return titles.filter { $0.name.range(of: wanted, options: [.caseInsensitive, .diacriticInsensitive]) != nil }
    }
}

extension ShelfSnapshot {
    public static let fileName = "shelf.json"

    /// The snapshot kept in `directory`, if there is a readable one.
    public static func read(from directory: URL) -> ShelfSnapshot? {
        guard let data = try? Data(contentsOf: directory.appending(path: fileName)) else { return nil }
        return try? JSONDecoder().decode(ShelfSnapshot.self, from: data)
    }

    /// Replaces the snapshot in `directory` in one step, so an extension never reads half of it.
    public func write(to directory: URL) throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.sortedKeys]
        try encoder.encode(self).write(to: directory.appending(path: Self.fileName), options: .atomic)
    }
}
