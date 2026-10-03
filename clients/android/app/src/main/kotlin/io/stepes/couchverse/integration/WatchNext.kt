package io.stepes.couchverse.integration

import android.annotation.SuppressLint
import android.content.Context
import android.net.Uri
import androidx.tvprovider.media.tv.TvContractCompat
import androidx.tvprovider.media.tv.WatchNextProgram
import io.stepes.couchverse.core.AppPhase
import io.stepes.couchverse.core.AppView
import io.stepes.couchverse.core.ContinueCard
import io.stepes.couchverse.core.HomeView
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.TitleKind

/**
 * Continue Watching on the Google TV home: the account's unfinished titles as the app's Watch
 * Next programs, replaced as a whole whenever the home's row changes. Each opens the player
 * where it stopped through a `couchverse://play` link.
 */
class WatchNext(private val context: Context) {
    fun publish(cards: List<ContinueCard>, now: Long = System.currentTimeMillis()) {
        val resolver = context.contentResolver
        runCatching {
            ours().forEach { id -> resolver.delete(TvContractCompat.buildWatchNextProgramUri(id), null, null) }
            cards.forEachIndexed { index, card ->
                resolver.insert(TvContractCompat.WatchNextPrograms.CONTENT_URI, program(card, now - index).toContentValues())
            }
        }
    }

    /** The ids of the Watch Next programs this app published. */
    private fun ours(): List<Long> {
        val cursor = context.contentResolver.query(
            TvContractCompat.WatchNextPrograms.CONTENT_URI,
            arrayOf(TvContractCompat.WatchNextPrograms._ID, TvContractCompat.WatchNextPrograms.COLUMN_INTERNAL_PROVIDER_ID),
            null,
            null,
            null,
        ) ?: return emptyList()
        return cursor.use {
            buildList {
                while (it.moveToNext()) {
                    if (it.getString(1)?.startsWith(PREFIX) == true) add(it.getLong(0))
                }
            }
        }
    }

    // the builder's setters are public API, which lint mistakes for the library's own
    @SuppressLint("RestrictedApi")
    private fun program(card: ContinueCard, engaged: Long): WatchNextProgram {
        val episode = card.kind == TitleKind.Series
        return WatchNextProgram.Builder()
            .setType(if (episode) TvContractCompat.PreviewPrograms.TYPE_TV_EPISODE else TvContractCompat.PreviewPrograms.TYPE_MOVIE)
            .setWatchNextType(TvContractCompat.WatchNextPrograms.WATCH_NEXT_TYPE_CONTINUE)
            .setLastEngagementTimeUtcMillis(engaged)
            .setTitle(card.name)
            .apply { card.episodeLabel?.let(::setEpisodeTitle) }
            .apply { (card.backdrop ?: card.poster)?.url?.let { setPosterArtUri(Uri.parse(it)) } }
            .setPosterArtAspectRatio(if (card.backdrop != null) TvContractCompat.PreviewPrograms.ASPECT_RATIO_16_9 else TvContractCompat.PreviewPrograms.ASPECT_RATIO_2_3)
            .setLastPlaybackPositionMillis((card.positionSeconds * 1000u).toInt())
            .setDurationMillis((card.durationSeconds * 1000u).toInt())
            .setInternalProviderId("$PREFIX${card.play.kind.string}/${card.play.id}")
            .setIntentUri(playLink(card))
            .build()
    }

    private companion object {
        const val PREFIX = "couchverse:"
    }
}

/** The link that plays [card] where it stopped. */
fun playLink(card: ContinueCard): Uri = Uri.parse("couchverse://play/${card.play.kind.string}/${card.play.id}")

/**
 * What Continue Watching outside the app should show: the loaded home's unfinished titles,
 * nothing once no account is left, and `null` (keep what is there) otherwise, so a process
 * started without the home (a download's worker, a TV on "Who's watching?") clears nothing.
 */
fun continueWatching(app: AppView?, home: HomeView?): List<ContinueCard>? = when {
    app?.phase == AppPhase.Welcome || app?.phase == AppPhase.SignIn -> emptyList()
    app?.phase == AppPhase.Ready && home?.status == LoadStatus.Loaded -> home.rows.flatMap { it.continueWatching }
    else -> null
}
