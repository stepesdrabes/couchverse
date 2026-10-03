package io.stepes.couchverse.downloads

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.work.Configuration
import androidx.work.WorkManager
import androidx.work.testing.SynchronousExecutor
import androidx.work.testing.WorkManagerTestInitHelper
import io.stepes.couchverse.core.DownloadStart
import io.stepes.couchverse.core.EffectOutput
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okio.Buffer
import org.junit.After
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertTrue

/** The executor over a real WorkManager running its work synchronously. */
@RunWith(RobolectricTestRunner::class)
class WorkDownloadsTest {
    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val server = MockWebServer()
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    private val events = CopyOnWriteArrayList<EffectOutput>()
    private lateinit var work: WorkManager
    private lateinit var downloads: WorkDownloads

    @Before
    fun setUp() {
        server.start()
        WorkManagerTestInitHelper.initializeTestWorkManager(
            context,
            Configuration.Builder().setExecutor(SynchronousExecutor()).build(),
        )
        work = WorkManager.getInstance(context)
        downloads = WorkDownloads(work, downloadsDirectory(context), scope)
    }

    @After
    fun tearDown() {
        scope.cancel()
        server.close()
        downloadsDirectory(context).deleteRecursively()
    }

    @Test
    fun `a download is fetched into the directory and finishes`() {
        server.enqueue(MockResponse.Builder().body(Buffer().write(ByteArray(4096) { 7 })).build())

        downloads.start(DownloadStart(server.url("/d1").toString(), "d1.mp4")) { events += it }
        online("d1.mp4")
        val done = awaitEnd()

        assertIs<EffectOutput.DownloadFinished>(done)
        assertEquals(4096uL, done.content.bytes)
        assertEquals(4096L, downloadsDirectory(context).resolve("d1.mp4").length())
    }

    @Test
    fun `after a relaunch a finished file is found, a missing one fails, and remove deletes it`() {
        downloadsDirectory(context).resolve("d2.mp4").writeBytes(ByteArray(100))

        downloads.start(DownloadStart("", "d2.mp4")) { events += it }
        assertEquals(100uL, assertIs<EffectOutput.DownloadFinished>(awaitEnd()).content.bytes)

        events.clear()
        downloads.start(DownloadStart("", "d3.mp4")) { events += it }
        assertIs<EffectOutput.DownloadFailed>(awaitEnd())

        downloads.remove("d2.mp4")
        assertTrue(!downloadsDirectory(context).resolve("d2.mp4").exists())
    }

    @Test
    fun `a refused download fails without retrying`() {
        server.enqueue(MockResponse.Builder().code(410).build())

        downloads.start(DownloadStart(server.url("/gone").toString(), "d4.mp4")) { events += it }
        online("d4.mp4")

        val failed = assertIs<EffectOutput.DownloadFailed>(awaitEnd())
        assertEquals(null, failed.content.noSpace)
        assertEquals(1, server.requestCount)
    }

    /** The transfer waits for a network, which the test's WorkManager never sees on its own. */
    private fun online(name: String) = runBlocking {
        val id = withTimeout(10_000) {
            var info = work.getWorkInfosForUniqueWork("download:$name").get().lastOrNull()
            while (info == null) {
                kotlinx.coroutines.delay(20)
                info = work.getWorkInfosForUniqueWork("download:$name").get().lastOrNull()
            }
            info.id
        }
        WorkManagerTestInitHelper.getTestDriver(context)!!.setAllConstraintsMet(id)
    }

    private fun awaitEnd(): EffectOutput = runBlocking {
        withTimeout(10_000) {
            while (events.none { it is EffectOutput.DownloadFinished || it is EffectOutput.DownloadFailed }) {
                kotlinx.coroutines.delay(20)
            }
        }
        events.last()
    }
}
