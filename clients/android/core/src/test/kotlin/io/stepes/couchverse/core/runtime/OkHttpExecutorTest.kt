package io.stepes.couchverse.core.runtime

import io.stepes.couchverse.core.EffectOutput
import io.stepes.couchverse.core.HttpFailureKind
import io.stepes.couchverse.core.HttpHeader
import io.stepes.couchverse.core.HttpRequest
import io.stepes.couchverse.core.HttpResponse
import io.stepes.couchverse.core.UploadRequest
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.RequestBody.Companion.toRequestBody
import java.net.ServerSocket
import java.util.concurrent.TimeUnit
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertIs

class OkHttpExecutorTest {
    private val server = MockWebServer()
    private val client = OkHttpClient.Builder().readTimeout(500, TimeUnit.MILLISECONDS).build()
    private val files = UploadFiles { handle ->
        if (handle == "content://picked/7") UploadFile("me.png", "PNGDATA".toRequestBody("image/png".toMediaType())) else null
    }
    private val executor = OkHttpExecutor(client, files)

    @BeforeTest
    fun start() = server.start()

    @AfterTest
    fun stop() = server.close()

    @Test
    fun `a request carries the core's headers and answers with status and body`() = runTest {
        server.enqueue(MockResponse(code = 404, body = """{"error":{"code":"not_found"}}"""))
        val output = executor.execute(
            HttpRequest("GET", server.url("/api/v1/titles/x").toString(), listOf(HttpHeader("Authorization", "Bearer t1"))),
        )

        assertEquals(EffectOutput.Http(HttpResponse(404u, """{"error":{"code":"not_found"}}""")), output)
        val recorded = server.takeRequest()
        assertEquals("GET", recorded.method)
        assertEquals("Bearer t1", recorded.headers["Authorization"])
    }

    @Test
    fun `json bodies are sent as json and bodiless posts still post`() = runTest {
        server.enqueue(MockResponse(code = 204))
        server.enqueue(MockResponse(code = 204))
        val json = listOf(HttpHeader("Content-Type", "application/json"))
        executor.execute(HttpRequest("POST", server.url("/a").toString(), json, """{"x":1}"""))
        executor.execute(HttpRequest("POST", server.url("/b").toString(), emptyList()))

        val withBody = server.takeRequest()
        assertEquals("""{"x":1}""", withBody.body?.utf8())
        assertContains(withBody.headers["Content-Type"].orEmpty(), "application/json")
        assertEquals(0L, server.takeRequest().bodySize)
    }

    @Test
    fun `an unreachable server is offline and a silent one times out`() = runTest {
        val closedPort = ServerSocket(0).use { it.localPort }
        val refused = executor.execute(HttpRequest("GET", "http://127.0.0.1:$closedPort/", emptyList()))
        assertEquals(HttpFailureKind.Offline, assertIs<EffectOutput.HttpFailed>(refused).content.kind)

        server.enqueue(MockResponse.Builder().body("late").headersDelay(2, TimeUnit.SECONDS).build())
        val slow = executor.execute(HttpRequest("GET", server.url("/slow").toString(), emptyList()))
        assertEquals(HttpFailureKind.Timeout, assertIs<EffectOutput.HttpFailed>(slow).content.kind)
    }

    @Test
    fun `an upload sends the picked file as a multipart form`() = runTest {
        server.enqueue(MockResponse(code = 200, body = "{}"))
        val request = HttpRequest("POST", server.url("/api/v1/me/avatar").toString(), emptyList())
        val output = executor.upload(UploadRequest(request, file = "content://picked/7", field = "file"))

        assertEquals(EffectOutput.Http(HttpResponse(200u, "{}")), output)
        val body = server.takeRequest().body?.utf8().orEmpty()
        assertContains(body, """name="file"; filename="me.png"""")
        assertContains(body, "PNGDATA")

        val gone = executor.upload(UploadRequest(request, file = "content://picked/8", field = "file"))
        assertIs<EffectOutput.HttpFailed>(gone)
    }
}
