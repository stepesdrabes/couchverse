import CouchverseCore
import CouchverseDesign
import SwiftUI

/// A movie or series: backdrop, logo, facts and overview, Play or Resume from the core's play
/// action, My List, and a series' seasons with each episode's progress. Tinted with the title's
/// own accent.
struct TitleScreen: View {
    let slug: String
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        let view = core.title(slug)
        CatalogStateView(
            status: view.status, hasContent: view.detail != nil, problem: view.problem, surface: .title(slug),
            notFound: L10n.catalogTitleNotFound
        ) {
            if let detail = view.detail {
                TitleContent(detail: detail, problem: view.status == .stale ? view.problem : nil)
            }
        } skeleton: {
            TitleSkeleton()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
        .background(Tokens.Palette.bg)
        .coreScreen(.title(slug), showsProgress: true)
        .catalogRefreshable(.title(slug)) {
            let view = core.title(slug)
            return view.status == .stale && view.problem == nil
        }
        #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            .toolbarBackgroundVisibility(.hidden, for: .navigationBar)
        #endif
    }
}

private struct TitleContent: View {
    let detail: TitleDetailView
    let problem: Problem?

    @Environment(CoreRuntime.self) private var core
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var season: UInt32?

    private var series: Bool { detail.kind == .series }

    private var backdropHeight: CGFloat {
        #if os(tvOS)
            1080
        #else
            sizeClass == .regular ? 520 : 340
        #endif
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Idiom.isTV ? 56 : Tokens.Spacing.xl) {
                header
                if problem != nil {
                    StaleNote(problem: problem)
                }
                if series, !detail.seasons.isEmpty {
                    SeasonsSection(detail: detail, season: $season)
                }
            }
            .padding(.bottom, Idiom.isTV ? 80 : Tokens.Spacing.xxxl)
        }
        .scrollIndicators(.hidden)
        .background(alignment: .top) { backdrop }
        .ignoresSafeArea(edges: .top)
        .accentIfAny(detail.accent)
    }

    /// Full bleed behind the whole page on TV, a header on touch devices; it fades into the page.
    private var backdrop: some View {
        ArtworkImage(image: detail.backdrop)
            .frame(height: backdropHeight)
            .frame(maxWidth: .infinity)
            .overlay {
                LinearGradient(
                    stops: [
                        .init(color: Tokens.Palette.bg.opacity(Idiom.isTV ? 0.2 : 0), location: 0),
                        .init(color: Tokens.Palette.bg.opacity(0.7), location: 0.6),
                        .init(color: Tokens.Palette.bg, location: 1),
                    ], startPoint: .top, endPoint: .bottom)
            }
            .overlay {
                if Idiom.isTV {
                    LinearGradient(
                        colors: [Tokens.Palette.bg.opacity(0.9), .clear], startPoint: .leading, endPoint: .trailing)
                }
            }
            .ignoresSafeArea()
            .accessibilityHidden(true)
    }

    private var header: some View {
        HStack(alignment: .bottom, spacing: Tokens.Spacing.xl) {
            if sizeClass == .regular && !Idiom.isTV {
                ArtworkImage(image: detail.poster)
                    .frame(width: 200, height: 300)
                    .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
                    .shadow(color: .black.opacity(0.5), radius: 24, y: 12)
            }
            VStack(alignment: .leading, spacing: Idiom.isTV ? Tokens.Spacing.xl : Tokens.Spacing.md) {
                TitleLogo(
                    logo: detail.logo, name: detail.name, maxWidth: Idiom.isTV ? 760 : 320,
                    maxHeight: Idiom.isTV ? 220 : 110)
                TitleFacts(detail: detail)
                if !detail.overview.isEmpty {
                    Text(detail.overview)
                        .typeRole(Tokens.TypeRamp.body)
                        .foregroundStyle(Tokens.Palette.text.opacity(0.85))
                        .lineLimit(Idiom.isTV ? 4 : nil)
                        .fixedSize(horizontal: false, vertical: !Idiom.isTV)
                        .frame(maxWidth: Idiom.isTV ? 1000 : 640, alignment: .leading)
                }
                if !detail.genres.isEmpty {
                    Text(detail.genres.joined(separator: " \u{00B7} "))
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.faintText)
                }
                TitleActions(detail: detail)
                    .padding(.top, Tokens.Spacing.sm)
            }
        }
        .padding(.horizontal, CardMetrics.edge)
        .padding(.top, Idiom.isTV ? 360 : backdropHeight * 0.55)
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// Year, seasons or length, rating, quality and HDR.
private struct TitleFacts: View {
    let detail: TitleDetailView

