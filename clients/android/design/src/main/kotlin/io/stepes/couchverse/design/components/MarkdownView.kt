package io.stepes.couchverse.design.components

import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.LinkAnnotation
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLinkStyles
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withLink
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.Block
import io.stepes.couchverse.core.Inline
import io.stepes.couchverse.core.MarkdownDoc
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.theme.LocalAccent

/**
 * A bio rendered from the core's markdown tree (D19). The core already dropped raw HTML, images
 * and unsafe links, so this only maps the tree onto text styles; links open in the browser.
 */
@Composable
fun MarkdownView(
    doc: MarkdownDoc,
    modifier: Modifier = Modifier,
    style: TextStyle = MaterialTheme.typography.bodyLarge,
    color: Color = Tokens.Palette.text,
) {
    val markdown = MarkdownStyle(style.copy(color = color), LocalAccent.current.accent)
    Column(modifier, verticalArrangement = Arrangement.spacedBy(10.dp)) {
        doc.blocks.forEach { markdown.Block(it) }
    }
}

private class MarkdownStyle(val body: TextStyle, val link: Color) {
    @Composable
    fun Block(block: Block) {
        when (block) {
            is Block.Paragraph -> Inlines(block.content, body)
            is Block.Heading -> {
                val type = MaterialTheme.typography
                val size = when (block.content.level.toInt()) {
                    1 -> type.titleLarge
                    2 -> type.titleMedium
                    else -> type.titleSmall
                }
                Inlines(
                    block.content.inlines,
                    body.merge(size).copy(fontWeight = FontWeight.Bold),
                    Modifier.semantics { heading() },
                )
            }
            is Block.Quote -> Row(Modifier.height(IntrinsicSize.Min)) {
                Box(Modifier.width(3.dp).fillMaxHeight().background(link.copy(alpha = 0.6f)))
                Column(Modifier.padding(start = 12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    block.content.forEach { Block(it) }
                }
            }
            is Block.List -> Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                val start = block.content.start
                block.content.items.forEachIndexed { index, item ->
                    Row {
                        val marker = if (start != null) "${start.toInt() + index}." else "•"
                        Text(marker, style = body, modifier = Modifier.width(24.dp))
                        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                            item.blocks.forEach { Block(it) }
                        }
                    }
                }
            }
            is Block.Code -> Text(
                block.content,
                style = body.copy(fontFamily = FontFamily.Monospace),
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Tokens.Palette.surface2, RoundedCornerShape(Tokens.Radius.input))
                    .horizontalScroll(rememberScrollState())
                    .padding(12.dp),
            )
            Block.Rule -> HorizontalDivider(color = Tokens.Palette.edge)
            is Block.Table -> Column(Modifier.horizontalScroll(rememberScrollState())) {
                val header = body.copy(fontWeight = FontWeight.Bold)
                Row { block.content.header.forEach { Inlines(it.inlines, header, CELL) } }
                HorizontalDivider(color = Tokens.Palette.edge)
                block.content.rows.forEach { row ->
                    Row { row.cells.forEach { Inlines(it.inlines, body, CELL) } }
                }
            }
        }
    }

    @Composable
    fun Inlines(inlines: List<Inline>, style: TextStyle, modifier: Modifier = Modifier) {
        val text = remember(inlines, style) { annotate(inlines) }
        Text(text, style = style, modifier = modifier)
    }

    private fun annotate(inlines: List<Inline>): AnnotatedString = buildAnnotatedString {
        fun append(inline: Inline) {
            when (inline) {
                is Inline.Text -> append(inline.content)
                is Inline.Code -> withStyle(SpanStyle(fontFamily = FontFamily.Monospace, background = Tokens.Palette.surface2)) {
                    append(inline.content)
                }
                is Inline.Strong -> withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { inline.content.forEach(::append) }
                is Inline.Emphasis -> withStyle(SpanStyle(fontStyle = FontStyle.Italic)) { inline.content.forEach(::append) }
                is Inline.Strike -> withStyle(SpanStyle(textDecoration = TextDecoration.LineThrough)) {
                    inline.content.forEach(::append)
                }
                is Inline.Link -> {
                    val styles = TextLinkStyles(SpanStyle(color = link, textDecoration = TextDecoration.Underline))
                    withLink(LinkAnnotation.Url(inline.content.href, styles)) { inline.content.children.forEach(::append) }
                }
                Inline.Break -> append('\n')
            }
        }
        inlines.forEach(::append)
    }

    private companion object {
        val CELL = Modifier.width(140.dp).padding(vertical = 6.dp, horizontal = 4.dp)
    }
}
