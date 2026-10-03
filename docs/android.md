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
| `feature-catalog` | Home, Movies, Series, Genres, My List, Search, the title page and the playback placeholder. |
| `feature-settings` | Servers, accounts, the display language, devices and approving another device. |
| `app` | `CouchverseApp` (the runtime, Coil, lifecycle events), `MainActivity`, the root navigation by app phase, the phone tabs and the TV sidebar, deep links and notices. |
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
  to sign in again), `player`, a no-op until Phase 12's Media3 player, and `download`, whose
  start fails ("downloads are not supported yet") until the downloads slice, with nothing to
  cancel or remove meanwhile. Store effects run one at a time in order, so a read always sees
  earlier writes.
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
`https://<server>/pair?code=...`) opens the approval, `title/<slug>` opens a title once signed in.

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
- CI (`android.yml`): Linux builds the core, runs every JVM test, Android lint and a debug APK;
  a macOS job verifies the screenshots against the same libraries and bindings.

## Running against a local server

`docker compose up db -d` and `make run-backend` start the server on :8080; an emulator reaches
the host as `10.0.2.2`, so the server address is `http://10.0.2.2:8080` (marked not
encrypted). A TV emulator signs in by pairing: approve the code from the web's `/pair` page, or
from the phone app (Settings, Sign in a device).

## Real-device checklist

The emulators cover the flows; these need hardware before a release:

- **Google TV device** (Chromecast with Google TV or Google TV Streamer, which runs a 32-bit
  userland on older models): the app appears in the launcher with its banner; "Who's watching?"
  takes focus on the last account and the remote moves between tiles; the chosen avatar flies
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

## Known gaps (Phase 12)

Playback (the play button opens a placeholder), couch, ranks and profiles, downloads, the
widget and Watch Next. "Who's watching?" tints each account with its identicon's hue rather
than its banner's accent and shows no rank ring yet: `AccountCard` carries neither.
