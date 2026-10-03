package io.stepes.couchverse.catalog

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Clear
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.tv.foundation.ExperimentalTvFoundationApi
import androidx.tv.foundation.text.PlatformImeOptions
import androidx.tv.foundation.text.TvKeyboardAlignment
import io.stepes.couchverse.catalog.phone.GridSkeletonPhone
import io.stepes.couchverse.catalog.phone.PosterGridPhone
import io.stepes.couchverse.catalog.tv.GridSkeletonTv
import io.stepes.couchverse.catalog.tv.PosterGridTv
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.SearchText
import io.stepes.couchverse.core.SearchView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.phone.PhoneGutter
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.focusOnStart

/**
 * Search as you type: every change goes to the core, which waits for a pause and drops answers
 * to queries typed past; earlier results stay up meanwhile.
 */
@OptIn(ExperimentalTvFoundationApi::class)
@Composable
fun SearchScreen(view: SearchView?, onQuery: (String) -> Unit, navigation: CatalogNavigation, autoFocus: Boolean = true) {
    val tv = LocalIsTv.current
    var query by rememberSaveable { mutableStateOf(view?.query.orEmpty()) }
    val focus = remember { FocusRequester() }
    val keyboard = LocalSoftwareKeyboardController.current
    Column(Modifier.fillMaxSize()) {
        OutlinedTextField(
            value = query,
            onValueChange = {
                query = it
                onQuery(it)
            },
            placeholder = { Text(stringResource(R.string.catalog_search_placeholder)) },
            leadingIcon = { Icon(Icons.Filled.Search, contentDescription = null) },
            trailingIcon = if (query.isNotEmpty() && !tv) {
                {
                    IconButton(onClick = {
                        query = ""
                        onQuery("")
                    }) { Icon(Icons.Filled.Clear, contentDescription = stringResource(R.string.common_clear)) }
                }
            } else {
                null
            },
            singleLine = true,
            keyboardOptions = KeyboardOptions(
                imeAction = ImeAction.Search,
                platformImeOptions = if (tv) PlatformImeOptions(TvKeyboardAlignment.Right) else null,
            ),
            keyboardActions = KeyboardActions(onSearch = { keyboard?.hide() }),
            modifier = Modifier
                .fillMaxWidth()
                .padding(
                    horizontal = if (tv) TvSafe.horizontal else PhoneGutter,
                    vertical = if (tv) TvSafe.vertical else 8.dp,
                )
                .focusOnStart(focus, enabled = autoFocus && query.isEmpty()),
        )
        Box(Modifier.weight(1f)) {
            val cards = view?.cards.orEmpty()
            val status = view?.status
            when {
                query.isBlank() -> Empty(
                    stringResource(R.string.catalog_search_start_title),
                    stringResource(R.string.catalog_search_start_message),
                )
                cards.isNotEmpty() -> if (tv) {
                    PosterGridTv(cards, navigation, loadingMore = false)
                } else {
                    PosterGridPhone(cards, navigation, loadingMore = false)
                }
                status == LoadStatus.Failed -> LoadFailed(view.problem, onRetry = { onQuery(query) })
                status == LoadStatus.Loaded -> Empty(
                    stringResource(R.string.catalog_search_empty_title),
                    stringResource(R.string.catalog_search_empty_message, query.trim()),
                )
                tv -> GridSkeletonTv()
                else -> GridSkeletonPhone()
            }
        }
    }
}

@Composable
fun SearchRoute(navigation: CatalogNavigation) {
    val send = rememberSend()
    val view by rememberSurface<SearchView>(Surface.Search, open = true)
    SearchScreen(view, onQuery = { send(Event.SearchChanged(SearchText(it))) }, navigation = navigation)
}
