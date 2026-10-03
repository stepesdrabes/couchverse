package io.stepes.couchverse.design.tv

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.runtime.Composable
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.focusRestorer
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text
import io.stepes.couchverse.design.Tokens

/** The TV's title-safe margins: content starts inside the area every TV shows uncropped. */
object TvSafe {
    val horizontal = 48.dp
    val vertical = 27.dp
}

/**
 * A titled row of focusable cards. Coming back to the row focuses the card that was focused
 * before, rather than whichever is nearest, so moving up and down keeps your place.
 */
@OptIn(ExperimentalComposeUiApi::class)
@Composable
fun TvShelf(title: String, modifier: Modifier = Modifier, content: LazyListScope.() -> Unit) {
    Column(modifier, verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(
            title,
            style = MaterialTheme.typography.titleLarge,
            color = Tokens.Palette.text,
            modifier = Modifier.padding(horizontal = TvSafe.horizontal).semantics { heading() },
        )
        LazyRow(
            modifier = Modifier.focusRestorer(),
            contentPadding = PaddingValues(horizontal = TvSafe.horizontal, vertical = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(20.dp),
            content = content,
        )
    }
}
