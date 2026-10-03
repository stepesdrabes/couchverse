package io.stepes.couchverse.core.runtime

import io.stepes.couchverse.core.EffectOutput
import io.stepes.couchverse.core.HttpHeader
import io.stepes.couchverse.core.SocketOpen
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import java.net.ServerSocket
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs

class OkHttpSocketExecutorTest {
    private val server = MockWebServer()
    private val executor = OkHttpSocketExecutor(OkHttpClient())
    private val events = LinkedBlockingQueue<EffectOutput>()

    @BeforeTest
    fun start() = server.start()

    @AfterTest
    fun stop() = server.close()

    @Test
    fun `a socket opens, carries frames both ways and ends once`() {
        server.enqueue(
            MockResponse.Builder().webSocketUpgrade(
                object : WebSocketListener() {
                    override fun onOpen(webSocket: WebSocket, response: Response) {
                        webSocket.send("welcome")
                    }

                    override fun onMessage(webSocket: WebSocket, text: String) {
                        webSocket.send("echo:$text")
                    }
                },
            ).build(),
        )
        val url = server.url("/api/v1/couch/ws").toString().replaceFirst("http", "ws")
        val socket = executor.open(SocketOpen(url, listOf(HttpHeader("Authorization", "Bearer t1"))), events::put)

        assertEquals(EffectOutput.SocketOpened, next())
        assertEquals("welcome", assertIs<EffectOutput.SocketText>(next()).content.text)
        socket.send("hi")
        assertEquals("echo:hi", assertIs<EffectOutput.SocketText>(next()).content.text)
        assertEquals("Bearer t1", server.takeRequest().headers["Authorization"])

        socket.close()
        assertEquals(1000, assertIs<EffectOutput.SocketClosed>(next()).content.code.toInt())
        assertEquals(null, events.poll(300, TimeUnit.MILLISECONDS))
    }

    @Test
    fun `a socket that cannot connect closes abnormally`() {
        val port = ServerSocket(0).use { it.localPort }
        executor.open(SocketOpen("ws://127.0.0.1:$port/ws", emptyList()), events::put)
        assertEquals(1006, assertIs<EffectOutput.SocketClosed>(next()).content.code.toInt())

        executor.open(SocketOpen("not a url", emptyList()), events::put)
        assertEquals(1006, assertIs<EffectOutput.SocketClosed>(next()).content.code.toInt())
    }

    private fun next(): EffectOutput = events.poll(5, TimeUnit.SECONDS) ?: error("no socket event")
}