    var body: some View {
        let length =
            detail.kind == .series
            ? L10n.catalogSeasonCount(count: detail.seasons.count)
            : detail.runtimeMinutes.map(CatalogLabels.runtime(minutes:))
        let text = [detail.year.map(String.init), length].compactMap { $0 }.joined(separator: " \u{00B7} ")
        // one line where it fits, the badges below the text at large sizes, wrapping as a group
        ViewThatFits(in: .horizontal) {
            HStack(spacing: Tokens.Spacing.sm) {
                Text(text).foregroundStyle(Tokens.Palette.mutedText)
                badges
            }
            VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                Text(text).foregroundStyle(Tokens.Palette.mutedText)
                FlowLayout(spacing: Tokens.Spacing.sm) { badges }
            }
        }
        .typeRole(Tokens.TypeRamp.caption)
        .accessibilityElement(children: .combine)
    }

    @ViewBuilder private var badges: some View {
        if let rating = detail.contentRating {
            FactBadge(text: rating)
        }
        if let quality = detail.quality {
            FactBadge(text: CatalogLabels.quality(quality))
        }
        if detail.hdr {
            FactBadge(text: L10n.catalogHdr)
        }
    }
}

private struct FactBadge: View {
    let text: String

    var body: some View {
        Text(text)
            .foregroundStyle(Tokens.Palette.text)
            .padding(.horizontal, Tokens.Spacing.sm)
            .padding(.vertical, Tokens.Spacing.xxs)
            .overlay { RoundedRectangle(cornerRadius: 4).strokeBorder(Tokens.Palette.mutedText.opacity(0.6)) }
    }
}

/// Rows of views at their own width, left to right, starting a new row where the next one would
/// not fit: the badges, which never break inside.
private struct FlowLayout: Layout {
    let spacing: CGFloat

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let rows = rows(subviews, width: proposal.width ?? .infinity)
        return CGSize(
            width: rows.map(\.width).max() ?? 0,
            height: rows.map(\.height).reduce(0, +) + CGFloat(max(rows.count - 1, 0)) * spacing)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        var y = bounds.minY
        for row in rows(subviews, width: bounds.width) {
            var x = bounds.minX
            for index in row.items {
                let size = subviews[index].sizeThatFits(.unspecified)
                subviews[index].place(
                    at: CGPoint(x: x, y: y + (row.height - size.height) / 2), proposal: ProposedViewSize(size))
                x += size.width + spacing
            }
            y += row.height + spacing
        }
    }

    private struct Row {
        var items: [Int] = []
        var width: CGFloat = 0
        var height: CGFloat = 0
    }

    private func rows(_ subviews: Subviews, width: CGFloat) -> [Row] {
        var rows: [Row] = []
        var row = Row()
        for (index, subview) in subviews.enumerated() {
            let size = subview.sizeThatFits(.unspecified)
            if !row.items.isEmpty, row.width + spacing + size.width > width {
                rows.append(row)
                row = Row()
            }
            row.width += (row.items.isEmpty ? 0 : spacing) + size.width
            row.height = max(row.height, size.height)
            row.items.append(index)
        }
        if !row.items.isEmpty {
            rows.append(row)
        }
        return rows
    }
}

