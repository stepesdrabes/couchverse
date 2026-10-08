import CouchverseCore
import CouchverseDesign
import SwiftUI

/// Who leads by XP, watch time or achievements, over all time, this month or this week (XP only
/// ever counts for all time). The top three stand on a podium when they earned something, every
/// member below is a row leading to their profile, and the viewer's own place stays in view.
struct LeaderboardScreen: View {
    @Environment(CoreRuntime.self) private var core
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @State private var metric = Metric.xp
    @State private var period = Period.all

    init(metric: Metric = .xp, period: Period = .all) {
        _metric = State(initialValue: metric)
        _period = State(initialValue: period)
    }

    /// The board on screen; XP is lifetime whatever the period, so it is always the all-time one.
    private var key: LeaderboardKey {
        LeaderboardKey(period: metric == .xp ? .all : period, metric: metric)
    }

    var body: some View {
        let view = core.leaderboard(key)
        ScrollView {
            VStack(alignment: .leading, spacing: Idiom.isTV ? 40 : Tokens.Spacing.xl) {
                VStack(alignment: .leading, spacing: Idiom.isTV ? 40 : Tokens.Spacing.xl) {
                    #if os(tvOS)
                        Text(L10n.leaderboardHeading)
                            .typeRole(Tokens.TypeRamp.title)
                            .foregroundStyle(Tokens.Palette.text)
                            .accessibilityAddTraits(.isHeader)
                    #endif
                    switchers
                    if view.hidden {
                        HiddenNotice()
                    }
                    #if os(tvOS)
                        if let me = view.me, view.status != .loading {
                            MyPlace(row: me, view: view)
                        }
                    #endif
                }
                .padding(.horizontal, CardMetrics.edge)
                if view.status == .stale {
                    StaleNote(problem: view.problem)
                }
                CatalogStateView(
                    status: view.status, hasContent: !view.rows.isEmpty, problem: view.problem,
                    surface: .leaderboard(key)
                ) {
                    board(view)
                } skeleton: {
                    LeaderboardSkeleton()
                }
                .padding(.horizontal, CardMetrics.edge)
            }
            .padding(.vertical, Idiom.isTV ? 40 : Tokens.Spacing.lg)
            .readableWidth(Idiom.isTV ? 1400 : 720)
        }
        .scrollIndicators(.hidden)
        .background(Tokens.Palette.bg)
        .coreScreen(.leaderboard(key))
        .catalogRefreshable(.leaderboard(key)) {
            let view = core.leaderboard(key)
            return view.status == .stale && view.problem == nil
        }
        #if os(iOS)
            .safeAreaInset(edge: .bottom) { stickyPlace(view) }
            .navigationTitle(L10n.leaderboardHeading)
        #endif
        .closesWithoutRankings()
        .selectionHaptic(trigger: key)
    }

    /// The viewer's own place, kept in view under the list once it is off the podium or hidden.
    @ViewBuilder private func stickyPlace(_ view: LeaderboardView) -> some View {
        if let me = view.me, view.hidden || (view.myPosition ?? 0) > 3 {
            MyPlace(row: me, view: view)
                .padding(.horizontal, Tokens.Spacing.lg)
                .padding(.bottom, Tokens.Spacing.sm)
                .readableWidth(720)
        }
    }

    private var switchers: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
            Picker(L10n.leaderboardTableCaption(metric: RanksWords.metric(metric)), selection: $metric) {
                ForEach([Metric.xp, .watch, .achievements], id: \.self) { metric in
                    Text(RanksWords.metric(metric)).tag(metric)
                }
            }
            .pickerStyle(.segmented)
            if metric == .xp {
                Label(L10n.leaderboardPeriodLocked, systemImage: "info.circle")
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.mutedText)
            } else {
                Picker(L10n.leaderboardPeriod, selection: $period) {
                    ForEach([Period.all, .month, .week], id: \.self) { period in
                        Text(RanksWords.period(period)).tag(period)
                    }
                }
                .pickerStyle(.segmented)
            }
        }
        .tvFocusSection()
    }

    @ViewBuilder private func board(_ view: LeaderboardView) -> some View {
        if view.rows.isEmpty {
            CatalogMessage(
                systemImage: "trophy", title: L10n.leaderboardEmptyTitle, message: L10n.leaderboardEmptyMessage)
        } else if view.allZero {
            CatalogMessage(
                systemImage: "trophy", title: L10n.leaderboardMetricEmpty, message: RanksWords.metric(view.key.metric))
        } else {
            VStack(alignment: .leading, spacing: Idiom.isTV ? 40 : Tokens.Spacing.xl) {
                // at the accessibility text sizes the rows below say the same without clipping names
                if view.podium, view.rows.count >= 3, Idiom.isTV || !dynamicTypeSize.isAccessibilitySize {
                    Podium(rows: Array(view.rows.prefix(3)), metric: view.key.metric)
                }
                LazyVStack(spacing: Idiom.isTV ? Tokens.Spacing.md : Tokens.Spacing.sm) {
                    ForEach(view.rows, id: \.username) { row in
                        LeaderRowView(row: row, metric: view.key.metric, medal: view.podium)
                    }
                }
                .tvFocusSection()
            }
        }
    }
}

