# Android

The Android and Google TV client (plan section 11) is the Gradle project in `clients/android/`:
one app, `io.stepes.couchverse`, with a phone UI and a TV UI over shared modules. The TV UI is
picked once at launch from `UiModeManager`; `LEANBACK_LAUNCHER` and an optional
`android.software.leanback` put the same APK on Google TV.

| Module | Contents |
|---|---|
| `core` | The shared Rust core: `libcouchverse_ffi.so` per ABI, the UniFFI bindings (`io.stepes.couchverse.core.ffi.CoreBridge`), the generated message types (`io.stepes.couchverse.core`, kotlinx-serialization) and `Core`, which speaks the bridge in those types. `runtime/` runs it for the app (below); `device/` measures the playback capabilities. |
| `design` | The generated tokens (`Tokens.kt`) and string catalogs (`res/values{,-cs}/strings.xml`), the phone (Material 3) and TV (Compose for TV) themes, motion, components and the runtime glue for screens (`LocalCoreRuntime`, `rememberSurface`). |
| `feature-accounts` | Welcome, adding a server, the QR scanner (phone), password sign-in, pairing, connect links, "Who's watching?" and the phone's account switcher. |
| `feature-catalog` | Home, Movies, Series, Genres, My List, Search and the title page (with the download buttons on phones). |
| `feature-playback` | The Media3 player (`PlaybackEngine`, the `player` effect's executor), the phone and TV player screens, the media session and picture-in-picture. |
| `feature-downloads` | The `download` effect's WorkManager executor, the Downloads screen and the download button with its quality choice (phones only). |
| `feature-couch` | Joining a couch, the couch panel, members, reactions, the remote and the ongoing-session notification. |
| `feature-ranks` | The rank chip, celebrations, profiles (heatmap, watch clock, achievements), leaderboards and the profile editor. |
| `feature-settings` | Servers, accounts, the display language, devices, approving another device, and the ways into the profile, leaderboards, downloads and joining a couch. |
| `app` | `CouchverseApp` (the runtime, the player, downloads, Coil, lifecycle events, the couch notification, Watch Next and the widget), `MainActivity`, the root navigation by app phase, the phone tabs and the TV sidebar, deep links and notices. |
| `testing` | The JVM test harness the UI modules share: Roborazzi devices and languages, fake artwork. |

A feature module keeps its screens in `phone/` and `tv/` packages behind one entry point per
screen (`HomeScreen`, `SignInScreen`), which picks the idiom from `LocalIsTv`. Screens take view
models and callbacks, so tests render them from fixtures; a `XxxRoute` beside each connects it
to the runtime. Build conventions (SDK levels, Compose, JVM test setup) live in
`build-logic/`; versions in `gradle/libs.versions.toml`. minSdk is 31, compile and target SDK 37.

## The runtime

`CoreRuntime` (`core/runtime/`) is the app's only stateful service and lives as long as the
process (`CouchverseApp`), so rotations and trips through the TV's home screen keep every view
model.

- Every bridge call runs on one serial dispatcher (a dedicated thread), stamped with
  `SystemClock.elapsedRealtime`. `nowMs()` is that clock, for countdowns against deadlines in
  view models (a pairing code's `expiresAtMs`). Messages also carry `wallMs`, the Unix time,
  for what depends on the date.
- Effects: `http` and `upload` over OkHttp (the picked file's content URI is the handle;
  `ContentUploadFiles` names the form part with an extension that matches its type), `socket`
  over OkHttp's WebSocket (frames reach the core in order; exactly one `socketClosed`),
  coroutine timers (one-shot and repeating, cancelled by `cancelTimer`), `store` as one file
  per key under `files/core/store`, `secureStore` sealed with AES-GCM under an Android Keystore
  key (`files/core/secure`; a value that no longer opens reads as missing, so the account asks
  to sign in again), `player` through a `PlayerExecutor` (the app's `PlaybackEngine`) and
  `download` through a `DownloadExecutor` (`WorkDownloads` on phones, `NoDownloads` on TVs,
  whose start fails). The runtime forwards a transfer's events in order and drops any after
  its terminal event or a cancel. Store effects run one at a time in order, so a read always
  sees earlier writes.
- View models are decoded on the core's thread and published as one `StateFlow` per surface,
  re-read only when a `render` names it; idle parametric surfaces are dropped after a while.
  Screens read them with `rememberSurface<T>(surface, open = true)`, which also sends
  `ScreenOpened`/`ScreenClosed` for catalog surfaces.
- At launch the app sends `AppStarted`, then `CapabilitiesReported` with a `DeviceProfile` built
  from `MediaCodecList`, the display's HDR types and the HDMI encodings (`measureDevice` +
  `deviceProfile`, a pure function with JVM tests); `AppBecameActive` on every return to the
  foreground.
- Plain http is allowed (D14) and user-installed CAs are trusted
  (`res/xml/network_security_config.xml`); the UI marks such servers "Not encrypted".

## Navigation and links

The root navigation follows `AppView.phase`: a splash while starting, Welcome, sign-in (or a
server chooser), "Who's watching?" (always first on a TV), and the signed-in screens keyed by
account, so a switch starts on a fresh home. Adding an account, the scanner, devices and
approvals sit above them. Phones get tabs (Home, Browse with Movies/Series/Genres, Search, My
List, the account); TVs a sidebar that expands with focus (the account, Home, Movies, Series,
Genres, My List, Search, Settings). Back on the TV's Home moves into the sidebar first.

On a TV every screen says where focus starts (`design/tv/`): `focusOnStart` on one element, or
`rememberScreenFocus()` with `Modifier.screenFocus(screen, key, start)` on each focusable
element of a screen that can be left and come back to. The latter keeps the focused key in the
screen's saved state, because navigation disposes a screen (and Compose's focus restorers with
it): Back from a title lands on the poster it was opened from. Focus moves only when a screen
lands, so refreshed content never pulls it back. Text fields take `remoteLeavesField`, since a
field keeps the arrow keys for its cursor once the keyboard has closed.

Links (`couchverse://`, a custom scheme because App Links need one verified domain):
`connect?server=...&code=...` signs this device in, `pair?code=...` (or a scanned
`https://<server>/pair?code=...`) opens the approval, `title/<slug>` opens a title once signed
in, `couch/<code>` (or a scanned `https://<server>/couch/<code>`) joins a couch session, and
`play/<movie|episode>/<id>` plays where it stopped (Watch Next and the widget).

## Playback

`PlaybackEngine` (`feature-playback`) is the `player` effect's executor and the app's one
ExoPlayer, created by the first `load` and released by `stop`. Commands arrive on the core's
thread and run on the main thread. The core decides everything (the source, the start, the
next episode, resume, what a couch follower does); the engine only carries it out:

- Sources: `file` and `hls` are URLs the core makes whole (the server writes paths), fetched
  through the app's OkHttp client; `download` is a file name in `files/downloads`.
  `maxHeight` caps track selection. A 4xx is not retried (a refused grant fails at once,
  so the core can refetch).
- Subtitles: a track with a URL is a sidecar WebVTT (`SubtitleConfiguration`); one without is
  in the stream and is chosen by language once the tracks are known. `selectAudio` takes the
  rendition's `index` among the stream's audio groups when it has one, its language otherwise.
- Reports go back as `PlayerReported` on every state change and about once a second while
  playing. `playing` is what the viewer asked for (play requested, ready or buffering), as on
  the web, so a host that stalls does not pause its followers. A failure reports `http_<code>`
  or Media3's error code name.

The player screen (`PlayerRoute`) sends `PlayRequested` (or `DownloadPlayRequested`) and
`PlayerClosed`, and draws `PlayerView`: qualities, audio, subtitles, episodes, the next-episode
card with its countdown, shuffle, and on the couch the panel, members and reactions. Phones get
touch controls, landscape, hidden system bars and picture-in-picture (entered automatically
while playing, shrinking from where the picture is; leaving the app otherwise pauses). TVs get
a remote layout: with the controls hidden, centre plays or pauses and left/right skip 10 s; Back closes a panel, then hides the
controls while playing, then leaves the player. `PlaybackService` is a `MediaSessionService` over the same player, so
the notification, lock screen, Bluetooth buttons and the TV's system controls work; the screen
starts it by connecting a `MediaController`.

## Couch

The couch rides on the runtime's `socket` effect; the core runs the session and the shell
follows `CouchView`:

- The root navigation follows the role, signed in or not: a follower is taken to the couch
  player, which plays whatever the core loads (the host's title, position and pause), and a
  remote to `CouchRemote` (play/pause, 10 s skips, previous/next). When the session ends the
  screen says so and closes after a few seconds.
- The host starts a couch from the player's couch panel, which shows the code and its QR (the
  web's `/couch/<code>` page). Joining is by code (Settings, Join a couch), a scanned QR or a
  link, as a viewer or a remote.
- Without an account, Welcome's "Join a couch session" asks for the server's address too and
  joins as a guest (no remote: a guest has no player of its own to steer). A
  `couchverse://couch/<code>?server=<address>` link (the web's "Open in the app") opens the
  form over any screen with both filled in, a scanned join page brings its origin as the
  server, and a link without a server waits for an account. The core joins through the
  account on its own server and as a guest anywhere else (`CouchCode.server`). The player
  shows its couch controls whenever a couch is live, so a guest's work without an account.
- Reactions rise over the video for everyone; the members list shows who is there and who is
  away.
- Phones keep an ongoing notification while in a session (promoted where the system allows),
  with Leave, or End for the host (`CouchActionReceiver` sends the event to the runtime).
  Android 13 and newer ask for the notification permission once per launch.

## Downloads and offline

Phones only; TVs never offer them (`NoDownloads`).

- `WorkDownloads` runs each `download` start as unique WorkManager work named after its file
  (`download:<name>`, keep the existing one), so a start after a relaunch, or one with an empty
  URL, attaches to the running transfer or reports the finished file. `DownloadWorker` is a
  foreground (`dataSync`) worker that resumes a partial file with a `Range` request, reports
  progress at most once a second and fails with `noSpace` when the disk fills. Its
  notification speaks the display language the download was asked in, which the work request
  carries, since the system's can differ (D21). Files live in
  `files/downloads/` (app-private; the app allows no backups), the partial ones beside them as
  `.part`.
- The title page offers a download button per movie and episode with a quality choice, hidden
  when the server turns downloads off (`SessionView.features.downloads`). Its states (queued,
  preparing, fetching with progress, ready, failed with retry) come from `DownloadsView`.
- The Downloads screen (from Settings) lists every download with its state, progress and size,
  and plays, retries or removes it. While `SessionView.offline` it replaces the tabs; progress
  watched offline is kept by the core and sent once the server is reachable.

## Ranks and profiles

From the core's `Rank`, `Profile`, `Leaderboard` and `ProfileEditor` surfaces, behind the
server's `rankingsEnabled`: the tier and level beside the profile row in Settings, a
celebration for each unlocked achievement, profiles (stats, the 26-week heatmap, the watch clock, the top title,
achievements), leaderboards with a podium and, pinned below the board while the viewer is
hidden from it or below its first three, where they stand (`LeaderboardView.me`, as the web
shows it), and the profile editor. Avatars and banners are
chosen with the system photo picker and sent through the `upload` effect; the bio is user
markdown, shown with the core's document tree. Achievement, tier and XP-source codes map to
strings through explicit tables (`Words.kt`), so R8 keeps every string; `WordsTest` checks
them against every code `contract/openapi.json` allows and every achievement the string
catalogs name.

"Who's watching?" draws each account's rank (`AccountCard.rank`) as a ring around its avatar
(`RankRing`: the tier's colour filled to the progress through the tier) and tints the glow
with its banner's palette (`AccountCard.accent`), or its identicon's hue without one. On a TV
focus brightens the ring and reveals the rank title; the phone's picker shows the title under
the name, and its switcher sheet glows in the active account's colour.

## Outside the app

- **Watch Next** (Google TV): the home's Continue Watching becomes the app's Watch Next
  programs, replaced as a whole whenever it changes; each opens `couchverse://play/...`.
- **Widget** (phones): a Glance widget with up to three unfinished titles and their progress,
  drawn from a snapshot the app writes (`files/widget/continue.json`) in the display language,
  so it never needs the core or the network.

## Building from source

Prerequisites:

- The Android SDK with platform 37 and build-tools 36 (Android Studio's SDK Manager, or
  `sdkmanager "platforms;android-37.0" "build-tools;36.0.0"`), found through `ANDROID_HOME` or
  `sdk.dir` in `clients/android/local.properties`.
- NDK r29: `sdkmanager "ndk;29.0.14206865"`. `make core-android` uses `ANDROID_NDK_HOME`, else
  the newest NDK in the SDK; Gradle strips the libraries with the catalog's `ndk` version.
- Rust through rustup, with the Android targets:
  `rustup target add aarch64-linux-android armv7-linux-androideabi x86_64-linux-android`
- cargo-ndk: `cargo install cargo-ndk`
- A JDK 17 or newer to launch Gradle. The build itself runs on JDK 21
  (`gradle/gradle-daemon-jvm.properties`): an installed one is picked up, otherwise Gradle
  downloads it.

Then, from the repository root:

```sh
make core-android        # the per-ABI libraries, the Kotlin bindings, the host library
make android-test        # the same, then every JVM test and the screenshot verification
cd clients/android && ./gradlew :app:assembleDebug
```

- `make core-android` writes build output, never committed: the libraries to
  `core/src/main/jniLibs/`, the bindings and a host build of the core to `core/build/`. A
  `./gradlew clean` therefore removes the last two; the build stops early (`checkRustCore`, and
  `checkRustHost` for the core's JVM tests) and asks for `make core-android` whenever anything
  is missing.
- Rebuild with `make core-android` after every change to the core: UniFFI compares API
  checksums on the first call, so the bindings and the libraries must come from the same build.
- The message types (`core/src/generated/`) and the design files (`design/src/generated/`) come
  from `make contract` and are committed; never edit them by hand.
- Builders who sign their own APKs can change the application id (D24):
  `./gradlew :app:assembleRelease -Pcouchverse.applicationId=org.example.couchverse
  -Pcouchverse.version=1.2.0`.

## Signing and releases

A release build (R8 with resource shrinking) is signed with the keystore named by
`COUCHVERSE_KEYSTORE`, with `COUCHVERSE_KEYSTORE_PASSWORD`, `COUCHVERSE_KEY_ALIAS` and
`COUCHVERSE_KEY_PASSWORD`. Without them it falls back to the debug key, so a build from source
still installs (but cannot update an APK signed with another key).

`release.yml` builds the APK on every `v*` tag and attaches `couchverse-<version>.apk` to the
GitHub release (version code `major * 10000 + minor * 100 + patch`). It reads the keystore from
repository secrets: `ANDROID_KEYSTORE` (the `.jks`, base64), `ANDROID_KEYSTORE_PASSWORD`,
`ANDROID_KEY_ALIAS` and `ANDROID_KEY_PASSWORD`. Without them the job warns and publishes a
debug-signed APK. To make a keystore once and store it:

```sh
keytool -genkeypair -v -keystore couchverse.jks -alias couchverse -keyalg RSA -keysize 4096 -validity 10000
base64 -i couchverse.jks | gh secret set ANDROID_KEYSTORE
gh secret set ANDROID_KEYSTORE_PASSWORD; gh secret set ANDROID_KEY_ALIAS; gh secret set ANDROID_KEY_PASSWORD
```

Keep the keystore safe: every later APK must be signed with it, or installed apps cannot update.

## Tests

- `core`: the real core on the JVM through JNA (`jna.library.path` points at the host build):
  the wire format, the runtime with in-memory stores, a scripted server and virtual time
  (timers, ordering, publishing, persistence across a relaunch), OkHttp against MockWebServer
  (requests, failures, uploads, WebSockets), the stores, and the device profile.
- UI modules: Compose UI tests on Robolectric (forms, D-pad focus on "Who's watching?", My
  List, play buttons, links) and Roborazzi screenshots of the key screens on a Pixel 7 and a
  1080p TV, in English and Czech, loaded, loading and failed, from fixture view models. The
  JVM tests run on Android 16 (`testing/src/main/resources/robolectric.properties`): Espresso,
  which Robolectric drives them through, does not run on 17 yet.
- Screenshots live in `<module>/src/test/screenshots/`. `./gradlew recordRoborazziDebug`
  records them, `verifyRoborazziDebug` compares (a 0.5 % pixel tolerance), and
  `compareRoborazziDebug` writes diffs to `build/outputs/roborazzi`. They are recorded on macOS
  (Apple silicon), and CI verifies them on a macOS runner: text rasterizes slightly differently
  on Linux.
- The player runs a real ExoPlayer on Robolectric against a local clip and MockWebServer
  (reports, commands, a refused URL failing without retries, the height cap, sidecar subtitles);
  downloads run WorkManager's test driver against MockWebServer (progress, resuming, a refusal,
  a relaunch finding the file or its absence, removal).
- CI (`android.yml`): Linux builds the core, runs every JVM test, Android lint, a debug APK and
  a release APK (R8); a macOS job verifies the screenshots against the same libraries and
  bindings.

## Running against a local server

`docker compose up db -d` and `make run-backend` start the server on :8080; an emulator reaches
the host as `10.0.2.2`, so the server address is `http://10.0.2.2:8080` (marked not
encrypted). A TV emulator signs in by pairing: approve the code from the web's `/pair` page, or
from the phone app (Settings, Sign in a device).

## Real-device checklist

The emulators cover the flows; these need hardware before a release:

- **Google TV device** (Chromecast with Google TV or Google TV Streamer, which runs a 32-bit
  userland on older models): the app appears in the launcher with its banner; "Who's watching?"
  takes focus on the last account and the remote moves between tiles, the glow following each
  account's banner colour and the focused ring brightening; the chosen avatar flies
  into the sidebar; D-pad reaches every control on Home, Movies, Series, Genres, My List,
  Search, the title page and Settings, and Back from a title lands on the poster it was opened
  from; Back on Home moves into the sidebar, then exits; the
  on-screen keyboard opens beside the add-server and password fields; the pairing QR code
  scans from a phone across the room and the countdown matches the server's expiry.
- **Phone camera**: scanning the web's "Connect a device" QR signs the phone in; scanning a
  TV's pairing QR opens the approval for the signed-in account; another QR code says it is not
  a Couchverse one; denying the camera and allowing it again from the prompt or system
  settings works.
- **Keystore persistence**: after a reboot and an app update the accounts are still signed in;
  after clearing the app's data, or a restore onto another device (the Keystore key does not
  travel), accounts show "Sign in again" instead of failing; tokens never appear in
  `files/core/store`.
- **Display language and scaling**: switching to Czech from Settings changes the UI and the
  catalog text at once; the largest font size and display size keep every screen usable.
- **TalkBack**: cards read their title and progress, the pairing code is read character by
  character, and the "Not encrypted" badge is announced.
- **Reduced motion** ("Remove animations"): the hero stops advancing, focus does not scale,
  and "Who's watching?" cross-fades.
- **Playback on a Google TV device**: Original and the ladder on the TV's own decoders (HEVC,
  HDR10 and Dolby Vision where the device has them; the display switching to the content's
  frame rate and range), AC-3/E-AC-3 passing through HDMI to a receiver, the remote's
  play/pause and skip keys, the system's media controls and "Continue watching" on the Google
  TV home opening the right episode at its position.
- **Playback on a phone**: picture-in-picture from the home gesture while playing and its
  play/pause action, the lock-screen and Bluetooth headset controls, rotation, a call or
  another app's audio pausing playback, audio continuing through a Bluetooth speaker.
- **Downloads on a phone**: a transfer that survives leaving the app, the screen turning off
  and a reboot (WorkManager resumes it), the progress notification, a full disk reported as
  such, playing a download in airplane mode and the progress reaching the server after
  reconnecting, a removal freeing the space.
- **Couch on hardware**: the ongoing notification (and its promoted chip on Android 16) with
  Leave/End, the notification permission prompt, a follower on a TV staying in sync with a
  phone host through pauses and seeks, a QR code joined from a phone's camera, a phone as a
  remote for a TV; with nothing signed in, Welcome's "Join a couch session" (the address typed
  with the TV's keyboard) and the web join page's "Open in the app" joining as a guest, the
  guest's avatars and backdrop loading from that server.
- **Widget**: placing it on the home screen, its titles following the app's display language,
  a tap playing the title, the list emptying after signing out.
- **Release APK**: installing the published APK over a debug build is refused (different
  keys); an update from one release to the next keeps the accounts.

## Known gaps

- A banner's accent reaches "Who's watching?" only through the account's profile, which the
  server serves with rankings on; with them off the account keeps its identicon's hue.
- TVs have no downloads (by design) and no widget; phones have no Watch Next.
- The player's quality menu lists the core's choices; ExoPlayer's own adaptive switching under
  "Auto" is not shown.
