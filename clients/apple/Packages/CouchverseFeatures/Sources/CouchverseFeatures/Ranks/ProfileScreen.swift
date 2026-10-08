import CouchverseCore
import CouchverseDesign
import SwiftUI

/// A member's profile (plan 10.4): the avatar in the rank ring over their banner, name, handle and
/// when they joined, the XP towards the next level, the bio, their numbers, the last 26 weeks of
/// activity, the hours they watch, what they watch most, achievements by category and where the
/// XP comes from. Your own adds Edit profile and says when it is private.
struct ProfileScreen: View {
    let username: String
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        let view = core.profile(username)
        ScrollView {
            CatalogStateView(
                status: view.status, hasContent: view.profile != nil, problem: view.problem,
                surface: .profile(username), notFound: L10n.profilesNotFound
            ) {
                if let profile = view.profile {
                    ProfileContent(profile: profile, problem: view.status == .stale ? view.problem : nil)
                }
            } skeleton: {
                ProfileSkeleton()
            }
        }
        .scrollIndicators(.hidden)
        .background(Tokens.Palette.bg)
        .coreScreen(.profile(username))
        .catalogRefreshable(.profile(username)) {
            let view = core.profile(username)
            return view.status == .stale && view.problem == nil
        }
        .closesWithoutRankings()
        .navigationBarTitleDisplayModeInline()
        .onAppear {
            // your achievements are on show: anything earned since the last check (a tenth title
            // on My List) unlocks now; the core throttles it
            if username == core.session.user?.username {
                core.send(.achievementsCheckRequested(CheckRequest(force: false)))
            }
        }
    }
}

private struct ProfileContent: View {
    let profile: ProfileDetail
    let problem: Problem?

    @Environment(\.horizontalSizeClass) private var sizeClass

    private var wide: Bool { Idiom.isTV || sizeClass == .regular }

    var body: some View {
        VStack(alignment: .leading, spacing: Idiom.isTV ? 56 : Tokens.Spacing.xxl) {
            ProfileHero(profile: profile)
            if problem != nil {
                StaleNote(problem: problem)
            }
            Group {
                if !profile.bio.blocks.isEmpty {
                    ReadingBlock(title: nil) {
                        MarkdownView(profile.bio)
                            .foregroundStyle(Tokens.Palette.text)
                    }
                }
                ReadingBlock(title: nil) { StatTiles(profile: profile) }
                charts
            }
            .padding(.horizontal, CardMetrics.edge)
            if !profile.topTitles.isEmpty {
                TopTitlesShelf(titles: profile.topTitles)
            }
            AchievementsSection(cards: profile.achievements, won: profile.achievementsWon)
                .padding(.horizontal, CardMetrics.edge)
            if !profile.xpSources.isEmpty {
                ReadingBlock(
                    title: L10n.rankSourcesHeading, detail: L10n.rankXpValue(xp: RanksWords.number(profile.xpTotal))
                ) {
                    XPSources(lines: profile.xpSources, total: profile.xpTotal)
                }
                .padding(.horizontal, CardMetrics.edge)
            }
        }
        .padding(.bottom, Idiom.isTV ? 80 : Tokens.Spacing.xxxl)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    /// The heatmap and the clock, side by side where there is room.
    @ViewBuilder private var charts: some View {
        let activity = ReadingBlock(title: L10n.profilesActivityHeading) {
            if profile.heatmap.activeDays == 0 {
                Text(L10n.profilesNoActivity)
                    .typeRole(Tokens.TypeRamp.body)
                    .foregroundStyle(Tokens.Palette.muted)
            } else {
                HeatmapView(heatmap: profile.heatmap)
            }
        }
        let clock = ReadingBlock(title: L10n.profilesClockHeading) {
            if profile.hours.allSatisfy({ $0 == 0 }) {
                Text(L10n.profilesClockEmpty)
                    .typeRole(Tokens.TypeRamp.body)
                    .foregroundStyle(Tokens.Palette.muted)
            } else {
                WatchClockView(hours: profile.hours)
                    .frame(maxWidth: .infinity)
            }
        }
        if wide {
            HStack(alignment: .top, spacing: Idiom.isTV ? 48 : Tokens.Spacing.xl) {
                activity.layoutPriority(1)
                clock.frame(maxWidth: Idiom.isTV ? 520 : 300)
            }
        } else {
            activity
            clock
        }
    }
}

/// The avatar in the rank ring, who they are and how far into their tier, over their banner (or a
/// glow in the banner's or the tier's colour).
private struct ProfileHero: View {
    let profile: ProfileDetail

