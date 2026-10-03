package io.stepes.couchverse.design

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.Block
import io.stepes.couchverse.core.HeadingBlock
import io.stepes.couchverse.core.Inline
import io.stepes.couchverse.core.LinkInline
import io.stepes.couchverse.core.ListBlock
import io.stepes.couchverse.core.ListItem
import io.stepes.couchverse.core.MarkdownDoc
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.components.InsecureBadge
import io.stepes.couchverse.design.components.MarkdownView
import io.stepes.couchverse.design.components.Pill
import io.stepes.couchverse.design.components.QrCode
import io.stepes.couchverse.design.components.SkeletonBox
import io.stepes.couchverse.design.components.WatchProgress
import io.stepes.couchverse.design.phone.BackdropCard
import io.stepes.couchverse.design.phone.PosterCard
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvBackdropCard
import io.stepes.couchverse.design.tv.TvPosterCard
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.screenshot
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** The design system's building blocks on each idiom, so a token or component change shows up. */
@RunWith(RobolectricTestRunner::class)
class ComponentsScreenshotTest {
    private val bio = MarkdownDoc(
        listOf(
            Block.Heading(HeadingBlock(2u, listOf(Inline.Text("About me")))),
            Block.Paragraph(
                listOf(
                    Inline.Text("Mostly "),
                    Inline.Strong(listOf(Inline.Text("sci-fi"))),
                    Inline.Text(", some "),
                    Inline.Emphasis(listOf(Inline.Text("noir"))),
                    Inline.Text(". Lists on "),
                    Inline.Link(LinkInline("https://example.com", listOf(Inline.Text("my site")))),
                    Inline.Text("."),
                ),
            ),
            Block.List(ListBlock(items = listOf(ListItem(listOf(Block.Paragraph(listOf(Inline.Text("Blade Runner")))))))),
        ),
    )

    @Test
    fun `phone components`() = screenshot("components", Device.Phone) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                PosterCard("Glass Harbor", ART + "poster-1?size=w342", onClick = {}, caption = "2024")
                PosterCard("No Poster", null, onClick = {}, accent = "#3a6ea5", caption = "2019")
            }
            BackdropCard("Northern Lights", ART + "backdrop-2?size=w780", onClick = {}, caption = "S1 E3", progress = 0.4f)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Pill("4K")
                Pill("HDR")
                Pill("PG-13")
                InsecureBadge()
            }
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Avatar(null, seed = "nora", size = 56.dp)
                Avatar(null, seed = "admin", size = 56.dp)
                Avatar(ART + "avatar-1?size=w342", seed = "x", size = 56.dp)
                QrCode("https://media.example.com/pair?code=WDJB-MJHT", "QR", Modifier.width(96.dp))
            }
            WatchProgress(0.66f)
            SkeletonBox(Modifier.width(200.dp).padding(vertical = 20.dp))
            MarkdownView(bio)
        }
    }

    @Test
    fun `tv components`() = screenshot("components", Device.Tv) {
        Column(Modifier.padding(32.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
            Row(horizontalArrangement = Arrangement.spacedBy(20.dp)) {
                TvPosterCard("Glass Harbor", ART + "poster-1?size=w342", onClick = {}, caption = "2024")
                TvPosterCard("No Poster", null, onClick = {}, accent = "#3a6ea5")
                TvBackdropCard("Northern Lights", ART + "backdrop-2?size=w780", onClick = {}, caption = "S1 E3", progress = 0.4f)
            }
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                TvActionButton("Play", onClick = {}, primary = true)
                TvActionButton("My List", onClick = {})
            }
        }
    }

    private companion object {
        const val ART = "https://media.example.com/api/v1/artwork/"
    }
}
