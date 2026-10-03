package io.stepes.couchverse.playback

import android.net.Uri
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.common.MimeTypes
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.TrackSelectionParameters
import androidx.media3.common.Tracks
import io.stepes.couchverse.core.PlayerLoad
import io.stepes.couchverse.core.PlayerSource
import io.stepes.couchverse.core.PlayerSubtitle
import java.io.File

/** The media item for a `load` command; a download's `url` is its name in [downloads]. */
fun mediaItem(load: PlayerLoad, downloads: File): MediaItem {
    val builder = MediaItem.Builder()
        .setUri(
            when (load.source) {
                PlayerSource.Download -> Uri.fromFile(File(downloads, load.url))
                else -> Uri.parse(load.url)
            },
        )
        .setMediaMetadata(metadata(load))
        .setSubtitleConfigurations(load.subtitles.filter { it.url != null }.map(::sidecar))
    if (load.source == PlayerSource.Hls) builder.setMimeType(MimeTypes.APPLICATION_M3U8)
    return builder.build()
}

private fun metadata(load: PlayerLoad): MediaMetadata = MediaMetadata.Builder()
    .setTitle(load.nowPlaying.title)
    .setSubtitle(load.nowPlaying.subtitle)
    .setDisplayTitle(load.nowPlaying.title)
    .setArtworkUri(load.nowPlaying.artwork?.let(Uri::parse))
    .setDurationMs((load.nowPlaying.durationSeconds * 1000).toLong().takeIf { it > 0 })
    .build()

private fun sidecar(subtitle: PlayerSubtitle): MediaItem.SubtitleConfiguration =
    MediaItem.SubtitleConfiguration.Builder(Uri.parse(subtitle.url))
        .setId(subtitle.id)
        .setMimeType(MimeTypes.TEXT_VTT)
        .setLanguage(subtitle.lang)
        .setLabel(subtitle.label)
        .setSelectionFlags(if (subtitle.forced) C.SELECTION_FLAG_FORCED else 0)
        .build()

/** The quality cap and audio language a load asks for; subtitles are chosen once tracks are known. */
fun TrackSelectionParameters.Builder.applyLoad(load: PlayerLoad): TrackSelectionParameters.Builder {
    clearOverrides()
    val maxHeight = load.maxHeight?.toInt()
    if (maxHeight != null) setMaxVideoSize(Int.MAX_VALUE, maxHeight) else clearVideoSizeConstraints()
    setPreferredAudioLanguage(load.audioLang)
    return this
}

/**
 * Shows the subtitle track [wanted] (an id from the load's subtitles, `null` for none) among
 * [tracks]. A sidecar track is found by its id; one inside the media (a download's) by its
 * language. Returns whether the track was found, so a selection can wait for the tracks.
 */
fun TrackSelectionParameters.Builder.selectSubtitles(
    wanted: PlayerSubtitle?,
    tracks: Tracks,
): Boolean {
    if (wanted == null) {
        clearOverridesOfType(C.TRACK_TYPE_TEXT)
        setTrackTypeDisabled(C.TRACK_TYPE_TEXT, true)
        return true
    }
    val group = tracks.groups.firstOrNull { group ->
        group.type == C.TRACK_TYPE_TEXT && (0 until group.length).any { i ->
            val format = group.getTrackFormat(i)
            if (wanted.url != null) {
                // merged sources prefix their tracks' ids with the source's index
                format.id == wanted.id || format.id?.endsWith(":${wanted.id}") == true
            } else {
                format.language?.let(::baseLanguage) == baseLanguage(wanted.lang)
            }
        }
    } ?: return false
    setTrackTypeDisabled(C.TRACK_TYPE_TEXT, false)
    setOverrideForType(TrackSelectionOverride(group.mediaTrackGroup, 0))
    return true
}

private fun baseLanguage(tag: String): String = tag.substringBefore('-').lowercase()