/// Play or Resume, My List, a random episode and a movie's download (iPhone and iPad); stacked when
/// the text is too large for a row.
private struct TitleActions: View {
    let detail: TitleDetailView
    @Environment(CoreRuntime.self) private var core
    @FocusState private var playFocused: Bool

    var body: some View {
        // a row; Play above the rest; at the largest text sizes one under another, wrapping
        ViewThatFits(in: .horizontal) {
            HStack(spacing: Tokens.Spacing.md) {
                play.fixedSize()
                secondary(fixed: true)
            }
            VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                play.fixedSize()
                HStack(spacing: Tokens.Spacing.md) { secondary(fixed: true) }
            }
            VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                play
                secondary(fixed: false)
            }
        }
        .tvFocusSection()
        .defaultFocus($playFocused, true)
    }

    private var play: some View {
        Button {
            if let play = detail.play {
                core.send(.playRequested(play.target))
            }
        } label: {
            Label(
                detail.play.map { CatalogLabels.playLabel($0, kind: detail.kind) } ?? L10n.commonPlay,
                systemImage: "play.fill")
        }
        .primaryAction()
        .disabled(detail.play == nil)
        .focused($playFocused)
        .accessibilityIdentifier("title-play")
    }

    @ViewBuilder private func secondary(fixed: Bool) -> some View {
        MyListButton(titleId: detail.id, inList: detail.inList, labelled: true)
            .fixedWidth(fixed)
        if detail.shuffle {
            Button(action: playRandom) {
                Label(L10n.catalogRandomEpisode, systemImage: "shuffle")
            }
            .secondaryAction()
            .fixedWidth(fixed)
        }
        #if os(iOS)
            if detail.kind == .movie, let play = detail.play {
                DownloadButton(target: play.target, labelled: true)
                    .fixedWidth(fixed)
            }
        #endif
    }

    /// A random episode, with shuffle switched on so the player keeps picking at random.
    private func playRandom() {
        guard let episode = detail.seasons.flatMap(\.episodes).randomElement() else { return }
        if !core.player.shuffle {
            core.send(.shuffleToggled)
        }
        core.send(.playRequested(PlayTarget(kind: .episode, id: episode.id)))
    }
}

/// A series' seasons: a picker when there is more than one, then the season's episodes as a list
/// on touch devices and a shelf of stills on TV.
private struct SeasonsSection: View {
    let detail: TitleDetailView
    @Binding var season: UInt32?

    private var current: SeasonView {
        detail.seasons.first { $0.number == season } ?? detail.seasons[0]
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.lg) {
            HStack(alignment: .firstTextBaseline, spacing: Tokens.Spacing.lg) {
                Text(L10n.catalogEpisodes)
                    .typeRole(Tokens.TypeRamp.section)
                    .foregroundStyle(Tokens.Palette.text)
                    .accessibilityAddTraits(.isHeader)
                Spacer(minLength: 0)
                if detail.seasons.count > 1 && !Idiom.isTV {
                    Picker(L10n.catalogSeason, selection: selection) {
                        ForEach(detail.seasons, id: \.number) { season in
                            Text(Self.name(season)).tag(season.number)
                        }
                    }
                    .pickerStyle(.menu)
                    .fixedSize()
                }
            }
            .padding(.horizontal, CardMetrics.edge)
            #if os(tvOS)
                if detail.seasons.count > 1 {
                    SeasonTabs(seasons: detail.seasons, selection: selection)
                }
                EpisodeShelf(season: current)
            #else
                LazyVStack(alignment: .leading, spacing: Tokens.Spacing.lg) {
                    ForEach(current.episodes, id: \.id) { episode in
                        EpisodeRow(episode: episode)
                    }
                }
                .padding(.horizontal, CardMetrics.edge)
            #endif
        }
    }

    private var selection: Binding<UInt32> {
        Binding(get: { current.number }, set: { season = $0 })
    }

    static func name(_ season: SeasonView) -> String {
        season.name.isEmpty ? L10n.catalogSeasonNumber(number: String(season.number)) : season.name
    }
}

