package io.stepes.couchverse.core

import io.stepes.couchverse.core.ffi.CoreBridge
import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.KSerializer
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.descriptors.elementNames
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.serializer
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * The generated Kotlin types against the Rust side's JSON: what Kotlin encodes, the core
 * decodes, and what the core encodes, Kotlin decodes and re-encodes to the same JSON.
 */
class WireFormatTest {
    @Test
    fun `every event encodes as the core expects`() {
        val movies = BrowseKey(kind = TitleKind.Movie, sort = BrowseSort.Name)
        val events = listOf(
            Event.AppStarted to """{"type":"appStarted"}""",
            Event.AppBecameActive to """{"type":"appBecameActive"}""",
            Event.SessionStarted to """{"type":"sessionStarted"}""",
            Event.SessionChanged to """{"type":"sessionChanged"}""",
            Event.ServerAddressSubmitted(ServerAddress("tv.home")) to
                """{"type":"serverAddressSubmitted","content":{"address":"tv.home"}}""",
            Event.ServerRemoved(ServerRef(SERVER)) to
                """{"type":"serverRemoved","content":{"serverId":"$SERVER"}}""",
            Event.PasswordSignInSubmitted(PasswordSignIn(SERVER, "nora", "hunter2")) to
                """{"type":"passwordSignInSubmitted",""" +
                """"content":{"serverId":"$SERVER","username":"nora","password":"hunter2"}}""",
            Event.PairingStarted(ServerRef(SERVER)) to
                """{"type":"pairingStarted","content":{"serverId":"$SERVER"}}""",
            Event.PairingCancelled to """{"type":"pairingCancelled"}""",
            Event.LinkOpened(Link("couchverse://pair?code=WDJB-MJHT")) to
                """{"type":"linkOpened","content":{"url":"couchverse://pair?code=WDJB-MJHT"}}""",
            Event.AccountSelected(AccountRef(ACCOUNT)) to
                """{"type":"accountSelected","content":{"accountId":"$ACCOUNT"}}""",
            Event.SignOutRequested(AccountRef(ACCOUNT)) to
                """{"type":"signOutRequested","content":{"accountId":"$ACCOUNT"}}""",
            Event.DevicesOpened to """{"type":"devicesOpened"}""",
            Event.DeviceRevoked(DeviceRef("d2")) to """{"type":"deviceRevoked","content":{"deviceId":"d2"}}""",
            Event.PairingApprovalOpened(UserCode("WDJB-MJHT")) to
                """{"type":"pairingApprovalOpened","content":{"code":"WDJB-MJHT"}}""",
            Event.PairingApproved(PairingApproval("WDJB-MJHT", deviceName = "Bedroom")) to
                """{"type":"pairingApproved","content":{"code":"WDJB-MJHT","deviceName":"Bedroom"}}""",
            Event.PairingDenied(UserCode("WDJB-MJHT")) to
                """{"type":"pairingDenied","content":{"code":"WDJB-MJHT"}}""",
            Event.DisplayLanguageChanged(LanguageChoice("cs")) to
                """{"type":"displayLanguageChanged","content":{"code":"cs"}}""",
            Event.ScreenOpened(Surface.Home) to """{"type":"screenOpened","content":{"type":"home"}}""",
            Event.ScreenClosed(Surface.Title("glass-harbor")) to
                """{"type":"screenClosed","content":{"type":"title","content":"glass-harbor"}}""",
            Event.RefreshRequested(Surface.Browse(movies)) to
                """{"type":"refreshRequested","content":{"type":"browse","content":{"kind":"movie","sort":"name"}}}""",
            Event.BrowseMoreRequested(BrowseKey(genre = "Drama", sort = BrowseSort.Added)) to
                """{"type":"browseMoreRequested","content":{"genre":"Drama","sort":"added"}}""",
            Event.SearchChanged(SearchText("glass")) to """{"type":"searchChanged","content":{"query":"glass"}}""",
            Event.WatchlistChanged(WatchlistChange("t1", listed = true)) to
                """{"type":"watchlistChanged","content":{"titleId":"t1","listed":true}}""",
            Event.NoticeDismissed(NoticeRef(3u)) to """{"type":"noticeDismissed","content":{"id":3}}""",
            Event.AchievementsCheckRequested(CheckRequest(force = true)) to
                """{"type":"achievementsCheckRequested","content":{"force":true}}""",
            Event.CelebrationDismissed to """{"type":"celebrationDismissed"}""",
            Event.ProfileVisibilityChanged(PublicChoice(public = false)) to
                """{"type":"profileVisibilityChanged","content":{"public":false}}""",
            Event.ProfileEditSubmitted(ProfileEdit("Nora", "Hi")) to
                """{"type":"profileEditSubmitted","content":{"displayName":"Nora","bio":"Hi"}}""",
            Event.PasswordChangeSubmitted(PasswordForm("old", "new")) to
                """{"type":"passwordChangeSubmitted","content":{"current":"old","new":"new"}}""",
            Event.ImageChosen(ImageChoice(ImageSlot.Banner, "content://media/7")) to
                """{"type":"imageChosen","content":{"slot":"banner","file":"content://media/7"}}""",
            Event.ImageRemoved(ImageSlotRef(ImageSlot.Avatar)) to
                """{"type":"imageRemoved","content":{"slot":"avatar"}}""",
            Event.PlayRequested(PlayTarget(PlayKind.Episode, "e1")) to
                """{"type":"playRequested","content":{"kind":"episode","id":"e1"}}""",
            Event.PlayerReported(PlayerReport(12.5, 2400.0, playing = true, buffering = false)) to
                """{"type":"playerReported","content":{"positionSeconds":12.5,"durationSeconds":2400.0,"playing":true,"buffering":false}}""",
            Event.PlayerClosed to """{"type":"playerClosed"}""",
            Event.QualityChosen(QualityChoice("720p")) to """{"type":"qualityChosen","content":{"key":"720p"}}""",
            Event.AudioChosen(TrackChoice("a-cs")) to """{"type":"audioChosen","content":{"id":"a-cs"}}""",
            Event.SubtitlesChosen(TrackChoice()) to """{"type":"subtitlesChosen","content":{}}""",
            Event.NextEpisodeRequested to """{"type":"nextEpisodeRequested"}""",
            Event.NextEpisodeCancelled to """{"type":"nextEpisodeCancelled"}""",
            Event.ShuffleToggled to """{"type":"shuffleToggled"}""",
            Event.CapabilitiesReported(
                DeviceProfile(
                    containers = listOf(Container.Mp4),
                    video = listOf(VideoSupport(VideoCodec.Hevc, maxBitDepth = 10u)),
                    audio = listOf(AudioSupport(AudioCodec.Eac3, maxChannels = 6u)),
                    maxHeight = 2160u,
                    hls = listOf(HlsFormat.Fmp4),
                ),
            ) to
                """{"type":"capabilitiesReported","content":{"containers":["mp4"],""" +
                """"video":[{"codec":"hevc","maxBitDepth":10}],"audio":[{"codec":"eac3","maxChannels":6}],""" +
                """"maxHeight":2160,"hls":["fmp4"]}}""",
            Event.CouchStartRequested to """{"type":"couchStartRequested"}""",
            Event.CouchJoinRequested(CouchCode("123456")) to
                """{"type":"couchJoinRequested","content":{"code":"123456"}}""",
            Event.CouchRemoteRequested(CouchCode("123456")) to
                """{"type":"couchRemoteRequested","content":{"code":"123456"}}""",
            Event.CouchLeft to """{"type":"couchLeft"}""",
            Event.CouchEndRequested to """{"type":"couchEndRequested"}""",
            Event.CouchEmojiSent(CouchReaction("🍿")) to """{"type":"couchEmojiSent","content":{"emoji":"🍿"}}""",
            Event.CouchLocalPauseChanged(CouchPause(paused = true)) to
                """{"type":"couchLocalPauseChanged","content":{"paused":true}}""",
            Event.CouchRemoteCommanded(RemoteControl(RemoteAction.Seek, 12.5)) to
                """{"type":"couchRemoteCommanded","content":{"action":"seek","positionSeconds":12.5}}""",
            Event.DownloadRequested(DownloadAsk(PlayTarget(PlayKind.Movie, "m1"), DownloadQuality.Hd720, listOf("cs"))) to
                """{"type":"downloadRequested","content":{"target":{"kind":"movie","id":"m1"},"quality":"720p","audio":["cs"]}}""",
            Event.DownloadRetried(DownloadRef("d1")) to """{"type":"downloadRetried","content":{"id":"d1"}}""",
            Event.DownloadRemoved(DownloadRef("d1")) to """{"type":"downloadRemoved","content":{"id":"d1"}}""",
            Event.DownloadPlayRequested(DownloadRef("d1")) to
                """{"type":"downloadPlayRequested","content":{"id":"d1"}}""",
        )
        assertEncodings(Event.serializer(), events)
        for ((event, _) in events) {
            // a fresh core per event: the point is that Rust decodes it, which throws otherwise
            bridge().use { it.send(CoreJson.encodeToString(Message(1u, event))) }
        }
    }

