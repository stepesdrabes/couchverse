import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The titles the viewer saved, newest first.
struct MyListScreen: View {
    @Environment(CoreRuntime.self) private var core

    private var list: MyListView { core.myList }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Tokens.Spacing.lg) {
                #if os(tvOS)
                    Text(L10n.navMyList)
                        .typeRole(Tokens.TypeRamp.title)
                        .foregroundStyle(Tokens.Palette.text)
                        .padding(.horizontal, CardMetrics.edge)
                        .accessibilityAddTraits(.isHeader)
                #endif
                if list.status == .stale {
                    StaleNote(problem: list.problem)
                }
                CatalogStateView(
                    status: list.status, hasContent: !list.cards.isEmpty, problem: list.problem, surface: .myList
                ) {
                    if list.cards.isEmpty {
                        CatalogMessage(
                            systemImage: "plus.square.dashed", title: L10n.catalogMyListEmptyTitle,
                            message: L10n.catalogMyListEmptyMessage)
                    } else {
                        PosterGrid(cards: list.cards)
                    }
                } skeleton: {
                    GridSkeleton()
                }
            }
            .padding(.vertical, Idiom.isTV ? 40 : Tokens.Spacing.lg)
        }
        .scrollIndicators(.hidden)
        .background(Tokens.Palette.bg)
        .coreScreen(.myList)
        .catalogRefreshable(.myList) { core.myList.status == .stale && core.myList.problem == nil }
        #if os(iOS)
            .navigationTitle(L10n.navMyList)
        #endif
    }
}