    @Environment(CoreRuntime.self) private var core
    @Environment(\.horizontalSizeClass) private var sizeClass

    private var wide: Bool { Idiom.isTV || sizeClass == .regular }
    private var tierColor: Color { RanksStyle.tier(profile.rank.tier.code) }
    private var avatarSize: CGFloat { Idiom.isTV ? 200 : wide ? 140 : 104 }

    var body: some View {
        let layout =
            wide
            ? AnyLayout(HStackLayout(alignment: .center, spacing: Idiom.isTV ? 64 : Tokens.Spacing.xxl))
            : AnyLayout(VStackLayout(alignment: .center, spacing: Tokens.Spacing.lg))
        layout {
            RankRing(
                color: tierColor, progress: Double(profile.rank.percent) / 100,
                level: String(profile.rank.tier.level), lineWidth: Idiom.isTV ? 10 : 6,
                flashes: profile.isSelf ? core.rank.levelUps : 0
            ) {
                AvatarView(url: profile.avatar?.url, seed: profile.username, name: profile.displayName)
                    .frame(width: avatarSize, height: avatarSize)
            }
            details
        }
        .frame(maxWidth: .infinity, alignment: wide ? .leading : .center)
        .padding(.horizontal, CardMetrics.edge)
        .padding(.top, Idiom.isTV ? 80 : Tokens.Spacing.xxl)
        .background(alignment: .top) { backdrop }
    }

    private var details: some View {
        VStack(alignment: wide ? .leading : .center, spacing: Tokens.Spacing.sm) {
            Text(RanksWords.tierName(profile.rank.tier.code))
                .typeRole(Tokens.TypeRamp.eyebrow)
                .textCase(.uppercase)
                .foregroundStyle(tierColor)
            Text(profile.displayName)
                .typeRole(Tokens.TypeRamp.title)
                .foregroundStyle(Tokens.Palette.text)
                .accessibilityAddTraits(.isHeader)
            Text(identity)
                .typeRole(Tokens.TypeRamp.caption)
                .foregroundStyle(Tokens.Palette.muted)
            XPProgress(badge: profile.rank, total: profile.xpTotal, color: tierColor)
                .frame(maxWidth: wide ? 560 : .infinity)
                .padding(.top, Tokens.Spacing.sm)
            if profile.isSelf {
                OwnProfileActions(profile: profile)
                    .padding(.top, Tokens.Spacing.sm)
            }
        }
        .multilineTextAlignment(wide ? .leading : .center)
    }

    /// `@nora · Joined 3 Jan 2025`.
    private var identity: String {
        let joined = RanksWords.date(profile.memberSince).map(L10n.profilesJoined(date:))
        return ["@\(profile.username)", joined].compactMap { $0 }.joined(separator: " \u{00B7} ")
    }

    private var backdrop: some View {
        let tint = profile.banner?.accent.flatMap(Color.init(hex:)) ?? tierColor
        return ZStack {
            if let banner = profile.banner {
                ArtworkImage(image: banner)
                    .opacity(0.45)
            } else {
                RadialGradient(
                    colors: [tint.opacity(0.35), tint.opacity(0)], center: .topLeading, startRadius: 0,
                    endRadius: Idiom.isTV ? 900 : 420)
            }
            LinearGradient(
                colors: [Tokens.Palette.bg.opacity(0.2), Tokens.Palette.bg], startPoint: .top, endPoint: .bottom)
        }
        .frame(height: Idiom.isTV ? 520 : wide ? 360 : 300)
        .frame(maxWidth: .infinity)
        .ignoresSafeArea(edges: .horizontal)
        .accessibilityHidden(true)
    }
}

/// The XP in all, the level and the bar towards the next one.
private struct XPProgress: View {
    let badge: RankBadge
    let total: UInt64
    let color: Color

