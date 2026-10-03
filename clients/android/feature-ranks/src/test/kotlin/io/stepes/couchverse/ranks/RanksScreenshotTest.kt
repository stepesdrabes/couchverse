package io.stepes.couchverse.ranks

import io.stepes.couchverse.core.AchievementCard
import io.stepes.couchverse.core.Block
import io.stepes.couchverse.core.HeatDay
import io.stepes.couchverse.core.Heatmap
import io.stepes.couchverse.core.Image
import io.stepes.couchverse.core.Inline
import io.stepes.couchverse.core.LeaderRow
import io.stepes.couchverse.core.LeaderboardKey
import io.stepes.couchverse.core.LeaderboardView
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.MarkdownDoc
import io.stepes.couchverse.core.Metric
import io.stepes.couchverse.core.Period
import io.stepes.couchverse.core.ProfileDetail
import io.stepes.couchverse.core.ProfileEditorView
import io.stepes.couchverse.core.ProfileTotals
import io.stepes.couchverse.core.ProfileView
import io.stepes.couchverse.core.RankBadge
import io.stepes.couchverse.core.SaveState
import io.stepes.couchverse.core.SessionUser
import io.stepes.couchverse.core.Tier
import io.stepes.couchverse.core.TitleKind
import io.stepes.couchverse.core.TopTitle
import io.stepes.couchverse.core.XpLine
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.Languages
import io.stepes.couchverse.testing.screenshot
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.util.Locale
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** Profiles, the leaderboard, the editor and a celebration, on a phone and a TV. */
@RunWith(RobolectricTestRunner::class)
class RanksScreenshotTest {
    private val profileActions = ProfileActions(onBack = {}, onEdit = {}, onPublic = {}, onOpenTitle = {}, onRetry = {})

    @Test
    fun `a member's profile`() = Device.entries.forEach { device ->
        Languages.forEach { language ->
            screenshot("profile", device, language) { ProfileScreen(ProfileView("nora", LoadStatus.Loaded, Fixtures.profile), profileActions) }
        }
    }

    @Test
    fun `the leaderboard and a celebration`() = Device.entries.forEach { device ->
        screenshot("leaderboard", device) {
            LeaderboardScreen(Fixtures.board, LeaderboardKey(Period.All, Metric.Xp), onKey = {}, onProfile = {}, onBack = {})
        }
        screenshot("celebration", device, "cs") { CelebrationCard(Fixtures.unlocked, onDismiss = {}) }
    }

    @Test
    fun `the profile editor`() = screenshot("profile_editor", Device.Phone) {
        ProfileEditorScreen(
            ProfileEditorState(
                SessionUser("nora", "Nora", admin = false, bio = "Mostly *science fiction*.", createdAt = "2025-02-01T10:00:00Z"),
                avatarUrl = null,
                bannerUrl = "https://media.example.com/api/v1/artwork/banner-nora?size=w1280",
                view = ProfileEditorView(SaveState(LoadStatus.Loaded), SaveState(LoadStatus.Idle), SaveState(LoadStatus.Idle), SaveState(LoadStatus.Idle)),
            ),
            ProfileEditorActions(onSave = {}, onPassword = {}, onPick = {}, onRemove = {}, onBack = {}),
        )
    }

    @Test
    fun `dates read in the display language, unknown codes as themselves`() {
        assertEquals("1. 2. 2025", memberSince("2025-02-01T10:00:00Z", Locale.forLanguageTag("cs")))
        assertEquals("Feb 1, 2025", memberSince("2025-02-01", Locale.US))
        assertNull(memberSince("soon", Locale.US))
        assertEquals(null, AchievementNames["never_heard_of"])
    }
}

private object Fixtures {
    private val rookie = Tier("rookie", 1u, "#7c8496", 0u)
    private val binger = Tier("binger", 4u, "#34d399", 1500u)
    private val popcorn = Tier("popcorn", 5u, "#fbbf24", 3000u)

    val unlocked = AchievementCard("streak_7", "streaks", "silver", unlocked = true, unlockedAt = "2026-09-30T20:00:00Z", value = 7u, target = 7u, percent = 100u, xp = 150u)

    val profile = ProfileDetail(
        username = "nora",
        displayName = "Nora",
        bio = MarkdownDoc(listOf(Block.Paragraph(listOf(Inline.Text("Mostly science fiction, and anything with a lighthouse."))))),
        banner = Image("https://media.example.com/api/v1/artwork/banner-nora?size=w1280"),
        memberSince = "2025-02-01T10:00:00Z",
        isSelf = true,
        public = true,
        rank = RankBadge(binger, popcorn, 2140u, 43u),
        xpTotal = 2140u,
        xpSources = listOf(XpLine("video", 4200u, 1u, 1400u), XpLine("episodes", 30u, 20u, 600u), XpLine("achievements", 2u, 70u, 140u)),
        achievements = listOf(
            unlocked,
            AchievementCard("first_play", "watching", "bronze", unlocked = true, value = 1u, target = 1u, percent = 100u, xp = 50u),
            AchievementCard("watch_50h", "watching", "silver", unlocked = false, value = 21u, target = 50u, percent = 42u, xp = 200u),
            AchievementCard("couch_host_1", "couch", "bronze", unlocked = false, value = 0u, target = 1u, percent = 0u, xp = 50u),
        ),
        achievementsWon = 2u,
        recentUnlocks = listOf(unlocked),
        totals = ProfileTotals(
            watchSeconds = 75_600u, moviesCompleted = 6u, episodesCompleted = 30u, seriesCompleted = 1u, distinctTitles = 9u,
            distinctGenres = 4u, activeDays = 41u, currentStreak = 7u, longestStreak = 12u, bestDayMinutes = 260u,
            couchHosted = 3u, couchJoined = 5u, biggestCouch = 4u, emojiSent = 48u,
        ),
        topTitles = listOf(
            TopTitle("glass-harbor", "Glass Harbor", TitleKind.Movie, 7_200u, Image("https://media.example.com/api/v1/artwork/p-glass?size=w342")),
            TopTitle("night-shift", "Night Shift", TitleKind.Series, 18_000u, Image("https://media.example.com/api/v1/artwork/p-night?size=w342")),
        ),
        favouriteGenre = "Science Fiction",
        hours = List(24) { hour -> if (hour in 19..23) (hour * 300).toULong() else if (hour in 8..10) 600u else 0u },
        heatmap = Heatmap("2025-10-06", List(364) { day -> HeatDay(if (day % 3 == 0) 1800u else 0u, if (day % 3 == 0) ((day / 3) % 4 + 1).toUByte() else 0u) }, 75_600u, 41u),
    )

    val board = LeaderboardView(
        key = LeaderboardKey(Period.All, Metric.Xp),
        status = LoadStatus.Loaded,
        rows = listOf(
            LeaderRow(1u, "otto", "Otto", null, 6u, "marathoner", 5400u, 5400u, 180_000u, 9u, isSelf = false),
            LeaderRow(2u, "nora", "Nora", null, 4u, "binger", 2140u, 2140u, 75_600u, 2u, isSelf = true),
            LeaderRow(3u, "vera", "Vera", null, 2u, "remote", 640u, 640u, 20_000u, 1u, isSelf = false),
            LeaderRow(4u, "admin", "admin", null, 1u, rookie.code, 90u, 90u, 3_000u, 0u, isSelf = false),
        ),
        myPosition = 2u,
        total = 4u,
        podium = true,
        allZero = false,
        hidden = false,
    )
}