/// The viewer opted out of leaderboards; privacy is changed in the profile editor.
private struct HiddenNotice: View {
    var body: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: Tokens.Spacing.md) { content }
            VStack(alignment: .leading, spacing: Tokens.Spacing.sm) { content }
        }
        .padding(Tokens.Spacing.lg)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Tokens.Palette.surface2, in: RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
    }

    @ViewBuilder private var content: some View {
        Label(L10n.leaderboardHiddenNotice, systemImage: "eye.slash")
            .typeRole(Tokens.TypeRamp.body)
            .foregroundStyle(Tokens.Palette.mutedText)
            .fixedSize(horizontal: false, vertical: true)
        Spacer(minLength: 0)
        NavigationLink(value: RanksRoute.editor) {
            Text(L10n.leaderboardHiddenAction)
        }
        .secondaryAction()
        .fixedSize()
    }
}

/// The top three on a podium: second, first and third from the left, read first to third.
private struct Podium: View {
    let rows: [LeaderRow]
    let metric: Metric

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.ambience) private var ambience
    @State private var risen = false

    var body: some View {
        HStack(alignment: .bottom, spacing: Idiom.isTV ? 48 : Tokens.Spacing.md) {
            place(1)
            place(0)
            place(2)
        }
        // the places' sort priorities read them first to third within the podium only
        .accessibilityElement(children: .contain)
        .frame(maxWidth: .infinity)
        .tvFocusSection()
        .onAppear {
            guard !risen else { return }
            if reduceMotion || ambience != .live {
                risen = true
            } else {
                withAnimation(Tokens.Motion.bouncy) { risen = true }
            }
        }
    }

    private func place(_ index: Int) -> some View {
        let row = rows[index]
        let metal = RanksStyle.podium[index]
        let avatar: CGFloat = Idiom.isTV ? (index == 0 ? 150 : 120) : (index == 0 ? 72 : 60)
        let plinth: CGFloat = (Idiom.isTV ? 2 : 1) * [96, 72, 56][index]
        return NavigationLink(value: RanksRoute.profile(username: row.username)) {
            VStack(spacing: Tokens.Spacing.sm) {
                Image(systemName: "crown.fill")
                    .foregroundStyle(metal.ring)
                    .opacity(index == 0 ? 1 : 0)
                    .accessibilityHidden(true)
                AvatarView(url: row.avatar?.url, seed: row.username, name: row.displayName)
                    .frame(width: avatar, height: avatar)
                    .padding(Idiom.isTV ? 6 : 3)
                    .background(
                        LinearGradient(
                            colors: [metal.from, metal.to], startPoint: .topLeading, endPoint: .bottomTrailing),
                        in: Circle())
                Text(row.displayName)
                    .typeRole(Tokens.TypeRamp.card)
                    .foregroundStyle(Tokens.Palette.text)
                    .lineLimit(1)
                Text(RanksWords.value(row.value, metric: metric))
                    .typeRole(Tokens.TypeRamp.card)
                    .foregroundStyle(metal.ring)
                    .monospacedDigit()
                    .lineLimit(1)
                    .minimumScaleFactor(0.7)
                Text(String(index + 1))
                    .typeRole(Tokens.TypeRamp.title)
                    .foregroundStyle(Tokens.Palette.faintText)
                    .frame(maxWidth: .infinity)
                    .frame(minHeight: plinth, alignment: .top)
                    .padding(.top, Tokens.Spacing.sm)
                    .background(
                        LinearGradient(
                            colors: [metal.from.opacity(0.35), metal.from.opacity(0)], startPoint: .top,
                            endPoint: .bottom),
                        in: UnevenRoundedRectangle(
                            topLeadingRadius: Tokens.Radius.card, topTrailingRadius: Tokens.Radius.card)
                    )
                    .overlay(alignment: .top) { Rectangle().fill(metal.ring).frame(height: 2) }
                    .scaleEffect(y: risen ? 1 : 0.01, anchor: .bottom)
                    .accessibilityHidden(true)
            }
            .frame(width: Idiom.isTV ? 280 : 104)
        }
        .cardButtonStyle()
        .accessibilityElement(children: .combine)
        .accessibilityLabel(String(index + 1) + ". " + row.displayName)
        .accessibilityValue(RanksWords.value(row.value, metric: metric))
        .accessibilitySortPriority(Double(3 - index))
    }
}

/// A member's place, avatar, name and rank, and their value on the board; the viewer's own is
/// marked in words as well as in colour.
private struct LeaderRowView: View {
    let row: LeaderRow
    let metric: Metric
    /// The top three wear their medal's colour.
    let medal: Bool

    @Environment(\.accent) private var accent

