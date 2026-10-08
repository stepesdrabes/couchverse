import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Every genre as a tile, leading to its listing.
struct GenresScreen: View {
    @Environment(CoreRuntime.self) private var core

    private var genres: GenresView { core.genres }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Tokens.Spacing.lg) {
                #if os(tvOS)
                    Text(L10n.navGenres)
                        .typeRole(Tokens.TypeRamp.title)
                        .foregroundStyle(Tokens.Palette.text)
                        .padding(.horizontal, CardMetrics.edge)
                        .accessibilityAddTraits(.isHeader)
                #endif
                if genres.status == .stale {
                    StaleNote(problem: genres.problem)
                }
                CatalogStateView(
                    status: genres.status, hasContent: !genres.genres.isEmpty, problem: genres.problem,
                    surface: .genres
                ) {
                    if genres.genres.isEmpty {
                        CatalogMessage(
                            systemImage: "square.grid.2x2", title: L10n.catalogGenresEmptyTitle,
                            message: L10n.catalogGenresEmptyMessage)
                    } else {
                        grid
                    }
                } skeleton: {
                    tiles(count: 8) { _ in Skeleton(height: tileHeight, cornerRadius: Tokens.Radius.card) }
                        .accessibilityElement()
                        .accessibilityLabel(L10n.commonLoading)
                }
            }
            .padding(.vertical, Idiom.isTV ? 40 : Tokens.Spacing.lg)
        }
        .scrollIndicators(.hidden)
        .background(Tokens.Palette.bg)
        .coreScreen(.genres)
        .catalogRefreshable(.genres) { core.genres.status == .stale && core.genres.problem == nil }
        #if os(iOS)
            .navigationTitle(L10n.navGenres)
        #endif
    }

    private var tileHeight: CGFloat { Idiom.isTV ? 160 : 88 }

    private var grid: some View {
        tiles(count: genres.genres.count) { index in
            let genre = genres.genres[index]
            NavigationLink(value: CatalogRoute.browse(BrowseKey(kind: nil, genre: genre.name, sort: .added))) {
                GenreTile(label: genre.label, seed: genre.name)
                    .frame(height: tileHeight)
            }
            .cardButtonStyle()
        }
    }

    private func tiles(count: Int, @ViewBuilder tile: @escaping (Int) -> some View) -> some View {
        LazyVGrid(
            columns: [GridItem(.adaptive(minimum: Idiom.isTV ? 360 : 160), spacing: CardMetrics.spacing)],
            spacing: CardMetrics.spacing
        ) {
            ForEach(0..<count, id: \.self, content: tile)
        }
        .padding(.horizontal, CardMetrics.edge)
    }
}

/// A genre's name over a gradient; the hue comes from the name, so a genre keeps its colour.
private struct GenreTile: View {
    let label: String
    let seed: String

    var body: some View {
        let hue = Double(seed.unicodeScalars.reduce(UInt32(7)) { ($0 &* 31) &+ $1.value } % 360) / 360
        ZStack(alignment: .bottomLeading) {
            LinearGradient(
                colors: [Color(hue: hue, saturation: 0.55, brightness: 0.5), Tokens.Palette.surface2],
                startPoint: .topLeading, endPoint: .bottomTrailing)
            Text(label)
                .typeRole(Tokens.TypeRamp.card)
                .foregroundStyle(Tokens.Palette.text)
                .lineLimit(2)
                .minimumScaleFactor(0.7)
                .padding(Idiom.isTV ? Tokens.Spacing.xl : Tokens.Spacing.md)
        }
        .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
        .accessibilityElement()
        .accessibilityLabel(label)
    }
}