    @Test
    fun `every surface encodes as the core expects`() {
        val surfaces = listOf(
            Surface.App to """{"type":"app"}""",
            Surface.Servers to """{"type":"servers"}""",
            Surface.Accounts to """{"type":"accounts"}""",
            Surface.SignIn to """{"type":"signIn"}""",
            Surface.Devices to """{"type":"devices"}""",
            Surface.PairingApproval to """{"type":"pairingApproval"}""",
            Surface.Session to """{"type":"session"}""",
            Surface.Markdown("*hi*") to """{"type":"markdown","content":"*hi*"}""",
            Surface.Home to """{"type":"home"}""",
            Surface.Browse(BrowseKey(TitleKind.Series, "Drama", BrowseSort.Year)) to
                """{"type":"browse","content":{"kind":"series","genre":"Drama","sort":"year"}}""",
            Surface.Title("glass-harbor") to """{"type":"title","content":"glass-harbor"}""",
            Surface.Genres to """{"type":"genres"}""",
            Surface.MyList to """{"type":"myList"}""",
            Surface.Search to """{"type":"search"}""",
            Surface.Notices to """{"type":"notices"}""",
            Surface.Rank to """{"type":"rank"}""",
            Surface.Profile("nora") to """{"type":"profile","content":"nora"}""",
            Surface.Leaderboard(LeaderboardKey(Period.Month, Metric.Xp)) to
                """{"type":"leaderboard","content":{"period":"month","metric":"xp"}}""",
            Surface.ProfileEditor to """{"type":"profileEditor"}""",
            Surface.Player to """{"type":"player"}""",
            Surface.Couch to """{"type":"couch"}""",
            Surface.Downloads to """{"type":"downloads"}""",
        )
        assertEncodings(Surface.serializer(), surfaces)
        bridge().use { bridge ->
            for ((surface, _) in surfaces) bridge.view(CoreJson.encodeToString(surface))
        }
    }

