package io.stepes.couchverse.integration

import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import android.net.Uri
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.glance.ColorFilter
import androidx.glance.GlanceId
import androidx.glance.GlanceModifier
import androidx.glance.Image
import androidx.glance.ImageProvider
import androidx.glance.action.clickable
import androidx.glance.appwidget.GlanceAppWidget
import androidx.glance.appwidget.GlanceAppWidgetManager
import androidx.glance.appwidget.GlanceAppWidgetReceiver
import androidx.glance.appwidget.LinearProgressIndicator
import androidx.glance.appwidget.action.actionStartActivity
import androidx.glance.appwidget.cornerRadius
import androidx.glance.appwidget.provideContent
import androidx.glance.appwidget.updateAll
import androidx.glance.background
import androidx.glance.layout.Alignment
import androidx.glance.layout.Column
import androidx.glance.layout.Row
import androidx.glance.layout.Spacer
import androidx.glance.layout.fillMaxSize
import androidx.glance.layout.fillMaxWidth
import androidx.glance.layout.height
import androidx.glance.layout.padding
import androidx.glance.layout.size
import androidx.glance.layout.width
import androidx.glance.text.FontWeight
import androidx.glance.text.Text
import androidx.glance.text.TextStyle
import androidx.glance.unit.ColorProvider
import io.stepes.couchverse.core.ContinueCard
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.theme.colorOf
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import java.util.Locale

/** What the widget shows, kept by the app so the widget never needs the core or the network. */
@Serializable
data class ContinueSnapshot(
    val language: String,
    val items: List<ContinueItem>,
    /** The session's accent, `#rrggbb`. */
    val accent: String? = null,
)

@Serializable
data class ContinueItem(val title: String, val label: String?, val progress: Float, val link: String)

/** Writes the snapshot and redraws the widgets on the home screen. */
object ContinueWidgets {
    private fun file(context: Context) = File(context.filesDir, "widget/continue.json")

    suspend fun publish(context: Context, cards: List<ContinueCard>, language: String, accent: String?) {
        val snapshot = ContinueSnapshot(
            language,
            cards.take(MAX).map { ContinueItem(it.name, it.episodeLabel, it.progress.toFloat(), playLink(it).toString()) },
            accent,
        )
        file(context).apply { parentFile?.mkdirs() }.writeText(Json.encodeToString(snapshot))
        if (GlanceAppWidgetManager(context).getGlanceIds(ContinueWatchingWidget::class.java).isNotEmpty()) {
            ContinueWatchingWidget().updateAll(context)
        }
    }

    fun read(context: Context): ContinueSnapshot? =
        runCatching { Json.decodeFromString<ContinueSnapshot>(file(context).readText()) }.getOrNull()

    private const val MAX = 3
}

/**
 * Continue Watching on the phone's home screen: the unfinished titles, each a tap from playing,
 * under the logo in the session's accent.
 */
class ContinueWatchingWidget : GlanceAppWidget() {
    override suspend fun provideGlance(context: Context, id: GlanceId) {
        val snapshot = ContinueWidgets.read(context)
        // the widget speaks the app's display language, not the system's
        val words = snapshot?.language?.let { language ->
            context.createConfigurationContext(Configuration(context.resources.configuration).apply { setLocale(Locale.forLanguageTag(language)) })
        } ?: context
        provideContent { Content(snapshot, words) }
    }

    @Composable
    private fun Content(snapshot: ContinueSnapshot?, words: Context) {
        val accent = colorOf(snapshot?.accent) ?: Tokens.Palette.accent
        Column(
            GlanceModifier.fillMaxSize().background(Tokens.Palette.bg).cornerRadius(16.dp).padding(14.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Image(
                    ImageProvider(R.drawable.couchverse_logo),
                    contentDescription = null,
                    colorFilter = ColorFilter.tint(ColorProvider(accent)),
                    modifier = GlanceModifier.size(width = 22.dp, height = 16.dp),
                )
                Spacer(GlanceModifier.width(8.dp))
                Text(
                    words.getString(R.string.home_row_continue_watching),
                    style = TextStyle(color = ColorProvider(Tokens.Palette.text), fontSize = 16.sp, fontWeight = FontWeight.Bold),
                )
            }
            Spacer(GlanceModifier.height(8.dp))
            val items = snapshot?.items.orEmpty()
            if (items.isEmpty()) {
                Text(words.getString(R.string.widget_continue_empty), style = TextStyle(color = ColorProvider(Tokens.Palette.muted), fontSize = 13.sp))
            }
            items.forEach { item ->
                Column(
                    GlanceModifier
                        .fillMaxWidth()
                        .padding(vertical = 6.dp)
                        .clickable(actionStartActivity(Intent(Intent.ACTION_VIEW, Uri.parse(item.link)))),
                ) {
                    Text(
                        listOfNotNull(item.title, item.label).joinToString(" · "),
                        style = TextStyle(color = ColorProvider(Tokens.Palette.text), fontSize = 14.sp),
                        maxLines = 1,
                    )
                    Spacer(GlanceModifier.height(4.dp))
                    LinearProgressIndicator(
                        progress = item.progress,
                        modifier = GlanceModifier.fillMaxWidth().height(3.dp),
                        color = ColorProvider(accent),
                        backgroundColor = ColorProvider(Color.White.copy(alpha = 0.15f)),
                    )
                }
            }
        }
    }
}

class ContinueWatchingReceiver : GlanceAppWidgetReceiver() {
    override val glanceAppWidget: GlanceAppWidget = ContinueWatchingWidget()
}