#if os(tvOS)
    /// The seasons as a row of buttons; focusing one shows its episodes below.
    private struct SeasonTabs: View {
        let seasons: [SeasonView]
        let selection: Binding<UInt32>

        var body: some View {
            ScrollView(.horizontal) {
                HStack(spacing: Tokens.Spacing.lg) {
                    ForEach(seasons, id: \.number) { season in
                        Button {
                            selection.wrappedValue = season.number
                        } label: {
                            Text(SeasonsSection.name(season))
                        }
                        .buttonStyle(.bordered)
                        .tint(selection.wrappedValue == season.number ? Tokens.Palette.text : nil)
                        .accessibilityAddTraits(selection.wrappedValue == season.number ? .isSelected : [])
                    }
                }
                .padding(.horizontal, CardMetrics.edge)
                .padding(.vertical, Tokens.Spacing.lg)
            }
            .scrollClipDisabled()
            .focusSection()
        }
    }

    private struct EpisodeShelf: View {
        let season: SeasonView

        var body: some View {
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: CardMetrics.spacing) {
                    ForEach(season.episodes, id: \.id) { episode in
                        EpisodeCard(episode: episode)
                    }
                }
                .padding(.horizontal, CardMetrics.edge)
                .padding(.vertical, 24)
            }
            .scrollClipDisabled()
            .focusSection()
        }
    }

    /// An episode's still with its progress; the card style lifts it with parallax.
    private struct EpisodeCard: View {
        let episode: EpisodeView
        @Environment(CoreRuntime.self) private var core

        var body: some View {
            let width: CGFloat = 400
            VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                Button {
                    core.send(.playRequested(PlayTarget(kind: .episode, id: episode.id)))
                } label: {
                    EpisodeStill(episode: episode)
                        .frame(width: width, height: width * 9 / 16)
                }
                .buttonStyle(.card)
                .accessibilityLabel(EpisodeLabels.name(episode))
                .accessibilityValue(
                    [EpisodeLabels.length(episode), EpisodeLabels.progress(episode)].compactMap { $0 }
                        .joined(separator: ", "))
                VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
                    Text("\(episode.number). \(EpisodeLabels.name(episode))")
                        .typeRole(Tokens.TypeRamp.card)
                        .foregroundStyle(Tokens.Palette.text)
                        .lineLimit(1)
                    if let length = EpisodeLabels.length(episode) {
                        Text(length)
                            .typeRole(Tokens.TypeRamp.caption)
                            .foregroundStyle(Tokens.Palette.mutedText)
                    }
                }
                .frame(width: width, alignment: .leading)
                .accessibilityHidden(true)
            }
        }
    }