    @Test
    fun `every effect output encodes as the core expects`() {
        val outputs = listOf(
            EffectOutput.Http(HttpResponse(204u, "")) to """{"type":"http","content":{"status":204,"body":""}}""",
            EffectOutput.HttpFailed(HttpFailure(HttpFailureKind.Tls, "bad cert")) to
                """{"type":"httpFailed","content":{"kind":"tls","message":"bad cert"}}""",
            EffectOutput.TimerFired to """{"type":"timerFired"}""",
            EffectOutput.Stored(StoredValue("[]")) to """{"type":"stored","content":{"value":"[]"}}""",
            EffectOutput.Stored(StoredValue()) to """{"type":"stored","content":{}}""",
            EffectOutput.StoreDone to """{"type":"storeDone"}""",
            EffectOutput.StoreFailed(StoreFailure("disk full")) to
                """{"type":"storeFailed","content":{"message":"disk full"}}""",
            EffectOutput.SocketOpened to """{"type":"socketOpened"}""",
            EffectOutput.SocketText(SocketText("{}")) to """{"type":"socketText","content":{"text":"{}"}}""",
            EffectOutput.SocketClosed(SocketClosed(1006.toUShort(), "gone")) to
                """{"type":"socketClosed","content":{"code":1006,"reason":"gone"}}""",
            EffectOutput.DownloadProgress(DownloadProgress(250u, 1000u)) to
                """{"type":"downloadProgress","content":{"receivedBytes":250,"totalBytes":1000}}""",
            EffectOutput.DownloadFinished(DownloadFinished(1000u)) to
                """{"type":"downloadFinished","content":{"bytes":1000}}""",
            EffectOutput.DownloadFailed(DownloadFailure("disk full", noSpace = true)) to
                """{"type":"downloadFailed","content":{"message":"disk full","noSpace":true}}""",
        )
        assertEncodings(EffectOutput.serializer(), outputs)
        bridge().use { bridge ->
            for ((output, _) in outputs) {
                // nothing waits for effect 99, but the resolution is decoded first
                assertEquals("[]", bridge.resolve(CoreJson.encodeToString(Resolution(1u, 99u, output))))
            }
        }
    }

