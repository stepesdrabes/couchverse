package io.stepes.couchverse.ui

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.DownloadState
import io.stepes.couchverse.core.DownloadsView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.couch.active
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.theme.LocalIsTv

/**
 * Asks once per launch to post notifications, the first time one would say something: a
 * couch session to leave from the shade, or a download running in the background. Before
 * Android 13 notifications need no permission.
 */
@Composable
fun NotificationPermission() {
    if (LocalIsTv.current || Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return
    val context = LocalContext.current
    val couch by rememberSurface<CouchView>(Surface.Couch)
    val downloads by rememberSurface<DownloadsView>(Surface.Downloads)
    val wanted = couch?.active == true ||
        downloads?.items?.any { it.state != DownloadState.Ready && it.state != DownloadState.Failed } == true
    var asked by rememberSaveable { mutableStateOf(false) }
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {}
    LaunchedEffect(wanted) {
        val granted = ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        if (wanted && !granted && !asked) {
            asked = true
            launcher.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
    }
}
