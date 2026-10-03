package io.stepes.couchverse.core.runtime

import io.stepes.couchverse.core.CoreEngine
import io.stepes.couchverse.core.Effect
import io.stepes.couchverse.core.EffectOutput
import io.stepes.couchverse.core.EffectRef
import io.stepes.couchverse.core.EffectRequest
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.HttpFailureKind
import io.stepes.couchverse.core.HttpFailure
import io.stepes.couchverse.core.PlayerCommand
import io.stepes.couchverse.core.SocketClosed
import io.stepes.couchverse.core.SocketCommand
import io.stepes.couchverse.core.SocketOpen
import io.stepes.couchverse.core.SocketSend
import io.stepes.couchverse.core.SocketText
import io.stepes.couchverse.core.Surface
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals

/** Socket and player effects through the runtime, with a scripted core standing in. */
@OptIn(ExperimentalCoroutinesApi::class)
class SocketEffectsTest {
    @Test
    fun `a socket's outputs reach the core in order and its commands reach the socket`() = runTest {
        val socket = FakeConnection()
        val engine = ScriptedEngine()
        val runtime = runtime(engine, socket)

        runtime.send(Event.AppStarted)
        advanceUntilIdle()
        assertEquals("wss://tv.home/api/v1/couch/ws", socket.opened?.url)

        socket.emit(EffectOutput.SocketOpened)
        socket.emit(EffectOutput.SocketText(SocketText("hello")))
        socket.emit(EffectOutput.SocketText(SocketText("ping")))
        advanceUntilIdle()
        assertEquals(listOf("socketOpened", "text hello", "text ping"), engine.seen)
        assertEquals(listOf("pong"), socket.sent)

        runtime.send(Event.AppBecameActive)
        advanceUntilIdle()
        assertEquals(true, socket.closed)
        assertEquals<List<PlayerCommand>>(listOf(PlayerCommand.Pause), engine.played)
        runtime.close()
    }

    @Test
    fun `a socket that ends is forgotten`() = runTest {
        val socket = FakeConnection()
        val engine = ScriptedEngine()
        val runtime = runtime(engine, socket)
        runtime.send(Event.AppStarted)
        advanceUntilIdle()

        socket.emit(EffectOutput.SocketClosed(SocketClosed(1006u, "gone")))
        advanceUntilIdle()
        runtime.send(Event.AppBecameActive)
        advanceUntilIdle()

        assertEquals(listOf("closed 1006"), engine.seen)
        assertEquals(false, socket.closed, "a closed socket is not closed again")
        runtime.close()
    }

    private fun kotlinx.coroutines.test.TestScope.runtime(engine: ScriptedEngine, socket: FakeConnection): CoreRuntime {
        val dispatcher = StandardTestDispatcher(testScheduler)
        val unused = EffectOutput.HttpFailed(HttpFailure(HttpFailureKind.Other, "unused"))
        return CoreRuntime(
            engine = engine,
            executors = EffectExecutors(
                http = { unused },
                upload = { unused },
                store = NoStore,
                secureStore = NoStore,
                sockets = socket,
                player = { engine.played += it },
            ),
            clock = { testScheduler.currentTime },
            coreDispatcher = dispatcher,
            ioDispatcher = dispatcher,
        )
    }

    /** Opens a socket on start, answers a ping with a pong, closes it and pauses on resume. */
    private class ScriptedEngine : CoreEngine {
        val seen = mutableListOf<String>()
        val played = mutableListOf<PlayerCommand>()

        override fun send(nowMs: ULong, event: Event): List<EffectRequest> = when (event) {
            Event.AppStarted -> listOf(EffectRequest(SOCKET, Effect.Socket(SocketCommand.Open(SocketOpen("wss://tv.home/api/v1/couch/ws", emptyList())))))
            Event.AppBecameActive -> listOf(
                EffectRequest(8u, Effect.Socket(SocketCommand.Close(EffectRef(SOCKET)))),
                EffectRequest(9u, Effect.Player(PlayerCommand.Pause)),
            )
            else -> emptyList()
        }

        override fun resolve(nowMs: ULong, id: ULong, output: EffectOutput): List<EffectRequest> {
            seen += when (output) {
                EffectOutput.SocketOpened -> "socketOpened"
                is EffectOutput.SocketText -> "text ${output.content.text}"
                is EffectOutput.SocketClosed -> "closed ${output.content.code}"
                else -> output.toString()
            }
            val ping = output is EffectOutput.SocketText && output.content.text == "ping"
            return if (ping) listOf(EffectRequest(7u, Effect.Socket(SocketCommand.Send(SocketSend(SOCKET, "pong"))))) else emptyList()
        }

        override fun viewJson(surface: Surface) = "null"

        override fun close() = Unit
    }

    private class FakeConnection : SocketExecutor, SocketConnection {
        var opened: SocketOpen? = null
        val sent = mutableListOf<String>()
        var closed = false
        private var events: (EffectOutput) -> Unit = {}

        override fun open(request: SocketOpen, events: (EffectOutput) -> Unit): SocketConnection {
            opened = request
            this.events = events
            return this
        }

        fun emit(output: EffectOutput) = events(output)

        override fun send(text: String) {
            sent += text
        }

        override fun close() {
            closed = true
        }
    }

    private object NoStore : KeyValueStore {
        override suspend fun read(key: String): String? = null

        override suspend fun write(key: String, value: String) = Unit

        override suspend fun delete(key: String) = Unit
    }

    private companion object {
        const val SOCKET: ULong = 5u
    }
}
