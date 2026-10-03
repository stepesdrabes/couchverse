package io.stepes.couchverse.core.runtime

import io.stepes.couchverse.core.EffectOutput
import io.stepes.couchverse.core.HttpRequest
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

/** Everything the runtime performs on the core's behalf, apart from timers it runs itself. */
class EffectExecutors(
    val http: HttpExecutor,
    val upload: UploadExecutor,
    val store: KeyValueStore,
    val secureStore: KeyValueStore,
)
