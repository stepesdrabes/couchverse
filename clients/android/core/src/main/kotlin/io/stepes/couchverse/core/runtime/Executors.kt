package io.stepes.couchverse.core.runtime

import io.stepes.couchverse.core.DownloadFailure
import io.stepes.couchverse.core.DownloadStart
import io.stepes.couchverse.core.EffectOutput
import io.stepes.couchverse.core.HttpRequest
import io.stepes.couchverse.core.PlayerCommand
import io.stepes.couchverse.core.SocketOpen
import io.stepes.couchverse.core.UploadRequest

/** Performs `http` effects; answers with `EffectOutput.Http` or `EffectOutput.HttpFailed`. */
fun interface HttpExecutor {
    suspend fun execute(request: HttpRequest): EffectOutput
}

/** Performs `upload` effects; answers like [HttpExecutor]. */
fun interface UploadExecutor {
    suspend fun upload(request: UploadRequest): EffectOutput
}

/** A string store for the `store` and `secureStore` effects. Calls never overlap. */
interface KeyValueStore {
    suspend fun read(key: String): String?

    suspend fun write(key: String, value: String)

    suspend fun delete(key: String)
}

/** Opens WebSockets for `socket` effects. */
fun interface SocketExecutor {
    /**
     * Opens [request]. [events] gets `SocketOpened`, a `SocketText` per frame and finally one
     * `SocketClosed`, in order, from any thread.
     */
    fun open(request: SocketOpen, events: (EffectOutput) -> Unit): SocketConnection
}

interface SocketConnection {
    fun send(text: String)

    fun close()
}

/**
 * Drives the video player for `player` effects, called on the core's thread; the player reports
 * back with `PlayerReported` events.
 */
fun interface PlayerExecutor {
    fun perform(command: PlayerCommand)
}

/** Runs `download` effects: files fetched into the downloads directory, surviving the app. */
interface DownloadExecutor {
    /**
     * Fetches [request]'s URL into the file it names, or with an empty URL only picks up a
     * transfer of that name that is running or finished. [events] gets `DownloadProgress` at most
     * about once a second, then one `DownloadFinished` or `DownloadFailed`, from any thread.
     */
    fun start(request: DownloadStart, events: (EffectOutput) -> Unit): DownloadTransfer

    /** Deletes a finished file. */
    fun remove(name: String)
}

fun interface DownloadTransfer {
    /** Stops the transfer and deletes what it fetched; no further events follow. */
    fun cancel()
}

/** For devices without downloads (TVs): every start fails at once. */
object NoDownloads : DownloadExecutor {
    override fun start(request: DownloadStart, events: (EffectOutput) -> Unit): DownloadTransfer {
        events(EffectOutput.DownloadFailed(DownloadFailure("downloads are not supported on this device")))
        return DownloadTransfer {}
    }

    override fun remove(name: String) = Unit
}

/** Everything the runtime performs on the core's behalf, apart from timers it runs itself. */
class EffectExecutors(
    val http: HttpExecutor,
    val upload: UploadExecutor,
    val store: KeyValueStore,
    val secureStore: KeyValueStore,
    val sockets: SocketExecutor,
    val player: PlayerExecutor = PlayerExecutor {},
    val downloads: DownloadExecutor = NoDownloads,
)
