package io.stepes.couchverse.core.runtime

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

/** Everything the runtime performs on the core's behalf, apart from timers it runs itself. */
class EffectExecutors(
    val http: HttpExecutor,
    val upload: UploadExecutor,
    val store: KeyValueStore,
    val secureStore: KeyValueStore,
    val sockets: SocketExecutor,
    // the player arrives with playback (Phase 12); until then its commands go nowhere
    val player: PlayerExecutor = PlayerExecutor {},
)