    @Test
    fun `ids and clocks cross as unsigned 64-bit numbers`() {
        val lastSafe = (1uL shl 53) - 1u
        val message = Message(lastSafe, Event.AppStarted, wallMs = 1_790_000_000_000u)
        assertSameJson(
            """{"nowMs":9007199254740991,"event":{"type":"appStarted"},"wallMs":1790000000000}""",
            CoreJson.encodeToString(message),
        )
        bridge().use { bridge ->
            val effects = roundTrip(EFFECTS, bridge.send(CoreJson.encodeToString(message)))
            assertEquals(listOf(1uL, 2uL, 3uL), effects.map { it.id })
            val resolution = Resolution(lastSafe, ULong.MAX_VALUE, EffectOutput.TimerFired)
            assertEquals("[]", bridge.resolve(CoreJson.encodeToString(resolution)))
        }
    }

    @Test
    fun `session views survive a round trip`() {
        signedIn().use { shell ->
            assertEquals(AppView(AppPhase.Ready, activeAccount = ACCOUNT), shell.view<AppView>(Surface.App))
            val session = shell.view<SessionView>(Surface.Session)
            assertEquals(LoadStatus.Loaded, session.status)
            assertEquals("nora", session.user?.username)
            assertEquals("cs", session.language)
            assertEquals(Features(couch = true, rankings = false, downloads = true), session.features)
            assertEquals("#3a6ea5", session.accent.accent)
            assertEquals(ACCOUNT, shell.view<AccountsView>(Surface.Accounts).accounts.single().id)
            assertEquals(BASE, shell.view<ServersView>(Surface.Servers).servers.single().url)

            shell.send(Event.DevicesOpened)
            shell.respond(
                "GET",
                "$API/me/devices",
                """[{"id":"d1","name":"Pixel","platform":"android","kind":"device","current":true,""" +
                    """"createdAt":"2026-09-01T10:00:00Z","lastSeenAt":"2026-10-02T09:00:00Z"}]""",
            )
            assertEquals(
                DeviceCard("d1", "Pixel", "android", "2026-10-02T09:00:00Z", current = true),
                shell.view<DevicesView>(Surface.Devices).devices.single(),
            )
        }
    }

