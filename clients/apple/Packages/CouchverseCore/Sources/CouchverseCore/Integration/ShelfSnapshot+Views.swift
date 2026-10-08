import CouchverseShared

extension ShelfSnapshot {
    /// The snapshot once the core's views changed. It empties when no account is left and starts
    /// over for another account, so one profile's titles never show for the next. Continue Watching
    /// and My List are replaced only by views that have loaded: a launch that has not loaded them
    /// yet, or a TV on "Who's watching?", keeps what was there.
    public func updated(
        app: AppView, home: HomeView, myList: MyListView, language: String, words: Words, accent: String
    ) -> ShelfSnapshot {
        switch app.phase {
        case .welcome, .signIn:
            return ShelfSnapshot(account: nil, language: language, words: words, accent: accent)
        case .ready:
            guard let active = app.activeAccount else { return self }
            var next =
                account == active
                ? self : ShelfSnapshot(account: active, language: language, words: words, accent: accent)
            next.language = language
            next.words = words
            next.accent = accent
            if home.status == .loaded {
                next.continueWatching = home.rows.flatMap(\.continueWatching).map(ContinueItem.init)
            }
            if myList.status == .loaded {
                next.myList = myList.cards.map(TitleItem.init)
            }
            return next
        case .starting, .chooseAccount:
            return self
        }
    }
}

extension ShelfSnapshot.ContinueItem {
    public init(_ card: ContinueCard) {
        self.init(
            slug: card.slug, name: card.name, label: card.episodeLabel, progress: card.progress,
            kind: card.play.kind.rawValue, playId: card.play.id, backdrop: card.backdrop?.url, poster: card.poster?.url)
    }
}

extension ShelfSnapshot.TitleItem {
    public init(_ card: Card) {
        self.init(slug: card.slug, name: card.name, year: card.year.map(Int.init), poster: card.poster?.url)
    }
}

extension PlayTarget {
    /// `couchverse://play/<kind>/<id>`.
    public var link: String {
        AppLinks.play(kind: kind.rawValue, id: id)
    }
}
