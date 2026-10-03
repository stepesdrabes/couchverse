package io.stepes.couchverse.design.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import io.stepes.couchverse.design.Tokens

/**
 * A member's picture, or the same pixel identicon the web draws from [seed] when there is none.
 * Decorative: the name next to it is what accessibility services read.
 */
@Composable
fun Avatar(
    url: String?,
    seed: String,
    modifier: Modifier = Modifier,
    size: Dp = 40.dp,
    shape: Shape = CircleShape,
) {
    Box(modifier.size(size).clip(shape).background(Tokens.Palette.surface2)) {
        Identicon(seed, Modifier.fillMaxSize().padding(size * 0.14f))
        if (url != null) {
            AsyncImage(model = url, contentDescription = null, modifier = Modifier.fillMaxSize(), contentScale = androidx.compose.ui.layout.ContentScale.Crop)
        }
    }
}

@Composable
private fun Identicon(seed: String, modifier: Modifier) {
    val identicon = remember(seed) { Identicon.of(seed) }
    Canvas(modifier) {
        val cell = size.minDimension / Identicon.VIEWBOX
        val origin = Offset(
            (size.width - cell * Identicon.VIEWBOX) / 2 + cell * Identicon.PADDING,
            (size.height - cell * Identicon.VIEWBOX) / 2 + cell * Identicon.PADDING,
        )
        for ((x, y) in identicon.cells) {
            drawRect(identicon.color, origin + Offset(x * cell, y * cell), Size(cell, cell))
        }
    }
}

/**
 * A port of minidenticons (MIT), which the web uses: a 5x5 mirrored pattern and a hue from a
 * small hash of the seed, so a member looks the same in every client.
 */
class Identicon private constructor(val color: Color, val cells: List<Pair<Int, Int>>) {
    companion object {
        /** The SVG's view box is 8 units wide around the 5x5 grid. */
        const val VIEWBOX = 8f
        const val PADDING = 1.5f
        private const val COLORS = 9
        private const val SATURATION = 0.95f
        private const val LIGHTNESS = 0.45f

        fun of(seed: String): Identicon {
            val hash = hash(seed)
            val hue = (hash % COLORS).toFloat() * (360f / COLORS)
            val cells = if (seed.isEmpty()) emptyList() else (0 until 25).mapNotNull { i ->
                if (hash and (1L shl (i % 15)) == 0L) return@mapNotNull null
                val x = if (i > 14) 7 - i / 5 else i / 5
                x to i % 5
            }
            return Identicon(Color.hsl(hue, SATURATION, LIGHTNESS), cells)
        }

        /** The seed's identity colour, for tinting around an avatar. */
        fun colorOf(seed: String): Color = of(seed).color

        /**
         * `seed.split('').reduce((h, c) => (h ^ c.charCodeAt(0)) * -5, 5) >>> 2` with JavaScript's
         * number semantics: `^` works on int32, the product is a double, `>>>` on uint32.
         */
        internal fun hash(seed: String): Long {
            var hash = 5.0
            for (char in seed) {
                hash = (int32(hash) xor char.code).toDouble() * -5
            }
            return (int32(hash).toLong() and 0xFFFF_FFFFL) ushr 2
        }

        private fun int32(value: Double): Int = value.toLong().toInt()
    }
}
