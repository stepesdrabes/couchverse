package io.stepes.couchverse.settings

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.ExitToApp
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.DeviceCard
import io.stepes.couchverse.core.DeviceRef
import io.stepes.couchverse.core.DevicesView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.CouchverseIcons
import io.stepes.couchverse.design.components.LoadState
import io.stepes.couchverse.design.components.SkeletonBox
import io.stepes.couchverse.design.components.StatusMessage
import io.stepes.couchverse.design.components.loadingSemantics
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.text.displayLocale
import io.stepes.couchverse.design.text.formatRelative
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvSafe
import androidx.tv.material3.ListItem as TvListItem
import androidx.tv.material3.Text as TvText

/** This account's signed-in devices, any of which (but this one) can be signed out. */
@Composable
fun DevicesScreen(view: DevicesView?, onRevoke: (DeviceCard) -> Unit, onRetry: () -> Unit, onBack: (() -> Unit)?) {
    val tv = LocalIsTv.current
    var revoking by remember { mutableStateOf<DeviceCard?>(null) }
    Column(Modifier.fillMaxSize().then(if (tv) Modifier.padding(horizontal = TvSafe.horizontal, vertical = TvSafe.vertical) else Modifier.statusBarsPadding())) {
        Header(stringResource(R.string.devices_heading), onBack)
        Text(
            stringResource(R.string.devices_hint),
            style = MaterialTheme.typography.bodyMedium,
            color = Tokens.Palette.muted,
            modifier = Modifier.padding(horizontal = if (tv) 0.dp else 16.dp, vertical = 4.dp),
        )
        view?.problem?.takeIf { view.status != LoadStatus.Failed }?.let {
            Text(problemMessage(it), color = Tokens.Palette.danger, modifier = Modifier.padding(16.dp))
        }
        LoadState(
            status = view?.status,
            skeleton = {
                Column(Modifier.loadingSemantics().padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    repeat(3) { SkeletonBox(Modifier.fillMaxWidth().height(56.dp)) }
                }
            },
            failed = {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    StatusMessage(
                        title = stringResource(R.string.devices_load_failed),
                        message = problemMessage(view?.problem),
                        action = {
                            if (tv) TvActionButton(stringResource(R.string.common_retry), onClick = onRetry, primary = true)
                            else Button(onClick = onRetry) { Text(stringResource(R.string.common_retry)) }
                        },
                    )
                }
            },
        ) {
            LazyColumn(Modifier.widthIn(max = 720.dp), contentPadding = PaddingValues(vertical = 8.dp)) {
                items(view?.devices.orEmpty(), key = { it.id }) { device -> DeviceRow(device, tv) { revoking = device } }
            }
        }
    }
    revoking?.let { device ->
        ConfirmDialog(
            title = stringResource(R.string.devices_sign_out_title, device.name),
            message = stringResource(R.string.devices_sign_out_message),
            confirm = stringResource(R.string.devices_sign_out),
            onConfirm = { onRevoke(device) },
            onDismiss = { revoking = null },
        )
    }
}

@Composable
private fun DeviceRow(device: DeviceCard, tv: Boolean, onRevoke: () -> Unit) {
    val locale = displayLocale()
    val seen = if (device.current) {
        stringResource(R.string.devices_this_device)
    } else {
        formatRelative(device.lastSeenAt, locale)?.let { stringResource(R.string.devices_last_seen, it) }
    }
    val detail = listOfNotNull(platformName(device.platform), seen).joinToString("  ·  ")
    if (tv) {
        TvListItem(
            selected = false,
            enabled = !device.current,
            onClick = onRevoke,
            leadingContent = { androidx.tv.material3.Icon(CouchverseIcons.Devices, contentDescription = null) },
            headlineContent = { TvText(device.name) },
            supportingContent = { TvText(detail) },
            trailingContent = if (device.current) null else {
                { androidx.tv.material3.Icon(Icons.AutoMirrored.Filled.ExitToApp, contentDescription = stringResource(R.string.devices_sign_out_device, device.name)) }
            },
        )
    } else {
        ListItem(
            colors = ListItemDefaults.colors(containerColor = Color.Transparent),
            leadingContent = { Icon(CouchverseIcons.Devices, contentDescription = null) },
            headlineContent = { Text(device.name) },
            supportingContent = { Text(detail) },
            trailingContent = if (device.current) null else {
                {
                    IconButton(onClick = onRevoke) {
                        Icon(Icons.AutoMirrored.Filled.ExitToApp, contentDescription = stringResource(R.string.devices_sign_out_device, device.name))
                    }
                }
            },
        )
    }
}

@Composable
internal fun Header(title: String, onBack: (() -> Unit)?) {
    androidx.compose.foundation.layout.Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(vertical = 8.dp)) {
        if (onBack != null && !LocalIsTv.current) {
            IconButton(onClick = onBack) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back))
            }
        }
        Text(
            title,
            style = MaterialTheme.typography.headlineMedium,
            color = Tokens.Palette.text,
            modifier = Modifier.padding(start = if (onBack == null || LocalIsTv.current) 16.dp else 0.dp).semantics { heading() },
        )
    }
}

@Composable
fun DevicesRoute(onBack: () -> Unit) {
    val send = rememberSend()
    val view by rememberSurface<DevicesView>(Surface.Devices)
    LaunchedEffect(Unit) { send(Event.DevicesOpened) }
    DevicesScreen(
        view,
        onRevoke = { send(Event.DeviceRevoked(DeviceRef(it.id))) },
        onRetry = { send(Event.DevicesOpened) },
        onBack = onBack,
    )
}
