import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Search as you type: every keystroke goes to the core, which debounces it, drops answers to
/// older queries and keeps the last one, so coming back shows it again.
struct SearchScreen: View {
    @Environment(CoreRuntime.self) private var core
    @State private var query = ""

    private var search: SearchView { core.search }

    var body: some View {
        ScrollView {
            content
                .padding(.vertical, Idiom.isTV ? 40 : Tokens.Spacing.lg)
        }
        .scrollIndicators(.hidden)
        .scrollDismissesKeyboard(.immediately)
        .background(Tokens.Palette.bg)
        .searchable(text: $query, prompt: L10n.catalogSearchPlaceholder)
        .onChange(of: query) { _, query in
            if query != core.search.query {
                core.send(.searchChanged(SearchText(query: query)))
            }
        }
        .task { query = core.search.query }
        .coreScreen(.search)
        #if os(iOS)
            .navigationTitle(L10n.navSearch)
        #endif
    }

    @ViewBuilder private var content: some View {
        let trimmed = search.query.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            CatalogMessage(systemImage: "magnifyingglass", title: L10n.catalogSearchPlaceholder, message: nil)
        } else if !search.cards.isEmpty {
            PosterGrid(cards: search.cards)
        } else {
            switch search.status {
            case .loading, .stale, .idle:
                GridSkeleton()
            case .failed:
                VStack(spacing: Tokens.Spacing.lg) {
                    CatalogMessage(
                        systemImage: "wifi.exclamationmark", title: L10n.errorPageTitle,
                        message: search.problem?.message)
                    Button {
                        core.send(.searchChanged(SearchText(query: search.query)))
                    } label: {
                        Label(L10n.commonRetry, systemImage: "arrow.clockwise")
                    }
                    .primaryAction()
                    .fixedSize()
                }
            default:
                CatalogMessage(
                    systemImage: "magnifyingglass", title: L10n.catalogSearchEmptyTitle,
                    message: L10n.catalogSearchEmptyMessage(query: trimmed))
            }
        }
    }
}
