package io.stepes.couchverse.design.phone

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.design.Tokens

/** Horizontal padding of phone screens; shelves scroll under it to the screen edge. */
val PhoneGutter = 16.dp

/** A titled, horizontally scrolling row of cards. */
@Composable
fun Shelf(title: String, modifier: Modifier = Modifier, content: LazyListScope.() -> Unit) {
    Column(modifier, verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Text(
            title,
            style = MaterialTheme.typography.titleLarge,
            color = Tokens.Palette.text,
            modifier = Modifier.padding(horizontal = PhoneGutter).semantics { heading() },
        )
        LazyRow(
            contentPadding = PaddingValues(horizontal = PhoneGutter),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            content = content,
        )
    }
}
