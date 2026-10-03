package io.stepes.couchverse.core.runtime

import android.content.Context
import android.os.SystemClock
import android.util.Log
import io.stepes.couchverse.core.Core
import io.stepes.couchverse.core.CoreConfig
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.asCoroutineDispatcher
import okhttp3.OkHttpClient
import java.io.File
import java.util.concurrent.Executors

/** The runtime as the app runs it: one core thread, OkHttp, files and the Keystore. */
fun androidCoreRuntime(
    context: Context,
    config: CoreConfig,
    client: OkHttpClient,
    player: PlayerExecutor = PlayerExecutor {},
    downloads: DownloadExecutor = NoDownloads,
): CoreRuntime {
    val directory = File(context.filesDir, "core")
    val http = OkHttpExecutor(client, ContentUploadFiles(context.contentResolver))
    val executors = EffectExecutors(
        http = http,
        upload = http,
        store = FileStore(File(directory, "store")),
        secureStore = EncryptedStore(FileStore(File(directory, "secure")), KeystoreSecretBox(KEY_ALIAS)),
        sockets = OkHttpSocketExecutor(client),
        player = player,
        downloads = downloads,
    )
    val coreThread = Executors.newSingleThreadExecutor { Thread(it, "couchverse-core") }
    return CoreRuntime(
        engine = Core(config),
        executors = executors,
        clock = SystemClock::elapsedRealtime,
        coreDispatcher = coreThread.asCoroutineDispatcher(),
        ioDispatcher = Dispatchers.IO,
        log = { message, error -> Log.w("CoreRuntime", message, error) },
    )
}

private const val KEY_ALIAS = "couchverse.secure-store"
