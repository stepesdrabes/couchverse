package io.stepes.couchverse.core.runtime

import io.stepes.couchverse.core.AccountsView
import io.stepes.couchverse.core.AppPhase
import io.stepes.couchverse.core.AppView
import io.stepes.couchverse.core.AuthMode
import io.stepes.couchverse.core.Core
import io.stepes.couchverse.core.CoreConfig
import io.stepes.couchverse.core.CoreEngine
import io.stepes.couchverse.core.EffectOutput
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.HttpFailure
import io.stepes.couchverse.core.HttpFailureKind
import io.stepes.couchverse.core.HttpRequest
import io.stepes.couchverse.core.HttpResponse
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.PairingState
import io.stepes.couchverse.core.PasswordSignIn
import io.stepes.couchverse.core.Platform
import io.stepes.couchverse.core.ServerAddress
import io.stepes.couchverse.core.ServerRef
import io.stepes.couchverse.core.ServersView
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.SignInView
import io.stepes.couchverse.core.Surface
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import kotlin.test.AfterTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * The runtime over the real core on the host JVM, with in-memory stores, a scripted server and
 * virtual time, so timers, ordering and publishing are checked without a device or a network.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class CoreRuntimeTest {
    private val runtimes = mutableListOf<CoreRuntime>()

    @AfterTest
    fun close() = runtimes.forEach { it.close() }

    @Test
    fun `a first launch reads both stores and welcomes`() = runTest {
        val shell = FakeShell(this)
        val runtime = shell.runtime()
        val app = runtime.view<AppView>(Surface.App)

        runtime.send(Event.AppStarted)
        advanceUntilIdle()

        assertEquals(AppPhase.Welcome, app.value?.phase)
        assertTrue(shell.store.log.containsAll(listOf("read servers", "read accounts")))
        assertTrue(shell.store.log.all { it.startsWith("read ") })
    }

    @Test
    fun `adding a server publishes it once the server says who it is`() = runTest {
        val shell = FakeShell(this)
        val answer = CompletableDeferred<Unit>()
        shell.server = { request ->
            answer.await()
            if (request.url == "https://tv.home/api/v1/server") ok(IDENTITY) else null
        }
        val runtime = shell.started()
        val servers = runtime.view<ServersView>(Surface.Servers)
        val app = runtime.view<AppView>(Surface.App)

        runtime.send(Event.ServerAddressSubmitted(ServerAddress("tv.home")))
        runCurrent()
        assertEquals(LoadStatus.Loading, servers.value?.add?.status)
        answer.complete(Unit)
        advanceUntilIdle()

        assertEquals(SERVER, servers.value?.add?.added)
        assertEquals("Home", servers.value?.servers?.single()?.name)
        assertEquals(AppPhase.SignIn, app.value?.phase)
        assertTrue(shell.store.values.getValue("servers").contains("https://tv.home"))
    }

    @Test
    fun `a network failure is reported to the core, not thrown`() = runTest {
        val shell = FakeShell(this)
        shell.server = { failed(HttpFailureKind.Offline) }
        val runtime = shell.started()
        val servers = runtime.view<ServersView>(Surface.Servers)

        runtime.send(Event.ServerAddressSubmitted(ServerAddress("https://tv.home")))
        advanceUntilIdle()

        assertEquals(LoadStatus.Failed, servers.value?.add?.status)
        assertEquals("offline", servers.value?.add?.problem?.code)
    }

    @Test
    fun `pairing polls on the runtime's timers until approved, then stops`() = runTest {
        val shell = FakeShell(this, platform = Platform.Androidtv)
        shell.withServer()
        var polls = 0
        var approved = false
        shell.server = { request ->
            when (request.url) {
                "$BASE/api/v1/auth/pairings" -> ok(PAIRING, status = 201u)
                "$BASE/api/v1/auth/pairings/poll" -> {
                    polls++
                    if (approved) ok(approval()) else ok("""{"status":"pending"}""")
                }
                else -> shell.session(request)
            }
        }
        val runtime = shell.started()
        val signIn = runtime.view<SignInView>(Surface.SignIn)
        val app = runtime.view<AppView>(Surface.App)

        runtime.send(Event.PairingStarted(ServerRef(SERVER)))
        runCurrent()
        val pairing = assertNotNull(signIn.value?.pairing)
        assertEquals("WDJB-MJHT", pairing.userCode)
        assertEquals(PairingState.Waiting, pairing.state)
        assertEquals(runtime.nowMs() + 600_000, pairing.expiresAtMs.toLong())

        advanceTimeBy(5_001)
        assertEquals(1, polls)
        advanceTimeBy(5_000)
        assertEquals(2, polls)

        approved = true
        advanceTimeBy(5_000)
        runCurrent()
        assertEquals(3, polls)
        assertEquals(AppPhase.Ready, app.value?.phase)

        // the poll and expiry timers were cancelled with the pairing
        advanceTimeBy(700_000)
        assertEquals(3, polls)
        assertEquals("tok-tv", shell.secure.values.getValue("token.$SERVER/2"))
    }

    @Test
    fun `a pairing code expires on time`() = runTest {
        val shell = FakeShell(this, platform = Platform.Androidtv)
        shell.withServer()
        shell.server = { request ->
            when (request.url) {
                "$BASE/api/v1/auth/pairings" -> ok(PAIRING, status = 201u)
                else -> ok("""{"status":"pending"}""")
            }
        }
        val runtime = shell.started()
        val signIn = runtime.view<SignInView>(Surface.SignIn)
        runtime.send(Event.PairingStarted(ServerRef(SERVER)))

        advanceTimeBy(599_000)
        assertEquals(PairingState.Waiting, signIn.value?.pairing?.state)
        advanceTimeBy(2_000)
        assertEquals(PairingState.Expired, signIn.value?.pairing?.state)
    }

    @Test
    fun `a render re-reads only the surfaces it names`() = runTest {
        val shell = FakeShell(this)
        shell.server = { ok(IDENTITY) }
        val runtime = shell.started()
        runtime.view<ServersView>(Surface.Servers)
        runtime.view<AccountsView>(Surface.Accounts)
        runtime.view<SessionView>(Surface.Session)
        advanceUntilIdle()
        shell.engine.reads.clear()

        runtime.send(Event.ServerAddressSubmitted(ServerAddress("https://tv.home")))
        advanceUntilIdle()

        assertTrue(Surface.Servers in shell.engine.reads)
        assertTrue(Surface.Accounts !in shell.engine.reads)
        assertTrue(Surface.Session !in shell.engine.reads)
    }

    @Test
    fun `what a session writes is there after a relaunch`() = runTest {
        val shell = FakeShell(this)
        shell.withServer()
        shell.server = { request ->
            when (request.url) {
                "$BASE/api/v1/auth/token" -> ok(DEVICE_TOKEN)
                else -> shell.session(request)
            }
        }
        val runtime = shell.started()
        val before = runtime.view<AppView>(Surface.App)
        runtime.send(Event.PasswordSignInSubmitted(PasswordSignIn(SERVER, "nora", "hunter2")))
        advanceUntilIdle()
        assertEquals(AppPhase.Ready, before.value?.phase)

        // a new process: the same stores, a fresh core
        val relaunched = shell.started()
        val after = relaunched.view<AppView>(Surface.App)
        advanceUntilIdle()
        val app = after.value
        assertEquals(AppPhase.Ready, app?.phase)
        assertEquals("$SERVER/2", app?.activeAccount)
    }

    /** The core, the stores and a scripted server for one test. */
    private inner class FakeShell(private val scope: TestScope, private val platform: Platform = Platform.Android) {
        val store = MemoryStore()
        val secure = MemoryStore()
        val requests = mutableListOf<HttpRequest>()
        var server: suspend (HttpRequest) -> EffectOutput? = { null }
        lateinit var engine: CountingEngine

        fun runtime(): CoreRuntime {
            val dispatcher = StandardTestDispatcher(scope.testScheduler)
            engine = CountingEngine(Core(CoreConfig(platform, AuthMode.Bearer, "Pixel", "en-US")))
            val http = HttpExecutor { request ->
                requests += request
                server(request) ?: failed(HttpFailureKind.Other)
            }
            val upload = UploadExecutor { failed(HttpFailureKind.Other) }
            return CoreRuntime(
                engine = engine,
                executors = EffectExecutors(http, upload, store, secure, sockets = { _, _ -> error("no sockets here") }),
                clock = { scope.testScheduler.currentTime + CLOCK_START },
                coreDispatcher = dispatcher,
                ioDispatcher = dispatcher,
            ).also { runtimes += it }
        }

        /** A runtime that has started and read its stores. */
        fun started(): CoreRuntime = runtime().also {
            it.send(Event.AppStarted)
            scope.advanceUntilIdle()
        }

        fun withServer() {
            store.values["servers"] =
                """[{"id":"$SERVER","url":"$BASE","name":"Home","version":"1.0.0","apiLevel":1,"accent":"#3a6ea5","insecure":false}]"""
        }

        /** The calls a freshly active account makes. */
        fun session(request: HttpRequest): EffectOutput? = when (request.url) {
            "$BASE/api/v1/auth/me" -> ok(USER)
            "$BASE/api/v1/features" -> ok("""{"couchEnabled":true,"rankingsEnabled":false,"downloadsEnabled":true}""")
            "$BASE/api/v1/me/preferences" -> ok("""{"language":"en"}""")
            "$BASE/api/v1/server" -> ok(IDENTITY)
            "$BASE/api/v1/me/artwork-grant" -> ok("""{"grant":"g-art","expiresIn":604800}""")
            else -> null
        }
    }

    /** Counts which surfaces the runtime reads, to check it re-reads only what a render names. */
    private class CountingEngine(private val core: Core) : CoreEngine by core {
        val reads = mutableListOf<Surface>()

        override fun viewJson(surface: Surface): String {
            reads += surface
            return core.viewJson(surface)
        }
    }

    private class MemoryStore : KeyValueStore {
        val values = mutableMapOf<String, String>()
        val log = mutableListOf<String>()

        override suspend fun read(key: String): String? {
            log += "read $key"
            return values[key]
        }

        override suspend fun write(key: String, value: String) {
            log += "write $key"
            values[key] = value
        }

        override suspend fun delete(key: String) {
            log += "delete $key"
            values.remove(key)
        }
    }

    private companion object {
        const val CLOCK_START = 50_000L
        const val SERVER = "4f6c0a5e-6a43-4c0e-9d4b-2b8f8d0b7a11"
        const val BASE = "https://tv.home"
        const val IDENTITY = """{"id":"$SERVER","name":"Home","version":"1.0.0","apiLevel":1,"accent":"#3a6ea5"}"""
        const val PAIRING =
            """{"deviceCode":"dc-1","userCode":"WDJB-MJHT","verifyPath":"/pair?code=WDJB-MJHT","expiresIn":600,"interval":5}"""
        const val USER =
            """{"id":2,"username":"nora","displayName":"Nora","role":"user","bio":"","disabled":false,"createdAt":"2026-01-01T00:00:00Z"}"""
        const val DEVICE_TOKEN = """{"deviceId":"d-2","token":"tok-tv","user":$USER}"""

        fun approval() = """{"status":"approved","device":$DEVICE_TOKEN}"""

        fun ok(body: String, status: UShort = 200u) = EffectOutput.Http(HttpResponse(status, body))

        fun failed(kind: HttpFailureKind) = EffectOutput.HttpFailed(HttpFailure(kind, "simulated"))
    }
}
