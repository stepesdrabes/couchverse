package io.stepes.couchverse.core.runtime

import io.stepes.couchverse.core.CoreEngine
import io.stepes.couchverse.core.CoreJson
import io.stepes.couchverse.core.Effect
import io.stepes.couchverse.core.EffectOutput
import io.stepes.couchverse.core.EffectRequest
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.HttpFailure
import io.stepes.couchverse.core.HttpFailureKind
import io.stepes.couchverse.core.StoreFailure
import io.stepes.couchverse.core.StoreOp
import io.stepes.couchverse.core.StoreRequest
import io.stepes.couchverse.core.StoredValue
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.core.TimerRequest
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineExceptionHandler
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.DeserializationStrategy
import kotlinx.serialization.serializer

/**
 * Runs the shared core for the app: every call into the bridge happens on [coreDispatcher] (it
 * must be serial), stamped with the monotonic [clock]; the effects it asks for are performed by
 * [executors] (I/O on [ioDispatcher]) and by the runtime's own timers, and their outputs are fed
 * back. View models are decoded on the core's thread and published as one [StateFlow] per
 * surface, re-read only when a `render` effect names the surface.
 */
class CoreRuntime(
    private val engine: CoreEngine,
    private val executors: EffectExecutors,
    private val clock: () -> Long,
    private val coreDispatcher: CoroutineDispatcher,
    private val ioDispatcher: CoroutineDispatcher,
    private val log: (String, Throwable?) -> Unit = { _, _ -> },
) : AutoCloseable {
    private val job = SupervisorJob()
    private val scope = CoroutineScope(
        job + coreDispatcher + CoroutineExceptionHandler { _, e -> log("core runtime failed", e) },
    )

    /** Running timers by effect id; only touched on the core's dispatcher. */
    private val timers = HashMap<ULong, Job>()

    /** Store effects run one at a time and in order, so a read always sees earlier writes. */
    private val storeQueue = Channel<suspend () -> Unit>(Channel.UNLIMITED)

    /** Published surfaces, least recently requested first; guarded by itself. */
    private val slots = LinkedHashMap<Surface, Slot<*>>(16, 0.75f, true)

    init {
        scope.launch(ioDispatcher) {
            for (operation in storeQueue) operation()
        }
    }

    /** The monotonic time the core sees, for countdowns against view model deadlines. */
    fun nowMs(): Long = clock()

    fun send(event: Event) {
        scope.launch { perform(engine.send(now(), event)) }
    }

    /**
     * The view model of [surface], `null` until it was first read. Holding the flow does not
     * open the surface: catalog screens also send `ScreenOpened` and `ScreenClosed`.
     */
    fun <T> view(surface: Surface, model: DeserializationStrategy<T>): StateFlow<T?> {
        var created: Slot<T>? = null
        val slot = synchronized(slots) {
            slots[surface] ?: Slot(surface, model).also {
                slots[surface] = it
                created = it
                evictIdle(keep = it)
            }
        }
        created?.let { scope.launch { read(it) } }
        @Suppress("UNCHECKED_CAST")
        return (slot as Slot<T>).flow.asStateFlow()
    }

    inline fun <reified T> view(surface: Surface): StateFlow<T?> = view(surface, serializer<T>())

    override fun close() {
        job.cancel()
        storeQueue.close()
        engine.close()
    }

    private fun now(): ULong = clock().toULong()

    private fun perform(effects: List<EffectRequest>) {
        for (request in effects) {
            when (val effect = request.effect) {
                is Effect.Http -> io(request.id) { executors.http.execute(effect.content) }
                is Effect.Upload -> io(request.id) { executors.upload.upload(effect.content) }
                is Effect.Timer -> startTimer(request.id, effect.content)
                is Effect.CancelTimer -> timers.remove(effect.content.id)?.cancel()
                is Effect.Store -> store(executors.store, request.id, effect.content)
                is Effect.SecureStore -> store(executors.secureStore, request.id, effect.content)
                is Effect.Render -> render(effect.content.surfaces)
            }
        }
    }

    private fun resolve(id: ULong, output: EffectOutput) {
        perform(engine.resolve(now(), id, output))
    }

    private fun io(id: ULong, work: suspend () -> EffectOutput) {
        scope.launch(ioDispatcher) {
            val output = attempt("perform effect $id") { work() }.getOrElse {
                EffectOutput.HttpFailed(HttpFailure(HttpFailureKind.Other, it.toString()))
            }
            withContext(coreDispatcher) { resolve(id, output) }
        }
    }

    private fun startTimer(id: ULong, timer: TimerRequest) {
        val repeat = timer.repeat == true
        val after = timer.afterMs.toLong().let { if (repeat) it.coerceAtLeast(1) else it }
        timers[id] = scope.launch {
            do {
                delay(after)
                if (!repeat) timers.remove(id)
                resolve(id, EffectOutput.TimerFired)
            } while (repeat)
        }
    }

    private fun store(store: KeyValueStore, id: ULong, request: StoreRequest) {
        val key = request.key
        storeQueue.trySend {
            when (val op = request.op) {
                StoreOp.Read -> {
                    val output = attempt("read $key") { store.read(key) }.fold(
                        onSuccess = { EffectOutput.Stored(StoredValue(it)) },
                        onFailure = { EffectOutput.StoreFailed(StoreFailure(it.toString())) },
                    )
                    withContext(coreDispatcher) { resolve(id, output) }
                }
                // writes and deletes are fire-and-forget: nothing in the core waits for them
                is StoreOp.Write -> attempt("write $key") { store.write(key, op.content) }
                StoreOp.Delete -> attempt("delete $key") { store.delete(key) }
            }
        }
    }

    /** Runs [work], logging a failure instead of letting it take the runtime down. */
    private suspend fun <T> attempt(what: String, work: suspend () -> T): Result<T> =
        try {
            Result.success(work())
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            log("could not $what", e)
            Result.failure(e)
        }

    private fun render(surfaces: List<Surface>) {
        val due = synchronized(slots) { surfaces.mapNotNull { slots[it] } }
        due.forEach { read(it) }
    }

    private fun <T> read(slot: Slot<T>) {
        try {
            slot.flow.value = CoreJson.decodeFromString(slot.model, engine.viewJson(slot.surface))
        } catch (e: Exception) {
            log("could not read ${slot.surface}", e)
        }
    }

    /** Drops surfaces nobody collects once there are many, so parametric ones do not pile up. */
    private fun evictIdle(keep: Slot<*>) {
        val iterator = slots.values.iterator()
        while (slots.size > MAX_SLOTS && iterator.hasNext()) {
            val slot = iterator.next()
            if (slot !== keep && slot.flow.subscriptionCount.value == 0) iterator.remove()
        }
    }

    private class Slot<T>(val surface: Surface, val model: DeserializationStrategy<T>) {
        val flow = MutableStateFlow<T?>(null)
    }

    private companion object {
        const val MAX_SLOTS = 48
    }
}
