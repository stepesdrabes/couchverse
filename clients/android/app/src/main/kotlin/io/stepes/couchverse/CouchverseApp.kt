package io.stepes.couchverse

import android.app.Application
import android.app.UiModeManager
import android.content.Context
import android.content.res.Configuration
import android.os.Build
import android.provider.Settings
import androidx.lifecycle.DefaultLifecycleObserver
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.ProcessLifecycleOwner
import androidx.work.WorkManager
import coil3.ImageLoader
import coil3.PlatformContext
import coil3.SingletonImageLoader
import coil3.network.okhttp.OkHttpNetworkFetcherFactory
import coil3.request.crossfade
import io.stepes.couchverse.core.AuthMode
import io.stepes.couchverse.core.CoreConfig
import io.stepes.couchverse.core.CouchView
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.HomeView
import io.stepes.couchverse.core.Platform
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.core.device.deviceProfile
import io.stepes.couchverse.core.device.measureDevice
import io.stepes.couchverse.core.runtime.CoreRuntime
import io.stepes.couchverse.core.runtime.NoDownloads
import io.stepes.couchverse.core.runtime.androidCoreRuntime
import io.stepes.couchverse.couch.CoreHost
import io.stepes.couchverse.couch.CouchNotification
import io.stepes.couchverse.downloads.WorkDownloads
import io.stepes.couchverse.downloads.downloadsDirectory
import io.stepes.couchverse.integration.ContinueWidgets
import io.stepes.couchverse.integration.WatchNext
import io.stepes.couchverse.integration.continueWatching
import io.stepes.couchverse.playback.PlaybackEngine
import io.stepes.couchverse.playback.PlaybackHost
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.MainScope
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import java.util.Locale
import java.util.concurrent.TimeUnit
import kotlin.concurrent.thread

/**
 * The process's one core runtime, image loader and HTTP client. The runtime outlives
 * activities, so a rotation or a trip through the TV's home screen keeps every view model.
 */
class CouchverseApp : Application(), SingletonImageLoader.Factory, PlaybackHost, CoreHost {
    override lateinit var runtime: CoreRuntime
        private set

    private val scope = MainScope()

    override lateinit var playback: PlaybackEngine
        private set


    /** The TV interface, picked once at launch from the UI mode. */
    val tv: Boolean by lazy { isTelevision(this) }

    private val http by lazy {
        OkHttpClient.Builder()
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .build()
    }

    override fun onCreate() {
        super.onCreate()
        val config = CoreConfig(
            platform = if (tv) Platform.Androidtv else Platform.Android,
            authMode = AuthMode.Bearer,
            deviceName = deviceName(),
            locale = Locale.getDefault().toLanguageTag(),
        )
        playback = PlaybackEngine(this, http, downloadsDirectory(this)) { runtime.send(Event.PlayerReported(it)) }
        // downloads are for phones; a TV streams
        val downloads = if (tv) NoDownloads else WorkDownloads(WorkManager.getInstance(this), downloadsDirectory(this))
        runtime = androidCoreRuntime(this, config, http, player = playback, downloads = downloads)
        runtime.send(Event.AppStarted)
        continueWatching()
        if (!tv) {
            val notification = CouchNotification(this)
            val couch = runtime.view<CouchView>(Surface.Couch)
            val session = runtime.view<SessionView>(Surface.Session)
            scope.launch { couch.combine(session) { c, s -> c to (s?.language ?: "en") }.collect { (c, language) -> notification.show(c, language) } }
        }
        // listing the decoders takes a moment; the profile is only needed once something plays
        thread(name = "device-profile") {
            runtime.send(Event.CapabilitiesReported(deviceProfile(measureDevice(this))))
        }
        ProcessLifecycleOwner.get().lifecycle.addObserver(
            object : DefaultLifecycleObserver {
                private var started = false

                override fun onStart(owner: LifecycleOwner) {
                    if (started) runtime.send(Event.AppBecameActive)
                    started = true
                }
            },
        )
    }

    /** Continue Watching outside the app: Watch Next on a TV, the home screen widget on a phone. */
    private fun continueWatching() {
        val home = runtime.view<HomeView>(Surface.Home)
        val session = runtime.view<SessionView>(Surface.Session)
        val watchNext = if (tv) WatchNext(this) else null
        scope.launch {
            home.map(::continueWatching)
                .combine(session) { cards, s -> cards to (s?.language ?: "en") }
                .distinctUntilChanged()
                .collect { (cards, language) ->
                    withContext(Dispatchers.IO) {
                        if (watchNext != null) watchNext.publish(cards) else ContinueWidgets.publish(this@CouchverseApp, cards, language)
                    }
                }
        }
    }

    override fun newImageLoader(context: PlatformContext): ImageLoader =
        ImageLoader.Builder(context)
            .components { add(OkHttpNetworkFetcherFactory(callFactory = { http })) }
            .crossfade(true)
            .build()

    /** The name a new device session gets: the one the user gave the device, else its model. */
    private fun deviceName(): String =
        Settings.Global.getString(contentResolver, Settings.Global.DEVICE_NAME)?.takeIf { it.isNotBlank() }
            ?: Build.MODEL
}

fun isTelevision(context: Context): Boolean =
    context.getSystemService(UiModeManager::class.java)?.currentModeType == Configuration.UI_MODE_TYPE_TELEVISION
