import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Movies, series or one genre as a poster grid, sorted and (for movies and series) filtered by
/// genre; more pages load as the end of the grid comes into view.
struct BrowseScreen: View {
    @State private var key: BrowseKey
    @Environment(CoreRuntime.self) private var core

    init(key: BrowseKey) {
        _key = State(initialValue: key)
    }

    private var view: BrowseView { core.browse(key) }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Idiom.isTV ? 40 : Tokens.Spacing.lg) {
                #if os(tvOS)
                    BrowseHeader(title: title, key: $key)
                #endif
                if view.status == .stale {
                    StaleNote(problem: view.problem)
                }
                CatalogStateView(
                    status: view.status, hasContent: !view.cards.isEmpty, problem: view.problem,
                    surface: .browse(key)
                ) {
                    grid
                } skeleton: {
                    GridSkeleton()
                }
            }
            .padding(.vertical, Idiom.isTV ? 40 : Tokens.Spacing.lg)
        }
        .scrollIndicators(.hidden)
        .background(Tokens.Palette.bg)
        .coreScreen(.browse(key))
        .modifier(GenreChoices(enabled: key.kind != nil))
        .catalogRefreshable(.browse(key)) {
            let view = core.browse(key)
            return view.status == .stale && view.problem == nil
        }
        #if os(iOS)
            .navigationTitle(title)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) { BrowseMenu(key: $key) }
            }
        #endif
    }

    private var title: String {
        if let genre = key.genre, key.kind == nil {
            return view.genreLabel ?? genre
        }
        return key.kind == .series ? L10n.navSeries : L10n.navMovies
    }

    @ViewBuilder private var grid: some View {
        if view.cards.isEmpty {
            CatalogMessage(
                systemImage: "film", title: L10n.catalogBrowseEmptyTitle, message: L10n.catalogBrowseEmptyMessage)
        } else {
            PosterGrid(cards: view.cards) {
                if view.more && !view.loadingMore {
                    core.send(.browseMoreRequested(key))
                }
            }
            if view.loadingMore {
                ProgressView()
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, Tokens.Spacing.xl)
            }
        }
    }
}

/// Keeps the genres loaded while a movie or series listing can filter by them.
private struct GenreChoices: ViewModifier {
    let enabled: Bool

    func body(content: Content) -> some View {
        if enabled {
            content.coreScreen(.genres)
        } else {
            content
        }
    }
}

/// The sort order and, for movies and series, the genre.
private struct BrowseMenu: View {
    @Binding var key: BrowseKey
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        Menu {
            Picker(L10n.catalogSortLabel, selection: sort) {
                ForEach([BrowseSort.added, .name, .year], id: \.self) { sort in
                    Text(CatalogLabels.sort(sort)).tag(sort)
                }
            }
            if key.kind != nil {
                Picker(L10n.catalogFilterGenre, selection: genre) {
                    Text(L10n.catalogFilterAll).tag(String?.none)
                    ForEach(core.genres.genres, id: \.name) { genre in
                        Text(genre.label).tag(Optional(genre.name))
                    }
                }
                .pickerStyle(.menu)
            }
        } label: {
            Label(L10n.catalogSortLabel, systemImage: "line.3.horizontal.decrease")
        }
        .accessibilityValue(
            [CatalogLabels.sort(key.sort), key.genre.map(genreLabel)].compactMap { $0 }.joined(separator: ", ")
        )
        .accessibilityIdentifier("browse-menu")
    }

    private func genreLabel(_ name: String) -> String {
        core.genres.genres.first { $0.name == name }?.label ?? name
    }

    private var sort: Binding<BrowseSort> {
        Binding(get: { key.sort }, set: { key = BrowseKey(kind: key.kind, genre: key.genre, sort: $0) })
    }

    private var genre: Binding<String?> {
        Binding(get: { key.genre }, set: { key = BrowseKey(kind: key.kind, genre: $0, sort: key.sort) })
    }
}

#if os(tvOS)
    /// The listing's name with its sort and genre as focusable menus, above the grid.
    private struct BrowseHeader: View {
        let title: String
        @Binding var key: BrowseKey
        @Environment(CoreRuntime.self) private var core

        var body: some View {
            HStack(alignment: .center, spacing: Tokens.Spacing.xl) {
                Text(title)
                    .typeRole(Tokens.TypeRamp.title)
                    .foregroundStyle(Tokens.Palette.text)
                    .accessibilityAddTraits(.isHeader)
                Spacer(minLength: 0)
                Menu {
                    ForEach([BrowseSort.added, .name, .year], id: \.self) { sort in
                        Button {
                            key = BrowseKey(kind: key.kind, genre: key.genre, sort: sort)
                        } label: {
                            if sort == key.sort {
                                Label(CatalogLabels.sort(sort), systemImage: "checkmark")
                            } else {
                                Text(CatalogLabels.sort(sort))
                            }
                        }
                    }
                } label: {
                    Label(CatalogLabels.sort(key.sort), systemImage: "arrow.up.arrow.down")
                }
                // the menus show only what is chosen; VoiceOver also says what they choose
                .accessibilityLabel(L10n.catalogSortLabel)
                .accessibilityValue(CatalogLabels.sort(key.sort))
                if key.kind != nil {
                    Menu {
                        genreButton(nil, label: L10n.catalogFilterAll)
                        ForEach(core.genres.genres, id: \.name) { genre in
                            genreButton(genre.name, label: genre.label)
                        }
                    } label: {
                        Label(
                            core.genres.genres.first { $0.name == key.genre }?.label ?? key.genre
                                ?? L10n.catalogFilterGenre,
                            systemImage: "line.3.horizontal.decrease")
                    }
                    .accessibilityLabel(L10n.catalogFilterGenre)
                    .accessibilityValue(
                        core.genres.genres.first { $0.name == key.genre }?.label ?? key.genre ?? L10n.catalogFilterAll)
                }
            }
            .padding(.horizontal, CardMetrics.edge)
            .focusSection()
        }

        private func genreButton(_ name: String?, label: String) -> some View {
            Button {
                key = BrowseKey(kind: key.kind, genre: name, sort: key.sort)
            } label: {
                if name == key.genre {
                    Label(label, systemImage: "checkmark")
                } else {
                    Text(label)
                }
            }
        }
    }
#endif
