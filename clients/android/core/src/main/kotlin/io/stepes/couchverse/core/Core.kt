package io.stepes.couchverse.core

import io.stepes.couchverse.core.ffi.CoreBridge
import kotlinx.serialization.DeserializationStrategy
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.json.Json
import kotlinx.serialization.serializer

/**
 * The bridge's wire format. Data-carrying enums are sealed classes tagged `{"type", "content"}`.
 * Defaults stay unencoded: an optional the shell leaves unset must be absent, because serde
 * rejects `null` for a field that only has a default (`CoreConfig.origin`).
 */
val CoreJson: Json = Json {
    classDiscriminator = "type"
    encodeDefaults = false
}

/** The core as the runtime drives it; [Core] is the real one, tests may wrap it. */
interface CoreEngine : AutoCloseable {
    /** [wallMs] is the Unix time, for what depends on the date; [nowMs] the monotonic clock. */
    fun send(nowMs: ULong, wallMs: ULong, event: Event): List<EffectRequest>

    fun resolve(nowMs: ULong, id: ULong, output: EffectOutput): List<EffectRequest>

    /** The current view model of [surface] as JSON. */
    fun viewJson(surface: Surface): String
}

/**
 * The shared core spoken to in the generated message types rather than JSON strings. Like the
 * bridge underneath, it is single-threaded by contract: call it from one serial context.
 */
class Core(config: CoreConfig) : CoreEngine {
    private val bridge = CoreBridge(CoreJson.encodeToString(config))

    /** Delivers a shell event; returns the effects to perform. */
    override fun send(nowMs: ULong, wallMs: ULong, event: Event): List<EffectRequest> =
        effects(bridge.send(CoreJson.encodeToString(Message(nowMs, event, wallMs))))

    /** Hands back the output of effect [id]; returns the effects to perform. */
    override fun resolve(nowMs: ULong, id: ULong, output: EffectOutput): List<EffectRequest> =
        effects(bridge.resolve(CoreJson.encodeToString(Resolution(nowMs, id, output))))

    override fun viewJson(surface: Surface): String = bridge.view(CoreJson.encodeToString(surface))

    /** The current view model of [surface]. */
    fun <T> view(surface: Surface, model: DeserializationStrategy<T>): T =
        CoreJson.decodeFromString(model, viewJson(surface))

    inline fun <reified T> view(surface: Surface): T = view(surface, serializer<T>())

    override fun close() = bridge.close()

    private fun effects(json: String) = CoreJson.decodeFromString(effectList, json)

    private companion object {
        val effectList = ListSerializer(EffectRequest.serializer())
    }
}
