package io.stepes.couchverse.integration

import io.stepes.couchverse.core.AppPhase
import io.stepes.couchverse.core.AppView
import io.stepes.couchverse.core.ContinueCard
import io.stepes.couchverse.core.HomeRowKind
import io.stepes.couchverse.core.HomeRowView
import io.stepes.couchverse.core.HomeView
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.PlayKind
import io.stepes.couchverse.core.PlayTarget
import io.stepes.couchverse.core.TitleKind
import org.junit.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class ContinueWatchingTest {
    private val card = ContinueCard(
        titleId = "t1",
        slug = "night-shift-2024",
        name = "Night Shift",
        kind = TitleKind.Series,
        episodeLabel = "S1:E2",
        positionSeconds = 600u,
        durationSeconds = 1800u,
        progress = 0.33,
        play = PlayTarget(PlayKind.Episode, "e2"),
    )
    private val home = HomeView(
        LoadStatus.Loaded,
        emptyList(),
        listOf(HomeRowView("continue", HomeRowKind.ContinueWatching, "Continue watching", emptyList(), listOf(card))),
    )
    private val ready = AppView(AppPhase.Ready, "s1/2")

    @Test
    fun `the loaded home is shown and an empty one clears`() {
        assertEquals(listOf(card), continueWatching(ready, home))
        assertEquals(emptyList(), continueWatching(ready, home.copy(rows = emptyList())))
    }

    @Test
    fun `without an account nothing is left to continue`() {
        assertEquals(emptyList(), continueWatching(AppView(AppPhase.SignIn), home))
        assertEquals(emptyList(), continueWatching(AppView(AppPhase.Welcome), null))
    }

    @Test
    fun `a home that is not there yet keeps what is published`() {
        assertNull(continueWatching(null, null))
        assertNull(continueWatching(AppView(AppPhase.Starting), null))
        assertNull(continueWatching(AppView(AppPhase.ChooseAccount), home))
        assertNull(continueWatching(ready, null))
        assertNull(continueWatching(ready, home.copy(status = LoadStatus.Loading)))
        assertNull(continueWatching(ready, home.copy(status = LoadStatus.Failed)))
    }
}
