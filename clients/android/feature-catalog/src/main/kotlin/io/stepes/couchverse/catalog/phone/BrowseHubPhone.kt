package io.stepes.couchverse.catalog.phone

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.PrimaryTabRow
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import io.stepes.couchverse.catalog.BrowseRoute
import io.stepes.couchverse.catalog.CatalogNavigation
import io.stepes.couchverse.catalog.GenresRoute
import io.stepes.couchverse.core.TitleKind
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens

/** The phone's Browse tab: movies, series and genres side by side. */
@Composable
fun BrowseHubPhone(navigation: CatalogNavigation) {
    var tab by rememberSaveable { mutableIntStateOf(0) }
    val tabs = listOf(R.string.nav_movies, R.string.nav_series, R.string.nav_genres)
    Column(Modifier.fillMaxSize()) {
        PhoneHeader(stringResource(R.string.nav_browse), onBack = null)
        PrimaryTabRow(selectedTabIndex = tab, containerColor = Color.Transparent) {
            tabs.forEachIndexed { index, label ->
                Tab(
                    selected = tab == index,
                    onClick = { tab = index },
                    text = { Text(stringResource(label)) },
                    selectedContentColor = Tokens.Palette.text,
                    unselectedContentColor = Tokens.Palette.muted,
                )
            }
        }
        Box(Modifier.weight(1f)) {
            when (tab) {
                0 -> BrowseRoute(TitleKind.Movie, null, stringResource(R.string.nav_movies), navigation, showTitle = false)
                1 -> BrowseRoute(TitleKind.Series, null, stringResource(R.string.nav_series), navigation, showTitle = false)
                else -> GenresRoute(navigation, showTitle = false)
            }
        }
    }
}
