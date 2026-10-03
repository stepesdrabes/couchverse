package io.stepes.couchverse.core

import io.stepes.couchverse.core.ffi.CoreBridge
import io.stepes.couchverse.core.ffi.CoreException
import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertIs

/** The real core on the host JVM, driven through the generated types the way a shell drives it. */
class CoreTest {
    private val config = CoreConfig(
        platform = Platform.Android,
        authMode = AuthMode.Bearer,
        deviceName = "Pixel",
        locale = "cs-CZ",
    )

    @Test
    fun `a first launch reads both stores and welcomes`() {
        Core(config).use { core ->
            val reads = core.send(1u, Event.AppStarted).map { assertIs<Effect.Store>(it.effect).content to it.id }
            assertEquals(listOf("servers", "accounts", "player.prefs"), reads.map { (store, _) -> store.key })
            assertEquals(listOf(StoreOp.Read, StoreOp.Read, StoreOp.Read), reads.map { (store, _) -> store.op })

            val effects = reads.flatMap { (_, id) -> core.resolve(2u, id, EffectOutput.Stored(StoredValue())) }
            val render = assertIs<Effect.Render>(effects.last().effect)
            assertContains(render.content.surfaces, Surface.App)
            assertEquals(AppView(AppPhase.Welcome), core.view<AppView>(Surface.App))
        }
    }

    @Test
    fun `adding a server asks it who it is`() {
        Core(config).use { core ->
            val effects = core.send(5u, Event.ServerAddressSubmitted(ServerAddress("tv.home")))
            val identify = effects.single { it.effect is Effect.Http }
            val accept = HttpHeader("Accept", "application/json")
            assertEquals(
                HttpRequest("GET", "https://tv.home/api/v1/server", listOf(accept)),
                (identify.effect as Effect.Http).content,
            )
            assertEquals(LoadStatus.Loading, core.view<ServersView>(Surface.Servers).add.status)

            val identity = """{"id":"srv","name":"Home","version":"1.0.0","apiLevel":1,"accent":"#e50914"}"""
            core.resolve(9u, identify.id, EffectOutput.Http(HttpResponse(200u, identity)))
            val servers = core.view<ServersView>(Surface.Servers)
            assertEquals(
                Server("srv", "https://tv.home", "Home", "1.0.0", 1u, "#e50914", insecure = false),
                servers.servers.single(),
            )
            assertEquals(AddServerView(LoadStatus.Loaded, "tv.home", added = "srv"), servers.add)
        }
    }

    @Test
    fun `markdown renders to a safe tree`() {
        Core(config).use { core ->
            val doc = core.view<MarkdownDoc>(Surface.Markdown("Hi **there** <b>x</b>"))
            val paragraph = listOf(
                Inline.Text("Hi "),
                Inline.Strong(listOf(Inline.Text("there"))),
                Inline.Text(" <b>x</b>"),
            )
            assertEquals(MarkdownDoc(listOf(Block.Paragraph(paragraph))), doc)
        }
    }

    @Test
    fun `malformed messages throw instead of reaching the core`() {
        CoreBridge(CoreJson.encodeToString(config)).use { bridge ->
            val error = assertFailsWith<CoreException.InvalidMessage> {
                bridge.send("""{"nowMs":1,"event":{"type":"launchMissiles"}}""")
            }
            assertContains(error.reason, "launchMissiles")
            assertFailsWith<CoreException> { bridge.resolve("""{"nowMs":1}""") }
            assertFailsWith<CoreException> { bridge.view("not json") }
        }
        assertFailsWith<CoreException> { CoreBridge("{}") }
    }
}