    var body: some View {
        let progress = RanksWords.progress(badge)
        VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
            HStack(alignment: .firstTextBaseline, spacing: Tokens.Spacing.sm) {
                Text(RanksWords.number(total))
                    .typeRole(Tokens.TypeRamp.title)
                    .foregroundStyle(Tokens.Palette.text)
                    .monospacedDigit()
                Text(L10n.rankXp)
                    .typeRole(Tokens.TypeRamp.card)
                    .foregroundStyle(Tokens.Palette.muted)
                Spacer(minLength: Tokens.Spacing.md)
                Text(L10n.rankLevel(level: String(badge.tier.level)))
                    .typeRole(Tokens.TypeRamp.card)
                    .foregroundStyle(Tokens.Palette.text)
                    .monospacedDigit()
            }
            XPBar(fraction: Double(badge.percent) / 100, color: color)
            ViewThatFits(in: .horizontal) {
                HStack {
                    Text(progress.into)
                    Spacer(minLength: Tokens.Spacing.md)
                    Text(progress.next)
                }
                VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) {
                    Text(progress.into)
                    Text(progress.next)
                }
            }
            .typeRole(Tokens.TypeRamp.caption)
            .foregroundStyle(Tokens.Palette.muted)
            .monospacedDigit()
        }
        .multilineTextAlignment(.leading)
        .accessibilityElement(children: .combine)
    }
}

/// Edit profile, and the way back to public when the profile is private.
private struct OwnProfileActions: View {
    let profile: ProfileDetail
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
            NavigationLink(value: RanksRoute.editor) {
                Label(L10n.profilesEditProfile, systemImage: "pencil")
            }
            .secondaryAction()
            .fixedSize()
            .accessibilityIdentifier("edit-profile")
            if !profile.public {
                VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                    Label(L10n.profilesPrivateSelfNotice, systemImage: "eye.slash")
                        .typeRole(Tokens.TypeRamp.caption)
                        .foregroundStyle(Tokens.Palette.muted)
                        .fixedSize(horizontal: false, vertical: true)
                    Button(L10n.profilesMakePublic) {
                        core.send(.profileVisibilityChanged(PublicChoice(public: true)))
                    }
                    .secondaryAction()
                    .fixedSize()
                }
            }
        }
    }
}

/// Watch time, titles and episodes finished, the streaks, the top genre and, with the couch on,
/// the couches hosted.
private struct StatTiles: View {
    let profile: ProfileDetail
    @Environment(CoreRuntime.self) private var core

    var body: some View {
        let totals = profile.totals
        let width: CGFloat = Idiom.isTV ? 300 : 150
        LazyVGrid(
            columns: [GridItem(.adaptive(minimum: width), spacing: Tokens.Spacing.md)], alignment: .leading,
            spacing: Tokens.Spacing.md
        ) {
            StatTile(
                systemImage: "clock", value: RanksWords.watchTime(seconds: totals.watchSeconds),
                label: L10n.profilesStatWatchTime)
            StatTile(
                systemImage: "checkmark.circle",
                value: RanksWords.number(totals.moviesCompleted + totals.seriesCompleted),
                label: L10n.profilesStatTitles)
            StatTile(
                systemImage: "list.and.film", value: RanksWords.number(totals.episodesCompleted),
                label: L10n.profilesStatEpisodes)
            StatTile(
                systemImage: "flame", value: L10n.profilesStreakDays(count: Int(totals.longestStreak)),
                label: L10n.profilesStatLongestStreak)
            StatTile(
                systemImage: "calendar", value: L10n.profilesStreakDays(count: Int(totals.currentStreak)),
                label: L10n.profilesStatCurrentStreak)
            StatTile(
                systemImage: "tag", value: profile.favouriteGenre.isEmpty ? "-" : profile.favouriteGenre,
                label: L10n.profilesStatFavouriteGenre)
            if core.session.features.couch {
                StatTile(
                    systemImage: "sofa", value: RanksWords.number(totals.couchHosted), label: L10n.profilesStatCouch)
            }
        }
    }
}

/// What they watch most, as posters leading to the titles.
private struct TopTitlesShelf: View {
    let titles: [TopTitle]

