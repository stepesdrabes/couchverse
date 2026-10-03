package io.stepes.couchverse.core.runtime

import io.stepes.couchverse.core.CoreEngine
import io.stepes.couchverse.core.DownloadCommand
import io.stepes.couchverse.core.DownloadFinished
import io.stepes.couchverse.core.DownloadName
import io.stepes.couchverse.core.DownloadProgress
import io.stepes.couchverse.core.DownloadRef
import io.stepes.couchverse.core.DownloadStart
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
import kotlin.test.assertTrue

/** Socket, player and download effects through the runtime, with a scripted core standing in. */
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

    @Test
    fun `a download reports until it ends, a cancel silences it, and messages carry the wall clock`() = runTest {
        val engine = ScriptedEngine()
        val downloads = FakeDownloads()
        val runtime = runtime(engine, FakeConnection(), downloads)

        runtime.send(Event.DownloadRetried(DownloadRef("d1")))
        advanceUntilIdle()
        assertEquals(listOf("d1.mp4"), downloads.started)
        assertEquals(listOf("d0.mp4"), downloads.removed)
        downloads.emit(EffectOutput.DownloadProgress(DownloadProgress(10u, 100u)))
        downloads.emit(EffectOutput.DownloadFinished(DownloadFinished(100u)))
        downloads.emit(EffectOutput.DownloadProgress(DownloadProgress(100u, 100u)))
        advanceUntilIdle()
        assertEquals(listOf("progress 10", "finished 100"), engine.seen, "nothing after the end")

        runtime.send(Event.DownloadRetried(DownloadRef("d1")))
        runtime.send(Event.DownloadRemoved(DownloadRef("d1")))
        advanceUntilIdle()
        assertTrue(downloads.cancelled)
        downloads.emit(EffectOutput.DownloadFinished(DownloadFinished(100u)))
        advanceUntilIdle()
        assertEquals(2, engine.seen.size, "a cancelled transfer says nothing more")
        assertEquals(listOf(WALL_MS, WALL_MS, WALL_MS), engine.wall)
        runtime.close()
    }

    private fun kotlinx.coroutines.test.TestScope.runtime(
        engine: ScriptedEngine,
        socket: FakeConnection,
        downloads: DownloadExecutor = NoDownloads,
    ): CoreRuntime {
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
                downloads = downloads,
            ),
            clock = { testScheduler.currentTime },
            wallClock = { WALL_MS.toLong() },
            coreDispatcher = dispatcher,
            ioDispatcher = dispatcher,
        )
    }

    /** Opens a socket on start, answers a ping with a pong, closes it and pauses on resume. */
    private class ScriptedEngine : CoreEngine {
        val seen = mutableListOf<String>()
        val played = mutableListOf<PlayerCommand>()

        val wall = mutableListOf<ULong>()

        override fun send(nowMs: ULong, wallMs: ULong, event: Event): List<EffectRequest> = when (event) {
            Event.AppStarted -> listOf(EffectRequest(SOCKET, Effect.Socket(SocketCommand.Open(SocketOpen("wss://tv.home/api/v1/couch/ws", emptyList())))))
            Event.AppBecameActive -> listOf(
                EffectRequest(8u, Effect.Socket(SocketCommand.Close(EffectRef(SOCKET)))),
                EffectRequest(9u, Effect.Player(PlayerCommand.Pause)),
            )
            is Event.DownloadRetried -> {
                wall += wallMs
                listOf(
                    EffectRequest(10u, Effect.Download(DownloadCommand.Start(DownloadStart("https://tv.home/d1.mp4", "d1.mp4")))),
                    EffectRequest(12u, Effect.Download(DownloadCommand.Remove(DownloadName("d0.mp4")))),
                )
            }
            is Event.DownloadRemoved -> {
                wall += wallMs
                listOf(EffectRequest(11u, Effect.Download(DownloadCommand.Cancel(EffectRef(10u)))))
            }
            else -> emptyList()
        }

        override fun resolve(nowMs: ULong, id: ULong, output: EffectOutput): List<EffectRequest> {
            seen += when (output) {
                EffectOutput.SocketOpened -> "socketOpened"
                is EffectOutput.SocketText -> "text ${output.content.text}"
                is EffectOutput.SocketClosed -> "closed ${output.content.code}"
                is EffectOutput.DownloadProgress -> "progress ${output.content.receivedBytes}"
                is EffectOutput.DownloadFinished -> "finished ${output.content.bytes}"
                is EffectOutput.DownloadFailed -> "failed ${output.content.message}"
                else -> output.toString()
            }
            val ping = output is EffectOutput.SocketText && output.content.text == "ping"
            return if (ping) listOf(EffectRequest(7u, Effect.Socket(SocketCommand.Send(SocketSend(SOCKET, "pong"))))) else emptyList()
        }

        override fun viewJson(surface: Surface) = "null"

        override fun close() = Unit
    }

    private class FakeDownloads : DownloadExecutor {
        val started = mutableListOf<String>()
        val removed = mutableListOf<String>()
        var cancelled = false
        private var events: (EffectOutput) -> Unit = {}

        override fun start(request: DownloadStart, events: (EffectOutput) -> Unit): DownloadTransfer {
            started += request.name
            this.events = events
            return DownloadTransfer { cancelled = true }
        }

        override fun remove(name: String) {
            removed += name
        }

        fun emit(output: EffectOutput) = events(output)
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
        const val WALL_MS: ULong = 1_790_000_000_000u
    }
}
