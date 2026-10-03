package io.stepes.couchverse.design.text

import android.icu.text.MeasureFormat
import android.icu.text.RelativeDateTimeFormatter
import android.icu.util.Measure
import android.icu.util.MeasureUnit
import android.icu.util.ULocale
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.ui.platform.LocalConfiguration
import java.time.Duration
import java.time.Instant
import java.util.Locale

/** The display language's locale, as [io.stepes.couchverse.design.theme.ProvideDisplayLanguage] set it. */
@Composable
@ReadOnlyComposable
fun displayLocale(): Locale = LocalConfiguration.current.locales[0]

/** A runtime in the display language's short form: "1h 52m", "1 h 52 min". */
fun formatRuntime(minutes: Int, locale: Locale): String {
    val format = MeasureFormat.getInstance(ULocale.forLocale(locale), MeasureFormat.FormatWidth.NARROW)
    val hours = minutes / 60
    val rest = minutes % 60
    val parts = buildList {
        if (hours > 0) add(Measure(hours, MeasureUnit.HOUR))
        if (rest > 0 || hours == 0) add(Measure(rest, MeasureUnit.MINUTE))
    }
    return format.formatMeasures(*parts.toTypedArray())
}

/** A playback position as a clock: "12:34", "1:02:03". */
fun formatClock(totalSeconds: Long): String {
    val s = totalSeconds.coerceAtLeast(0)
    val hours = s / 3600
    val minutes = (s % 3600) / 60
    val seconds = s % 60
    return if (hours > 0) "%d:%02d:%02d".format(hours, minutes, seconds) else "%d:%02d".format(minutes, seconds)
}

/** "3 hours ago", "před 3 hodinami" for an RFC 3339 timestamp; `null` when it does not parse. */
fun formatRelative(timestamp: String, locale: Locale, now: Instant = Instant.now()): String? {
    val then = runCatching { Instant.parse(timestamp) }.getOrNull()
        ?: runCatching { java.time.OffsetDateTime.parse(timestamp).toInstant() }.getOrNull()
        ?: return null
    val seconds = Duration.between(now, then).seconds
    val formatter = RelativeDateTimeFormatter.getInstance(ULocale.forLocale(locale))
    val magnitude = kotlin.math.abs(seconds)
    val direction = if (seconds < 0) RelativeDateTimeFormatter.Direction.LAST else RelativeDateTimeFormatter.Direction.NEXT
    val (amount, unit) = when {
        magnitude >= 365 * DAY -> magnitude / (365 * DAY) to RelativeDateTimeFormatter.RelativeUnit.YEARS
        magnitude >= 30 * DAY -> magnitude / (30 * DAY) to RelativeDateTimeFormatter.RelativeUnit.MONTHS
        magnitude >= 7 * DAY -> magnitude / (7 * DAY) to RelativeDateTimeFormatter.RelativeUnit.WEEKS
        magnitude >= DAY -> magnitude / DAY to RelativeDateTimeFormatter.RelativeUnit.DAYS
        magnitude >= 3600 -> magnitude / 3600 to RelativeDateTimeFormatter.RelativeUnit.HOURS
        magnitude >= 60 -> magnitude / 60 to RelativeDateTimeFormatter.RelativeUnit.MINUTES
        else -> return formatter.format(RelativeDateTimeFormatter.Direction.PLAIN, RelativeDateTimeFormatter.AbsoluteUnit.NOW)
    }
    val rounded = amount.toDouble()
    return formatter.format(rounded, direction, unit)
}

private const val DAY = 86_400L
