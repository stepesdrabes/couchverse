import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Picks what a catalog screen shows for its load state: whatever content there is (stale beats
/// blank), its skeleton while a first load runs, otherwise not found or failed with a retry.
struct CatalogStateView<Content: View, Placeholder: View>: View {
    let status: LoadStatus
    let hasContent: Bool
    let problem: Problem?
    let surface: Surface
    var notFound = L10n.errorNotFound
    @ViewBuilder let content: () -> Content
    @ViewBuilder let skeleton: () -> Placeholder

    var body: some View {
        if hasContent || status == .loaded || status == .stale {
            content()
        } else {
            switch status {
            case .notFound:
                CatalogMessage(systemImage: "questionmark.square.dashed", title: notFound, message: nil)
            case .failed:
                LoadFailed(problem: problem, surface: surface)
            default:
                skeleton()
            }
        }
    }
}

/// A cold load that failed: what went wrong and a way to try again.
struct LoadFailed: View {
    let problem: Problem?
    let surface: Surface
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        VStack(spacing: Tokens.Spacing.lg) {
            CatalogMessage(
                systemImage: "wifi.exclamationmark", title: L10n.errorPageTitle, message: problem?.message)
            Button {
                core.send(.refreshRequested(surface))
            } label: {
                Label(L10n.commonRetry, systemImage: "arrow.clockwise")
            }
            .primaryAction()
            .fixedSize()
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, Idiom.isTV ? 120 : 64)
    }
}

/// An empty or unavailable screen: a symbol, a line and an optional explanation.
struct CatalogMessage: View {
    let systemImage: String
    let title: String
    let message: String?

    var body: some View {
        VStack(spacing: Tokens.Spacing.md) {
            Image(systemName: systemImage)
                .font(.system(size: Idiom.isTV ? 72 : 44, weight: .light))
                .foregroundStyle(Tokens.Palette.faintText)
                .accessibilityHidden(true)
            Text(title)
                .typeRole(Tokens.TypeRamp.section)
                .foregroundStyle(Tokens.Palette.text)
                .multilineTextAlignment(.center)
            if let message {
                Text(message)
                    .typeRole(Tokens.TypeRamp.body)
                    .foregroundStyle(Tokens.Palette.mutedText)
                    .multilineTextAlignment(.center)
            }
        }
        .fixedSize(horizontal: false, vertical: true)
        .frame(maxWidth: Idiom.isTV ? 900 : 420)
        .frame(maxWidth: .infinity)
        .padding(.vertical, Idiom.isTV ? 120 : 64)
        .padding(.horizontal, CardMetrics.edge)
        .accessibilityElement(children: .combine)
    }
}

/// Shown above content the core kept from before when it could not refresh it.
struct StaleNote: View {
    let problem: Problem?

    var body: some View {
        if let problem {
            Label(problem.message, systemImage: "wifi.slash")
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.mutedText)
                .padding(.horizontal, CardMetrics.edge)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
    }
}

/// Shelves of grey cards in the shape of a home or a title's rows.
struct ShelfSkeleton: View {
    var backdrops = false

    var body: some View {
        let width = backdrops ? CardMetrics.backdropWidth : CardMetrics.posterWidth
        let height = backdrops ? width * 9 / 16 : width * 1.5
        VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
            Skeleton(width: Idiom.isTV ? 320 : 160, height: Idiom.isTV ? 32 : 20)
                .padding(.horizontal, CardMetrics.edge)
            HStack(spacing: CardMetrics.spacing) {
                ForEach(0..<8, id: \.self) { _ in
                    Skeleton(width: width, height: height, cornerRadius: Tokens.Radius.card)
                }
            }
            .padding(.horizontal, CardMetrics.edge)
            .fixedSize(horizontal: true, vertical: false)
            .frame(maxWidth: .infinity, alignment: .leading)
            .clipped()
        }
    }
}

/// A poster grid's skeleton.
struct GridSkeleton: View {
    var body: some View {
        LazyVGrid(columns: CardMetrics.posterGrid, alignment: .leading, spacing: Idiom.isTV ? 56 : Tokens.Spacing.lg) {
            ForEach(0..<12, id: \.self) { _ in
                Skeleton(aspectRatio: 2 / 3, cornerRadius: Tokens.Radius.card)
            }
        }
        .padding(.horizontal, CardMetrics.edge)
        .accessibilityElement()
        .accessibilityLabel(L10n.commonLoading)
    }
}