    var body: some View {
        ShelfRow(title: L10n.profilesTopTitlesHeading) {
            ForEach(titles, id: \.slug) { title in
                TopTitleCard(title: title)
            }
        }
    }
}

private struct TopTitleCard: View {
    let title: TopTitle

    var body: some View {
        let width = CardMetrics.posterWidth
        VStack(alignment: .leading, spacing: CardMetrics.captionGap) {
            NavigationLink(value: CatalogRoute.title(slug: title.slug)) {
                ArtworkImage(image: title.poster)
                    .overlay(alignment: .bottomLeading) {
                        if title.poster == nil {
                            Text(title.name)
                                .typeRole(Tokens.TypeRamp.card)
                                .foregroundStyle(Tokens.Palette.text)
                                .padding(Tokens.Spacing.sm)
                        }
                    }
                    .frame(width: width, height: width * 1.5)
                    .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.card, style: .continuous))
            }
            .cardButtonStyle()
            .accessibilityLabel(title.name)
            .accessibilityValue(RanksWords.watchTime(seconds: title.seconds))
            VStack(alignment: .leading, spacing: Tokens.Spacing.xxs) {
                Text(title.name)
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.text)
                Text(RanksWords.watchTime(seconds: title.seconds))
                    .typeRole(Tokens.TypeRamp.caption)
                    .foregroundStyle(Tokens.Palette.muted)
            }
            .lineLimit(1)
            .frame(width: width, alignment: .leading)
            .accessibilityHidden(true)
        }
    }
}

/// Where the XP comes from, each source with its share of the whole.
private struct XPSources: View {
    let lines: [XpLine]
    let total: UInt64

    @Environment(\.accent) private var accent

    var body: some View {
        VStack(alignment: .leading, spacing: Idiom.isTV ? Tokens.Spacing.lg : Tokens.Spacing.md) {
            ForEach(lines, id: \.key) { line in
                VStack(alignment: .leading, spacing: Tokens.Spacing.xs) {
                    HStack(alignment: .firstTextBaseline) {
                        Text(RanksWords.xpSource(line.key))
                            .foregroundStyle(Tokens.Palette.text)
                        Spacer(minLength: Tokens.Spacing.md)
                        Text(L10n.rankXpValue(xp: RanksWords.number(line.xp)))
                            .foregroundStyle(Tokens.Palette.muted)
                            .monospacedDigit()
                    }
                    .typeRole(Tokens.TypeRamp.body)
                    XPBar(fraction: total == 0 ? 0 : Double(line.xp) / Double(total), color: accent.color)
                }
                .accessibilityElement(children: .combine)
            }
        }
        .frame(maxWidth: Idiom.isTV ? 900 : .infinity, alignment: .leading)
    }
}

/// A profile's shape while it first loads.
private struct ProfileSkeleton: View {
    var body: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
            HStack(spacing: Tokens.Spacing.xl) {
                Skeleton(circle: Idiom.isTV ? 220 : 116)
                VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                    Skeleton(width: Idiom.isTV ? 220 : 90, height: 12)
                    Skeleton(width: Idiom.isTV ? 480 : 180, height: Idiom.isTV ? 48 : 28)
                    Skeleton(width: Idiom.isTV ? 360 : 150, height: 12)
                    Skeleton(width: Idiom.isTV ? 560 : nil, height: 10, cornerRadius: Tokens.Radius.pill)
                }
            }
            .padding(.top, Idiom.isTV ? 80 : Tokens.Spacing.xxl)
            LazyVGrid(
                columns: [GridItem(.adaptive(minimum: Idiom.isTV ? 300 : 150), spacing: Tokens.Spacing.md)],
                spacing: Tokens.Spacing.md
            ) {
                ForEach(0..<6, id: \.self) { _ in
                    Skeleton(height: Idiom.isTV ? 180 : 96, cornerRadius: Tokens.Radius.card)
                }
            }
            Skeleton(height: Idiom.isTV ? 300 : 160, cornerRadius: Tokens.Radius.card)
        }
        .padding(.horizontal, CardMetrics.edge)
        .accessibilityElement()
        .accessibilityLabel(L10n.commonProcessing)
    }
}