    @Test
    fun `catalog views survive a round trip`() {
        signedIn().use { shell ->
            shell.send(Event.ScreenOpened(Surface.Home))
            val authorization = shell.request("GET", "$API/home?lang=cs").headers.first { it.name == "Authorization" }
            assertEquals("Bearer tok-2", authorization.value)
            shell.respond("GET", "$API/home?lang=cs", fixture("catalog/home"))
            val home = shell.view<HomeView>(Surface.Home)
            assertEquals(LoadStatus.Loaded, home.status)
            assertEquals("Glass Harbor", home.featured.single().name)
            val resume = home.rows.first { it.kind == HomeRowKind.ContinueWatching }.continueWatching.single()
            assertEquals(PlayTarget(PlayKind.Episode, "e2"), resume.play)
            assertEquals(0.25, resume.progress)

            val title = Surface.Title("glass-harbor")
            shell.send(Event.ScreenOpened(title))
            shell.respond("GET", "$API/titles/glass-harbor?lang=cs", fixture("catalog/title"))
            val detail = assertNotNull(shell.view<TitleView>(title).detail)
            assertEquals(Quality.Uhd, detail.quality)
            val episodes = detail.seasons.single().episodes
            assertEquals(listOf(1.0, 700.0 / 2400), episodes.map { it.progress })
            val play = PlayAction(PlayTarget(PlayKind.Episode, "e2"), 700u, EpisodeNumber(season = 1u, episode = 2u))
            assertEquals(play, detail.play)

            val movies = BrowseKey(kind = TitleKind.Movie, sort = BrowseSort.Name)
            shell.send(Event.ScreenOpened(Surface.Browse(movies)))
            shell.respond("GET", "$API/titles?lang=cs&kind=movie&sort=name&page=1", fixture("catalog/browse"))
            val browse = shell.view<BrowseView>(Surface.Browse(movies))
            assertEquals(movies, browse.key)
            assertEquals(listOf("a", "b"), browse.cards.map { it.titleId })
            assertEquals(3uL, browse.total)
            assertTrue(browse.more)
            shell.send(Event.BrowseMoreRequested(movies))
            shell.request("GET", "$API/titles?lang=cs&kind=movie&sort=name&page=2")
            assertTrue(shell.view<BrowseView>(Surface.Browse(movies)).loadingMore)
        }
    }

    @Test
    fun `ranks views and uploads survive a round trip`() {
        signedIn(rankings = true).use { shell ->
            shell.respond("POST", "$API/me/achievements/check", fixture("ranks/check"))
            val rank = shell.view<RankView>(Surface.Rank)
            assertEquals("rookie", rank.rank?.tier?.code)
            assertEquals(140uL, rank.rank?.xp)
            assertEquals("first_play", rank.celebration?.code)

            val board = Surface.Leaderboard(LeaderboardKey(Period.Week, Metric.Watch))
            shell.send(Event.ScreenOpened(board))
            shell.respond("GET", "$API/leaderboard?period=week", fixture("ranks/leaderboard"))
            val leaders = shell.view<LeaderboardView>(board)
            assertEquals(listOf("nora", "a"), leaders.rows.map { it.username })
            assertEquals(1u, leaders.myPosition)

            shell.send(Event.ImageChosen(ImageChoice(ImageSlot.Avatar, "content://media/42")))
            val upload = shell.upload("POST", "$API/me/avatar")
            assertEquals("content://media/42" to "file", upload.file to upload.field)
            assertEquals("Bearer tok-2", upload.request.headers.first { it.name == "Authorization" }.value)
            assertEquals(null, upload.request.body)
            shell.respond(
                "POST",
                "$API/me/avatar",
                """{"id":2,"username":"nora","displayName":"Nora","role":"user","bio":"",""" +
                    """"disabled":false,"createdAt":"2026-01-01T00:00:00Z","avatarId":"av-new"}""",
            )
            assertEquals(LoadStatus.Loaded, shell.view<ProfileEditorView>(Surface.ProfileEditor).avatar.status)
            assertEquals("av-new", shell.view<SessionView>(Surface.Session).user?.avatarId)
        }
    }

