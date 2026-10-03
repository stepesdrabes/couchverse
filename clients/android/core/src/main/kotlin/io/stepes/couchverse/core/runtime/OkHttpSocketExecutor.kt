package io.stepes.couchverse.core.runtime

import io.stepes.couchverse.core.EffectOutput
import io.stepes.couchverse.core.SocketClosed
import io.stepes.couchverse.core.SocketOpen
import io.stepes.couchverse.core.SocketText
import okhttp3.Headers
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import java.util.concurrent.atomic.AtomicBoolean

/** `socket` effects over OkHttp's WebSocket, which takes `ws://` and `wss://` URLs as they are. */
class OkHttpSocketExecutor(private val client: OkHttpClient) : SocketExecutor {
    override fun open(request: SocketOpen, events: (EffectOutput) -> Unit): SocketConnection {
        val closed = AtomicBoolean(false)
        // the core expects exactly one terminal output, whichever way the socket ends
        fun finish(code: Int, reason: String?) {
            if (closed.compareAndSet(false, true)) {
                events(EffectOutput.SocketClosed(SocketClosed(code.toUShort(), reason?.takeIf { it.isNotEmpty() })))
            }
        }
        val listener = object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) = events(EffectOutput.SocketOpened)

            override fun onMessage(webSocket: WebSocket, text: String) = events(EffectOutput.SocketText(SocketText(text)))

            override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                webSocket.close(code, null)
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) = finish(code, reason)

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) =
                finish(ABNORMAL_CLOSURE, t.message)
        }
        val headers = Headers.Builder().apply { request.headers.forEach { add(it.name, it.value) } }.build()
        val socket = try {
            client.newWebSocket(Request.Builder().url(request.url).headers(headers).build(), listener)
        } catch (e: IllegalArgumentException) {
            finish(ABNORMAL_CLOSURE, e.message)
            return Closed
        }
        return object : SocketConnection {
            override fun send(text: String) {
                socket.send(text)
            }

            override fun close() {
                socket.close(NORMAL_CLOSURE, null)
            }
        }
    }

    private object Closed : SocketConnection {
        override fun send(text: String) = Unit

        override fun close() = Unit
    }

    private companion object {
        const val NORMAL_CLOSURE = 1000
        const val ABNORMAL_CLOSURE = 1006
    }
}