    var body: some View {
        NavigationLink(value: RanksRoute.profile(username: row.username)) {
            LeaderLine(row: row, metric: metric, place: String(row.position), medal: medal)
                .padding(.horizontal, Tokens.Spacing.md)
                .padding(.vertical, Idiom.isTV ? Tokens.Spacing.md : Tokens.Spacing.sm)
                .background {
                    // a TV's card style draws each row's platter; the viewer's is tinted on both
                    if row.isSelf || !Idiom.isTV {
                        RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous)
                            .fill(row.isSelf ? accent.soft : Tokens.Palette.surface)
                    }
                }
                .contentShape(Rectangle())
        }
        .cardButtonStyle()
    }
}

private struct LeaderLine: View {
    let row: LeaderRow
    let metric: Metric
    let place: String
    let medal: Bool

    @ScaledMetric(relativeTo: .body) private var placeWidth: CGFloat = Idiom.isTV ? 64 : 36
    @ScaledMetric(relativeTo: .body) private var avatarSize: CGFloat = Idiom.isTV ? 64 : 36
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    /// At the accessibility text sizes the value goes under the name rather than squeezing it.
    private var stacked: Bool { dynamicTypeSize.isAccessibilitySize }

    var body: some View {
        let metal = medal && (1...3).contains(row.position) ? RanksStyle.podium[Int(row.position) - 1] : nil
        HStack(spacing: Tokens.Spacing.md) {
            Text(place)
                .typeRole(Tokens.TypeRamp.card)
                .foregroundStyle(metal?.ring ?? Tokens.Palette.mutedText)
                .monospacedDigit()
                .frame(width: placeWidth)
            AvatarView(url: row.avatar?.url, seed: row.username, name: row.displayName)
                .frame(width: avatarSize, height: avatarSize)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) {
                HStack(spacing: Tokens.Spacing.sm) {
                    Text(row.displayName)
                        .typeRole(Tokens.TypeRamp.card)
                        .foregroundStyle(Tokens.Palette.text)
                        .lineLimit(stacked ? 2 : 1)
                    if row.isSelf {
                        Text(L10n.leaderboardYou)
                            .typeRole(Tokens.TypeRamp.caption)
                            .foregroundStyle(Tokens.Palette.mutedText)
                    }
                }
                Text(RanksWords.rankLine(tier: row.tierCode, level: row.level))
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.mutedText)
                    .lineLimit(stacked ? 2 : 1)
                if stacked {
                    value
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            if !stacked {
                value
            }
        }
        .multilineTextAlignment(.leading)
        .accessibilityElement(children: .combine)
    }

    private var value: some View {
        Text(RanksWords.value(row.value, metric: metric))
            .typeRole(Tokens.TypeRamp.card)
            .foregroundStyle(Tokens.Palette.text)
            .monospacedDigit()
            .lineLimit(1)
    }
}

/// Where the viewer stands: their place of how many, or that they are hidden.
private struct MyPlace: View {
    let row: LeaderRow
    let view: LeaderboardView

    @Environment(\.accent) private var accent
    @Environment(\.ambience) private var ambience

    var body: some View {
        let ranked = !view.hidden && row.position > 0
        VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
            Text(
                ranked
                    ? L10n.leaderboardYourPosition(rank: String(row.position), total: String(view.total))
                    : L10n.leaderboardUnranked
            )
            .typeRole(Tokens.TypeRamp.caption)
            .foregroundStyle(Tokens.Palette.mutedText)
            LeaderLine(row: row, metric: view.key.metric, place: ranked ? String(row.position) : "-", medal: false)
        }
        .padding(.horizontal, Tokens.Spacing.md)
        .padding(.vertical, Tokens.Spacing.sm)
        .placeSurface(solid: ambience == .flat || Idiom.isTV)
        .overlay {
            RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous)
                .strokeBorder(accent.color.opacity(0.5))
        }
        .accessibilityElement(children: .combine)
    }
}

extension View {
    /// Glass over the list where it can render; solid on TV and in snapshots, which have no host
    /// app to draw glass in.
    @ViewBuilder fileprivate func placeSurface(solid: Bool) -> some View {
        let shape = RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous)
        if solid {
            background(Tokens.Palette.surface2, in: shape)
        } else {
            glassEffect(.regular, in: shape)
        }
    }
}

/// A board's shape while it first loads.
private struct LeaderboardSkeleton: View {
    var body: some View {
        VStack(spacing: Tokens.Spacing.md) {
            ForEach(0..<8, id: \.self) { _ in
                HStack(spacing: Tokens.Spacing.md) {
                    Skeleton(width: Idiom.isTV ? 48 : 24, height: 16)
                    Skeleton(circle: Idiom.isTV ? 64 : 36)
                    VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                        Skeleton(width: Idiom.isTV ? 320 : 140, height: 14)
                        Skeleton(width: Idiom.isTV ? 220 : 100, height: 10)
                    }
                    Spacer()
                    Skeleton(width: Idiom.isTV ? 140 : 60, height: 14)
                }
            }
        }
        .accessibilityElement()
        .accessibilityLabel(L10n.commonLoading)
    }
}
