import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Card sizes per idiom: a TV is read from the couch, so everything is bigger and fewer fit.
enum CardMetrics {
    static var posterWidth: CGFloat {
        #if os(tvOS)
            220
        #else
            UIDevice.current.userInterfaceIdiom == .pad ? 150 : 112
        #endif
    }

    static var backdropWidth: CGFloat {
        #if os(tvOS)
            440
        #else
            UIDevice.current.userInterfaceIdiom == .pad ? 320 : 250
        #endif
    }

    static var spacing: CGFloat { Idiom.isTV ? 48 : Tokens.Spacing.md }
    /// Between a card and its caption: on TV, clear of the card's focus lift.
    static var captionGap: CGFloat { Idiom.isTV ? 28 : Tokens.Spacing.sm }
    static var edge: CGFloat { Idiom.isTV ? 80 : Tokens.Spacing.lg }
}

/// A title's poster, leading to its page. On TV the system's card style lifts it with parallax.
struct PosterCard: View {
    let card: Card
    var width: CGFloat = CardMetrics.posterWidth

    var body: some View {
        VStack(alignment: .leading, spacing: CardMetrics.captionGap) {
            NavigationLink(value: CatalogRoute.title(slug: card.slug)) {
                ArtworkImage(image: card.poster)
                    .overlay(alignment: .bottomLeading) {
                        if card.poster == nil {
                            Text(card.name)
                                .typeRole(Tokens.TypeRamp.card)
                                .foregroundStyle(Tokens.Palette.text)
                                .padding(Tokens.Spacing.sm)
                        }
                    }
                    .frame(width: width, height: width * 1.5)
                    .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
            }
            .cardButtonStyle()
            .accessibilityLabel(card.name)
            .accessibilityValue(card.year.map(String.init) ?? "")
            Text(card.name)
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.muted)
                .lineLimit(1)
                .frame(width: width, alignment: .leading)
                .accessibilityHidden(true)
        }
    }
}

/// Continue Watching: the backdrop with how far in, starting playback where it stopped.
struct ContinueCardView: View {
    let card: ContinueCard
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        let width = CardMetrics.backdropWidth
        VStack(alignment: .leading, spacing: CardMetrics.captionGap) {
            Button {
                core.send(.playRequested(card.play))
            } label: {
                ArtworkImage(image: card.backdrop ?? card.poster)
                    .frame(width: width, height: width * 9 / 16)
                    .overlay(alignment: .bottom) {
                        ProgressBar(fraction: card.progress)
                            .padding(Tokens.Spacing.sm)
                    }
                    .overlay {
                        Image(systemName: "play.fill")
                            .font(.system(size: Idiom.isTV ? 44 : 26))
                            .foregroundStyle(.white)
                            .padding(Idiom.isTV ? 24 : 14)
                            .glassEffect(.regular, in: Circle())
                    }
                    .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
            }
            .cardButtonStyle()
            .contextMenu {
                NavigationLink(value: CatalogRoute.title(slug: card.slug)) {
                    Label(L10n.catalogMoreInfo, systemImage: "info.circle")
                }
            }
            .accessibilityLabel([card.name, card.episodeLabel].compactMap { $0 }.joined(separator: ", "))
            .accessibilityValue(L10n.catalogResumeFrom(time: CatalogLabels.clock(seconds: card.positionSeconds)))
            VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) {
                Text(card.name)
                    .typeRole(Tokens.TypeRamp.card)
                    .foregroundStyle(Tokens.Palette.text)
                if let label = card.episodeLabel {
                    Text(label)
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.muted)
                }
            }
            .lineLimit(1)
            .frame(width: width, alignment: .leading)
            .accessibilityHidden(true)
        }
    }
}

/// How far into a title, as a thin accent bar.
struct ProgressBar: View {
    let fraction: Double
    @Environment(\.accent) private var accent

    var body: some View {
        GeometryReader { geometry in
            ZStack(alignment: .leading) {
                Capsule().fill(.white.opacity(0.25))
                Capsule().fill(accent.color)
                    .frame(width: geometry.size.width * min(max(fraction, 0), 1))
            }
        }
        .frame(height: Idiom.isTV ? 6 : 4)
        .accessibilityHidden(true)
    }
}

/// A labelled horizontal shelf of cards.
struct ShelfRow<Content: View>: View {
    let title: String
    @ViewBuilder let content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
            Text(title)
                .typeRole(Tokens.TypeRamp.section)
                .foregroundStyle(Tokens.Palette.text)
                .padding(.horizontal, CardMetrics.edge)
                .accessibilityAddTraits(.isHeader)
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: CardMetrics.spacing) {
                    content()
                }
                .padding(.horizontal, CardMetrics.edge)
                .padding(.vertical, Idiom.isTV ? 24 : 0)
            }
            .scrollIndicators(.hidden)
            .scrollClipDisabled()
        }
        .tvFocusSection()
    }
}

/// A grid of posters that asks for more as its end comes into view.
struct PosterGrid: View {
    let cards: [Card]
    var onNearEnd: (() -> Void)?

    var body: some View {
        let width = CardMetrics.posterWidth
        LazyVGrid(
            columns: [GridItem(.adaptive(minimum: width, maximum: width * 1.25), spacing: CardMetrics.spacing)],
            alignment: .leading, spacing: Idiom.isTV ? 56 : Tokens.Spacing.lg
        ) {
            ForEach(Array(cards.enumerated()), id: \.element.titleId) { index, card in
                PosterCard(card: card, width: width)
                    .onAppear {
                        if index >= cards.count - 8 {
                            onNearEnd?()
                        }
                    }
            }
        }
        .padding(.horizontal, CardMetrics.edge)
        // moving down from controls above lands in the grid, not only on what is straight below
        .tvFocusSection()
    }
}

extension View {
    /// Keeps a button at its label's width in a row; stacked at the largest text sizes, it takes
    /// the width there is and its label wraps.
    @ViewBuilder func fixedWidth(_ fixed: Bool) -> some View {
        if fixed {
            fixedSize()
        } else {
            self
        }
    }

    /// The tvOS card style (lift, parallax, specular highlight); plain on touch devices.
    func cardButtonStyle() -> some View {
        #if os(tvOS)
            buttonStyle(.card)
        #else
            buttonStyle(.plain)
        #endif
    }
}