    @Test
    fun `every markdown block and inline survives a round trip`() {
        val source = """
            |# Title
            |
            |Some **strong**, *emphasis*, ~~strike~~, `code` and [a link](https://example.com).
            |A new line
            |
            |> quoted
            |
            |3. three
            |4. four
            |
            |```
            |let x = 1;
            |```
            |
            |---
            |
            || a | b |
            ||---|---|
            || 1 | 2 |
        """.trimMargin()
        Shell().use { shell ->
            val doc = shell.view<MarkdownDoc>(Surface.Markdown(source))
            assertEquals(Block.Heading(HeadingBlock(1u, listOf(Inline.Text("Title")))), doc.blocks.first())
            val tags = tags(CoreJson.encodeToJsonElement(MarkdownDoc.serializer(), doc))
            val expected = variants(Block.serializer()) + variants(Inline.serializer())
            assertTrue(tags.containsAll(expected), "missing ${expected - tags}")
        }
    }

    /**
     * A shell with in-memory stores over the raw bridge, so every effect batch and view model is
     * checked against the core's own JSON. Store reads are answered from the maps once a batch is
     * absorbed; HTTP requests wait for [respond].
     */
    private class Shell : AutoCloseable {
        val store = mutableMapOf<String, String>()
        val secure = mutableMapOf<String, String>()
        private val http = mutableMapOf<ULong, HttpRequest>()
        private val uploads = mutableMapOf<ULong, UploadRequest>()
        private val bridge = bridge()
        private var now = 1_000uL

        fun send(event: Event) = absorb(bridge.send(CoreJson.encodeToString(Message(now, event))))

        fun request(method: String, url: String): HttpRequest = outstanding(method, url).value

        fun upload(method: String, url: String): UploadRequest = uploads.getValue(outstanding(method, url).key)

        /** Answers an outstanding request or upload with a 200 and [body]. */
        fun respond(method: String, url: String, body: String) {
            val id = outstanding(method, url).key
            http.remove(id)
            uploads.remove(id)
            resolve(id, EffectOutput.Http(HttpResponse(200u, body)))
        }

        inline fun <reified T> view(surface: Surface): T =
            roundTrip(serializer<T>(), bridge.view(CoreJson.encodeToString(surface)))

        override fun close() = bridge.close()

        private fun outstanding(method: String, url: String) =
            http.entries.firstOrNull { (_, request) -> request.method == method && request.url == url }
                ?: error("no outstanding $method $url; outstanding: ${http.values.map { "${it.method} ${it.url}" }}")

        private fun resolve(id: ULong, output: EffectOutput) {
            now += 10u
            absorb(bridge.resolve(CoreJson.encodeToString(Resolution(now, id, output))))
        }

        private fun absorb(json: String) {
            val reads = mutableListOf<Pair<ULong, String?>>()
            for (request in roundTrip(EFFECTS, json)) {
                when (val effect = request.effect) {
                    is Effect.Store -> apply(store, request.id, effect.content)?.let { reads += it }
                    is Effect.SecureStore -> apply(secure, request.id, effect.content)?.let { reads += it }
                    is Effect.Http -> http[request.id] = effect.content
                    is Effect.Upload -> {
                        http[request.id] = effect.content.request
                        uploads[request.id] = effect.content
                    }
                    is Effect.Timer, is Effect.CancelTimer, is Effect.Render, is Effect.Player, is Effect.Socket, is Effect.Download -> Unit
                }
            }
            for ((id, value) in reads) resolve(id, EffectOutput.Stored(StoredValue(value)))
        }

        /** Applies a write or delete; returns a read to answer. */
        private fun apply(
            map: MutableMap<String, String>,
            id: ULong,
            request: StoreRequest,
        ): Pair<ULong, String?>? {
            when (val op = request.op) {
                is StoreOp.Write -> map[request.key] = op.content
                StoreOp.Delete -> map.remove(request.key)
                StoreOp.Read -> return id to map[request.key]
            }
            return null
        }
    }