#else
    /// An episode as a row: its still with progress, number and name, length and overview, and
    /// its download beside it.
    private struct EpisodeRow: View {
        let episode: EpisodeView
        @Environment(CoreRuntime.self) private var core
        @Environment(\.dynamicTypeSize) private var dynamicTypeSize
        @Environment(\.horizontalSizeClass) private var sizeClass

        var body: some View {
            HStack(spacing: Tokens.Spacing.sm) {
                row
                DownloadButton(target: PlayTarget(kind: .episode, id: episode.id))
            }
        }

        private var row: some View {
            Button {
                core.send(.playRequested(PlayTarget(kind: .episode, id: episode.id)))
            } label: {
                let layout =
                    dynamicTypeSize.isAccessibilitySize
                    ? AnyLayout(VStackLayout(alignment: .leading, spacing: Tokens.Spacing.md))
                    : AnyLayout(HStackLayout(alignment: .top, spacing: Tokens.Spacing.md))
                layout {
                    let width: CGFloat = sizeClass == .regular ? 220 : 150
                    EpisodeStill(episode: episode)
                        .frame(width: width, height: width * 9 / 16)
                    VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
                        Text("\(episode.number). \(EpisodeLabels.name(episode))")
                            .typeRole(Tokens.TypeRamp.card)
                            .foregroundStyle(Tokens.Palette.text)
                            .lineLimit(2)
                        if let length = EpisodeLabels.length(episode) {
                            Text(length)
                                .typeRole(Tokens.TypeRamp.caption)
                                .foregroundStyle(Tokens.Palette.mutedText)
                        }
                        if !episode.overview.isEmpty {
                            Text(episode.overview)
                                .typeRole(Tokens.TypeRamp.caption)
                                .foregroundStyle(Tokens.Palette.faintText)
                                .lineLimit(3)
                        }
                    }
                    .multilineTextAlignment(.leading)
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityElement(children: .combine)
            .accessibilityAddTraits(.isButton)
            .accessibilityValue(EpisodeLabels.progress(episode) ?? "")
        }
    }
#endif

/// The still with how far in and a play glyph.
private struct EpisodeStill: View {
    let episode: EpisodeView

    var body: some View {
        ArtworkImage(image: episode.still)
            .overlay(alignment: .bottom) {
                if episode.progress > 0 {
                    ProgressBar(fraction: episode.completed ? 1 : episode.progress)
                        .padding(Tokens.Spacing.sm)
                }
            }
            .overlay(alignment: .topTrailing) {
                if episode.completed {
                    Image(systemName: "checkmark.circle.fill")
                        .font(.system(size: Idiom.isTV ? 32 : 18))
                        .foregroundStyle(.white, .black.opacity(0.4))
                        .padding(Tokens.Spacing.sm)
                        .accessibilityHidden(true)
                }
            }
            .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
    }
}

enum EpisodeLabels {
    static func name(_ episode: EpisodeView) -> String {
        episode.name.isEmpty ? L10n.catalogEpisodeNumber(number: String(episode.number)) : episode.name
    }

    static func length(_ episode: EpisodeView) -> String? {
        let minutes = episode.runtimeMinutes ?? episode.durationSeconds.map { UInt32(($0 + 30) / 60) }
        return minutes.flatMap { $0 > 0 ? CatalogLabels.runtime(minutes: $0) : nil }
    }

    /// How far in, as the still's bar and check show it; nil before it is started.
    static func progress(_ episode: EpisodeView) -> String? {
        if episode.completed {
            return L10n.catalogWatched
        }
        let percent = Int((min(max(episode.progress, 0), 1) * 100).rounded())
        return percent > 0 ? L10n.catalogProgress(percent: String(percent)) : nil
    }
}

/// A title page's shape while it first loads.
private struct TitleSkeleton: View {
    var body: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.lg) {
            Skeleton(height: Idiom.isTV ? 420 : 260, cornerRadius: 0)
            VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                Skeleton(width: Idiom.isTV ? 560 : 240, height: Idiom.isTV ? 110 : 56)
                Skeleton(width: Idiom.isTV ? 360 : 180, height: 14)
                Skeleton(height: 14)
                Skeleton(width: Idiom.isTV ? 700 : 260, height: 14)
                HStack(spacing: Tokens.Spacing.md) {
                    Skeleton(
                        width: Idiom.isTV ? 260 : 150, height: Idiom.isTV ? 66 : 48, cornerRadius: Tokens.Radius.pill)
                    Skeleton(
                        width: Idiom.isTV ? 220 : 120, height: Idiom.isTV ? 66 : 48, cornerRadius: Tokens.Radius.pill)
                }
            }
            .padding(.horizontal, CardMetrics.edge)
            .frame(maxWidth: Idiom.isTV ? 1100 : .infinity, alignment: .leading)
        }
        .ignoresSafeArea(edges: .top)
        .accessibilityElement()
        .accessibilityLabel(L10n.commonLoading)
    }
}

extension View {
    /// Tints the subtree with a palette when the content has one of its own.
    @ViewBuilder func accentIfAny(_ palette: AccentPalette?) -> some View {
        if let palette {
            accent(palette)
        } else {
            self
        }
    }
}
