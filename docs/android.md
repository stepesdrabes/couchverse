# Android

The Android and Google TV client (plan section 11) is the Gradle project in `clients/android/`.
Today it holds the two library modules every later one builds on; the app and the `feature-*`
modules arrive in Phase 11.

| Module | Contents |
|---|---|
| `core` | The shared Rust core: `libcouchverse_ffi.so` per ABI, the UniFFI bindings (`io.stepes.couchverse.core.ffi.CoreBridge`), the generated message types (`io.stepes.couchverse.core`, kotlinx-serialization) and `Core`, which speaks the bridge in those types. |
| `design` | The generated design tokens (`Tokens.kt`: Compose colours, dimensions, motion) and string catalogs (`res/values{,-cs}/strings.xml`). |

Versions live in `gradle/libs.versions.toml`; minSdk is 31, compile and target SDK 37.

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
make android-test        # the same, then the JVM tests of the bindings
cd clients/android && ./gradlew assembleDebug
```

- `make core-android` writes build output, never committed: the libraries to
  `core/src/main/jniLibs/`, the bindings and a host build of the core to `core/build/`. A
  `./gradlew clean` therefore removes the last two; the build stops early (`checkRustCore`) and
  asks for `make core-android` whenever anything is missing.
- Rebuild with `make core-android` after every change to the core: UniFFI compares API
  checksums on the first call, so the bindings and the libraries must come from the same build.
- The JVM unit tests load the host build through JNA (`jna.library.path`), so the real core runs
  without an emulator.
- The message types (`core/src/generated/`) and the design files (`design/src/generated/`) come
  from `make contract` and are committed; never edit them by hand.