    private companion object {
        const val SERVER = "4f6c0a5e-6a43-4c0e-9d4b-2b8f8d0b7a11"
        const val ACCOUNT = "$SERVER/2"
        const val BASE = "https://media.example.com"
        const val API = "$BASE/api/v1"
        val EFFECTS = ListSerializer(EffectRequest.serializer())

        fun bridge() = CoreBridge(
            CoreJson.encodeToString(CoreConfig(Platform.Android, AuthMode.Bearer, "Pixel", "en-GB")),
        )

        /** A phone that resumes its signed-in account, with the session loaded in Czech. */
        fun signedIn(rankings: Boolean = false): Shell {
            val shell = Shell()
            shell.store["servers"] = """[{"id":"$SERVER","url":"$BASE","name":"Home Media","version":"1.4.0",""" +
                """"apiLevel":1,"accent":"#3a6ea5","insecure":false}]"""
            shell.store["accounts"] = """{"accounts":[{"id":"$ACCOUNT","serverId":"$SERVER","userId":2,""" +
                """"username":"nora","displayName":"Nora","avatarId":"av-2","artworkGrant":"g-art"}],""" +
                """"active":"$ACCOUNT"}"""
            shell.secure["token.$ACCOUNT"] = "tok-2"
            shell.send(Event.AppStarted)
            shell.respond(
                "GET",
                "$API/auth/me",
                """{"id":2,"username":"nora","displayName":"Nora","role":"user","bio":"",""" +
                    """"disabled":false,"createdAt":"2026-01-01T00:00:00Z","avatarId":"av-2"}""",
            )
            shell.respond("GET", "$API/features", """{"couchEnabled":true,"rankingsEnabled":$rankings,"downloadsEnabled":true}""")
            shell.respond("GET", "$API/me/preferences", """{"language":"cs"}""")
            shell.respond(
                "GET",
                "$API/server",
                """{"id":"$SERVER","name":"Home Media","version":"1.4.0","apiLevel":1,"accent":"#3a6ea5"}""",
            )
            return shell
        }

        /** An API response body from src/test/resources. */
        fun fixture(path: String) = checkNotNull(WireFormatTest::class.java.getResource("/$path.json")).readText()

        /** Checks that [cases] cover every variant of a sealed class and encode as given. */
        fun <T> assertEncodings(serializer: KSerializer<T>, cases: List<Pair<T, String>>) {
            assertEquals(variants(serializer), cases.map { (_, json) -> tag(json) }.toSet())
            for ((value, json) in cases) {
                assertSameJson(json, CoreJson.encodeToString(serializer, value))
                assertEquals(value, CoreJson.decodeFromString(serializer, json))
            }
        }

        /** Decodes the core's [json] and checks that encoding the value again gives it back. */
        fun <T> roundTrip(serializer: KSerializer<T>, json: String): T {
            val value = CoreJson.decodeFromString(serializer, json)
            assertSameJson(json, CoreJson.encodeToString(serializer, value))
            return value
        }

        /** Numbers compare by value: Kotlin and Rust spell some doubles differently (`1.0E-4`). */
        fun assertSameJson(expected: String, actual: String) = assertEquals(
            canonical(CoreJson.parseToJsonElement(expected)),
            canonical(CoreJson.parseToJsonElement(actual)),
        )

        fun canonical(json: JsonElement): JsonElement = when (json) {
            is JsonObject -> JsonObject(json.mapValues { (_, value) -> canonical(value) })
            is JsonArray -> JsonArray(json.map(::canonical))
            is JsonPrimitive -> {
                val number = if (json.isString) null else json.content.toBigDecimalOrNull()
                if (number == null) json else JsonPrimitive(number.stripTrailingZeros())
            }
        }

        fun tag(json: String) = CoreJson.parseToJsonElement(json).jsonObject.getValue("type").jsonPrimitive.content

        /** The serial names of a sealed class's variants. */
        @OptIn(ExperimentalSerializationApi::class)
        fun variants(serializer: KSerializer<*>): Set<String> =
            serializer.descriptor.getElementDescriptor(1).elementNames.toSet()

        /** Every variant tag anywhere in [json]. */
        fun tags(json: JsonElement): Set<String> = when (json) {
            is JsonObject -> json.values.flatMap(::tags).toSet() +
                setOfNotNull((json["type"] as? JsonPrimitive)?.content)
            is JsonArray -> json.flatMap(::tags).toSet()
            else -> emptySet()
        }
    }
}
