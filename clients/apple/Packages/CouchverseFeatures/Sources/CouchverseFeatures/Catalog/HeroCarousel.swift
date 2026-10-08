import CouchverseCore
import CouchverseDesign
import SwiftUI

/// The featured titles, one at a time over their backdrops, moving on every 8 seconds (plan 12.2)
/// unless the viewer is busy with it: a focused button on TV, a finger on a phone.
struct HeroCarousel: View {
    let featured: [FeaturedCard]

    @Environment(CoreRuntime.self) private var core
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var index = 0
    @State private var touching = false
    @FocusState private var focused: Bool

    private var height: CGFloat {
        #if os(tvOS)
            820
        #else
            UIDevice.current.userInterfaceIdiom == .pad ? 600 : 520
        #endif
    }

    var body: some View {
        let current = featured[min(index, featured.count - 1)]
        // the slide sets the height, at least the backdrop's, so large text grows the hero
        HeroSlide(card: current, focus: $focused)
            .id("slide-\(current.titleId)")
            .transition(.opacity.combined(with: .offset(y: reduceMotion ? 0 : 16)))
            .padding(.horizontal, CardMetrics.edge)
            .padding(.top, height * 0.4)
            .padding(.bottom, Idiom.isTV ? 40 : Tokens.Spacing.xl)
            .frame(maxWidth: .infinity, minHeight: height, alignment: .bottomLeading)
            .background(alignment: .top) {
                ArtworkImage(image: current.backdrop)
                    .id(current.titleId)
                    .transition(.opacity)
                    .frame(height: height)
                    .overlay {
                        LinearGradient(
                            stops: [
                                .init(color: Tokens.Palette.bg.opacity(0), location: 0.3),
                                .init(color: Tokens.Palette.bg.opacity(0.75), location: 0.75),
                                .init(color: Tokens.Palette.bg, location: 1),
                            ], startPoint: .top, endPoint: .bottom)
                    }
                    .overlay {
                        LinearGradient(
                            colors: [Tokens.Palette.bg.opacity(0.85), .clear], startPoint: .leading,
                            endPoint: .center)
                    }
                    // edge to edge, under the TV's overscan margins too
                    .ignoresSafeArea(edges: .horizontal)
                    .accessibilityHidden(true)
            }
            .overlay(alignment: .bottomTrailing) {
                if featured.count > 1 {
                    PageDots(count: featured.count, index: index)
                        .padding(.trailing, CardMetrics.edge)
                        .padding(.bottom, Idiom.isTV ? 48 : Tokens.Spacing.xl)
                }
            }
            .animation(Tokens.Motion.smooth, value: index)
            #if os(iOS)
                .simultaneousGesture(swipe)
            #endif
            .accessibilityElement(children: .contain)
            .accessibilityLabel(L10n.catalogFeaturedTitles)
            .task(id: featured.map(\.titleId)) {
                index = 0
                while !Task.isCancelled {
                    try? await Task.sleep(for: Tokens.Motion.heroInterval)
                    if !focused && !touching && featured.count > 1 {
                        index = (index + 1) % featured.count
                    }
                }
            }
    }

    #if os(iOS)
        private var swipe: some Gesture {
            DragGesture(minimumDistance: 20)
                .onChanged { _ in touching = true }
                .onEnded { value in
                    touching = false
                    guard featured.count > 1, abs(value.translation.width) > 60 else { return }
                    index = (index + (value.translation.width < 0 ? 1 : featured.count - 1)) % featured.count
                }
        }
    #endif
}

private struct HeroSlide: View {
    let card: FeaturedCard
    /// True while any of the slide's buttons has focus, which holds the carousel still.
    let focus: FocusState<Bool>.Binding
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        VStack(alignment: .leading, spacing: Idiom.isTV ? Tokens.Spacing.lg : Tokens.Spacing.md) {
            Text(([L10n.catalogFeatured] + card.genres.prefix(2)).joined(separator: " \u{00B7} "))
                .typeRole(Tokens.TypeRamp.eyebrow)
                .textCase(.uppercase)
                .foregroundStyle(Tokens.Palette.muted)
            TitleLogo(
                logo: card.logo, name: card.name, maxWidth: Idiom.isTV ? 640 : 300,
                maxHeight: Idiom.isTV ? 200 : 110)
            Text(
                CatalogLabels.facts(
                    year: card.year, rating: card.contentRating, runtime: card.runtimeMinutes, kind: card.kind)
            )
            .typeRole(Tokens.TypeRamp.caption)
            .foregroundStyle(Tokens.Palette.muted)
            if !card.overview.isEmpty {
                Text(card.overview)
                    .typeRole(Tokens.TypeRamp.body)
                    .foregroundStyle(Tokens.Palette.text.opacity(0.85))
                    .lineLimit(Idiom.isTV ? 3 : 2)
                    .frame(maxWidth: Idiom.isTV ? 900 : 560, alignment: .leading)
            }
            // a row where it fits, else stacked, wrapping at the largest text sizes
            ViewThatFits(in: .horizontal) {
                HStack(spacing: Tokens.Spacing.md) { buttons(fixed: true) }
                VStack(alignment: .leading, spacing: Tokens.Spacing.md) { buttons(fixed: false) }
            }
            .tvFocusSection()
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    @ViewBuilder private func buttons(fixed: Bool) -> some View {
        if card.kind == .movie {
            Button {
                core.send(.playRequested(PlayTarget(kind: .movie, id: card.titleId)))
            } label: {
                Label(L10n.commonPlay, systemImage: "play.fill")
            }
            .primaryAction()
            .fixedWidth(fixed)
            .focused(focus)
        }
        NavigationLink(value: CatalogRoute.title(slug: card.slug)) {
            Label(L10n.catalogMoreInfo, systemImage: "info.circle")
        }
        .modifier(MoreInfoStyle(primary: card.kind == .series))
        .fixedWidth(fixed)
        .focused(focus)
        MyListButton(titleId: card.titleId, inList: card.inList)
            .fixedSize()
            .focused(focus)
    }
}

/// "More info" leads when there is nothing to play straight away (a series).
private struct MoreInfoStyle: ViewModifier {
    let primary: Bool

    func body(content: Content) -> some View {
        if primary {
            content.primaryAction()
        } else {
            content.secondaryAction()
        }
    }
}

/// Adds or removes a title from My List; the core shows the change at once and rolls it back
/// if the server refuses.
struct MyListButton: View {
    let titleId: String
    let inList: Bool
    var labelled = false
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        Button {
            core.send(.watchlistChanged(WatchlistChange(titleId: titleId, listed: !inList)))
        } label: {
            if labelled {
                Label(L10n.navMyList, systemImage: inList ? "checkmark" : "plus")
            } else {
                Image(systemName: inList ? "checkmark" : "plus")
            }
        }
        .secondaryAction()
        .contentTransition(.symbolEffect(.replace))
        .accessibilityLabel(inList ? L10n.catalogRemoveFromList : L10n.catalogAddToList)
        .accessibilityAddTraits(inList ? .isSelected : [])
    }
}

private struct PageDots: View {
    let count: Int
    let index: Int

    var body: some View {
        HStack(spacing: Tokens.Spacing.sm) {
            ForEach(0..<count, id: \.self) { i in
                Capsule()
                    .fill(i == index ? Tokens.Palette.text : Tokens.Palette.text.opacity(0.3))
                    .frame(width: i == index ? 22 : 8, height: 8)
            }
        }
        .accessibilityHidden(true)
    }
}
