package io.stepes.couchverse.downloads

import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import okio.Buffer
import org.junit.After
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull
import kotlin.test.assertTrue

class FetchTest {
    @get:Rule
    val folder = TemporaryFolder()

    private val server = MockWebServer().apply { start() }
    private val client = OkHttpClient()
    private val bytes = ByteArray(200_000) { (it % 251).toByte() }

    @After
    fun tearDown() = server.close()

    @Test
    fun `a file arrives whole and reports its progress`() {
        server.enqueue(MockResponse.Builder().body(Buffer().write(bytes)).build())
        val part = folder.root.resolve("d1.mp4.part")
        val heard = mutableListOf<Pair<Long, Long?>>()

        val size = fetch(client, server.url("/d1").toString(), part) { received, total -> heard += received to total }

        assertEquals(bytes.size.toLong(), size)
        assertTrue(part.readBytes().contentEquals(bytes))
        assertEquals(bytes.size.toLong() to bytes.size.toLong(), heard.last())
        assertNull(server.takeRequest().headers["Range"])
    }

    @Test
    fun `a retry resumes where the last attempt stopped`() {
        val part = folder.root.resolve("d1.mp4.part").apply { writeBytes(bytes.copyOfRange(0, 50_000)) }
        server.enqueue(
            MockResponse.Builder()
                .code(206)
                .addHeader("Content-Range", "bytes 50000-199999/200000")
                .body(Buffer().write(bytes.copyOfRange(50_000, bytes.size)))
                .build(),
        )

        val size = fetch(client, server.url("/d1").toString(), part) { _, _ -> }

        assertEquals(bytes.size.toLong(), size)
        assertTrue(part.readBytes().contentEquals(bytes))
        assertEquals("bytes=50000-", server.takeRequest().headers["Range"])
    }

    @Test
    fun `a server that ignores the range sends it all again`() {
        val part = folder.root.resolve("d1.mp4.part").apply { writeBytes(ByteArray(10)) }
        server.enqueue(MockResponse.Builder().body(Buffer().write(bytes)).build())

        fetch(client, server.url("/d1").toString(), part) { _, _ -> }

        assertTrue(part.readBytes().contentEquals(bytes))
    }

    @Test
    fun `a refusal says so instead of keeping the error page`() {
        server.enqueue(MockResponse.Builder().code(403).body("expired").build())
        val part = folder.root.resolve("d1.mp4.part")

        val refused = assertFailsWith<Refused> { fetch(client, server.url("/d1").toString(), part) { _, _ -> } }

        assertEquals(403, refused.code)
        assertTrue(!part.exists() || part.length() == 0L)
    }

    @Test
    fun `running out of space is told apart from a network failure`() {
        assertTrue(isNoSpace(java.io.IOException("write failed: ENOSPC (No space left on device)")))
        assertTrue(!isNoSpace(java.io.IOException("unexpected end of stream")))
    }
}
