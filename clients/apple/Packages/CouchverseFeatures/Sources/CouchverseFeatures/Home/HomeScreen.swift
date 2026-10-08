import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Home: the featured hero, then the server's rows (Continue Watching, the newest titles and the
/// admin's genre rows). The core keeps the last home per account, so a warm start paints it at
/// once and refreshes it behind.
struct HomeScreen: View {
    @Environment(CoreRuntime.self) private var core

    private var home: HomeView { core.home }
    private var hasContent: Bool { !home.featured.isEmpty || !home.rows.isEmpty }

    var body: some View {
        ScrollView {
            CatalogStateView(
                status: home.status, hasContent: hasContent, problem: home.problem, surface: .home
            ) {
                content
            } skeleton: {
                HomeSkeleton()
            }
        }
        .scrollIndicators(.hidden)
        .scrollBounceBehavior(.basedOnSize)
        .background(Tokens.Palette.bg)
        .ignoresSafeArea(edges: hasContent && !home.featured.isEmpty ? .top : [])
        .coreScreen(.home, showsProgress: true)
        .catalogRefreshable(.home) { core.home.status == .stale && core.home.problem == nil }
        #if os(iOS)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) { AccountButton() }
            }
            .toolbarBackgroundVisibility(.hidden, for: .navigationBar)
        #endif
    }

    @ViewBuilder private var content: some View {
        if !hasContent {
            CatalogMessage(
                systemImage: "film.stack", title: L10n.catalogLibraryEmptyTitle,
                message: L10n.catalogLibraryEmptyMessage)
        } else {
            LazyVStack(alignment: .leading, spacing: Idiom.isTV ? 64 : Tokens.Spacing.xxl) {
                if !home.featured.isEmpty {
                    HeroCarousel(featured: home.featured)
                }
                if home.status == .stale {
                    StaleNote(problem: home.problem)
                }
                ForEach(home.rows, id: \.id) { row in
                    HomeRow(row: row)
                }
            }
            .padding(.bottom, Idiom.isTV ? 80 : Tokens.Spacing.xxl)
        }
    }
}

private struct HomeRow: View {
    let row: HomeRowView

    var body: some View {
        ShelfRow(title: CatalogLabels.row(row)) {
            if row.kind == .continueWatching {
                ForEach(row.continueWatching, id: \.play) { card in
                    ContinueCardView(card: card)
                }
            } else {
                ForEach(row.cards, id: \.titleId) { card in
                    PosterCard(card: card, shelf: row.id)
                }
            }
        }
    }
}

/// The hero's and two shelves' shapes while a first home loads.
private struct HomeSkeleton: View {
    var body: some View {
        VStack(alignment: .leading, spacing: Idiom.isTV ? 64 : Tokens.Spacing.xxl) {
            VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                Skeleton(width: Idiom.isTV ? 520 : 240, height: Idiom.isTV ? 120 : 64)
                Skeleton(width: Idiom.isTV ? 760 : 300, height: 16)
                Skeleton(width: Idiom.isTV ? 620 : 260, height: 16)
                HStack(spacing: Tokens.Spacing.md) {
                    Skeleton(
                        width: Idiom.isTV ? 220 : 120, height: Idiom.isTV ? 66 : 48, cornerRadius: Tokens.Radius.pill)
                    Skeleton(
                        width: Idiom.isTV ? 220 : 120, height: Idiom.isTV ? 66 : 48, cornerRadius: Tokens.Radius.pill)
                }
            }
            .padding(.horizontal, CardMetrics.edge)
            .padding(.top, Idiom.isTV ? 300 : 220)
            ShelfSkeleton(backdrops: true)
            ShelfSkeleton()
        }
        .accessibilityElement()
        .accessibilityLabel(L10n.commonLoading)
    }
}

/// The signed-in profile's avatar, opening the account switcher (iPhone and iPad).
struct AccountButton: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.showAccountSwitcher) private var showAccountSwitcher

    var body: some View {
        let card = core.accounts.accounts.first { $0.id == core.app.activeAccount }
        Button {
            showAccountSwitcher()
        } label: {
            AvatarView(url: card?.avatarUrl, seed: card?.username ?? "", name: card?.displayName ?? "")
                .frame(width: 32, height: 32)
        }
        .accessibilityLabel(L10n.accountsSwitch)
        .accessibilityValue(card?.displayName ?? "")
        .accessibilityIdentifier("account-switcher")
    }
}
