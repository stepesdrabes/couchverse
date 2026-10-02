# Spike S2: core bridge end to end

Status: done (2026-10-02). Throwaway code lived in `/tmp/cv-spike-s2`; this records what the real
`core/` follows. Plan references: 7.1 (bridge), 7.7 (bindings and packaging).

## Verdict

The bridge shape from 7.1 works unchanged on every target: Rust scenario tests through the JSON
bridge, `swift test` on macOS (Swift 6 mode, warnings as errors), a SwiftUI app on the iOS 27 and
tvOS 27 simulators (URLSession fetch -> resolve -> Render -> view), iOS/tvOS device slices (link
only), wasm in Node, Android `.so` for arm64-v8a/armeabi-v7a/x86_64 (16 KB aligned), and the UniFFI
Kotlin bindings plus typeshare Kotlin types on the host JVM. No nightly Rust is needed: rustup ships
prebuilt `rust-std` for `aarch64-apple-tvos(-sim)`.

## Versions

rustc 1.97.1, uniffi 0.32.2, wasm-bindgen 0.2.129 (crate pinned with `=`, CLI must match),
typeshare-cli 1.13.4 with `typeshare-annotation` 1.0.5, serde 1.0.229, serde_json 1.0.151,
binaryen (wasm-opt) 133, cargo-ndk 4.1.2 with NDK r29 (`--platform 31`), Xcode 27.0, Swift 6.4,
Kotlin 2.4.20, kotlinx-serialization-json 1.11.0, JNA 5.19.1.

## Bridge as built

- `CoreBridge { new(config), send(inbound), resolve(inbound), view(surface) }`, strings only; every
  method returns `Result` (`CoreError::InvalidMessage { reason }` becomes Swift `throws`, a Kotlin
  `CoreException`, a JS `Error`). Malformed JSON is an error, never a panic.
- UniFFI objects are `Arc<Self>` and must be `Send + Sync`: the bridge holds a
  `std::sync::Mutex<Bridge>` (never contended; poisoning tolerated with
  `unwrap_or_else(|p| p.into_inner())`). wasm takes `&mut self`; wasm-bindgen's borrow flag rejects
  re-entrancy.
- Stream effects: the id from `Socket::Open` is resolved repeatedly (`Opened`, `Frame`...); the
  core cancels with `Socket::Close { id }`; `Closed`/`Failed` terminates and unregisters the id;
  later resolves for it return `[]`. `Send`, `Close`, `Cancel` and `Render` are fire-and-forget.
- Wire example: `send {"nowMs":5,"event":{"type":"ScreenOpened","content":{"type":"Home"}}}` ->
  `[{"id":1,"effect":{"type":"Http","content":{...}}}]`.

## Recipes

Apple (static libs for ios, ios-sim, tvos, tvos-sim, darwin, then):

```sh
cargo run -q -p uniffi-bindgen --bin uniffi-bindgen-swift -- --swift-sources --headers --modulemap \
  --module-name CouchverseCoreFFI --modulemap-filename module.modulemap \
  target/aarch64-apple-darwin/release/libcouchverse_ffi.a target/apple/bindings
# headers go in Headers/CouchverseCoreFFI/ of every slice
xcodebuild -create-xcframework -library <lib> -headers target/apple/include ... -output CouchverseCoreFFI.xcframework
```

`crates/ffi/uniffi.toml`: `[bindings.swift] module_name = "CouchverseCore"`,
`ffi_module_name = "CouchverseCoreFFI"`, `ffi_module_filename = "CouchverseCoreFFI"`;
`[bindings.kotlin] package_name`, `cdylib_name = "couchverse_ffi"`. Package: tools-version 6.2,
platforms iOS 27 / tvOS 27 / macOS 26, `.binaryTarget(path:)`, Swift 6 mode, warnings as errors.

Web:

```sh
cargo build -p wasm --profile release-wasm --target wasm32-unknown-unknown
wasm-bindgen --target web --out-dir <pkg> --out-name couchverse_core <wasm>
wasm-opt -O3 --enable-bulk-memory --enable-nontrapping-float-to-int --enable-sign-ext \
  --enable-mutable-globals --enable-reference-types --enable-multivalue <in> -o <out>
```

Android: `cargo ndk -t arm64-v8a -t armeabi-v7a -t x86_64 --platform 31 -o <jniLibs> build --release -p ffi`,
then `uniffi-bindgen generate --library <host dylib> --language kotlin --no-format`.

## Gotchas

1. Do not pass `--xcframework` to `uniffi-bindgen-swift` for `-library` xcframeworks: it emits a
   `framework module` map and `canImport(CouchverseCoreFFI)` silently fails ("cannot find type
   'RustBuffer'"). Use a plain `module` map.
2. `uniffi-bindgen-swift` 0.32 has no `--no-format` and never formats; `--no-format` belongs to
   `uniffi-bindgen generate`. The swift-format hang from the plan was not reproduced.
3. Xcode 27 merges every xcframework's `Headers/` into one include dir: a top-level
   `module.modulemap` collides ("Multiple commands produce"), so headers live in
   `Headers/<ModuleName>/`. The Clang module name must differ from the Swift module name.
4. The ffi crate uses `uniffi = { default-features = false }`; only the bindgen crate enables
   `["cli", "cargo-metadata"]` (otherwise cargo_metadata is compiled into every mobile slice).
5. Build the bindgen binaries in the dev profile (release with fat LTO took 13 minutes).
6. Name the library `couchverse_ffi`, never `ffi` (collides with the system libffi).
7. Depend on `typeshare = { package = "typeshare-annotation", version = "1.0.5" }`; the `typeshare`
   crate drags chrono and core-foundation into the core.
8. Parallel `cargo build --target` runs block on the build-dir lock: build targets sequentially.
9. A Rust panic kills the wasm instance (`panic = "abort"` traps); the web shell must recreate the
   core. Native keeps unwind, so UniFFI reports panics as internal errors.
10. Regenerate Swift/Kotlin bindings in the same build as the libraries: UniFFI checks API
    checksums on the first call.

## typeshare rules

- 64-bit integers are rejected: use `pub type U53 = u64;` (typeshare maps the name `U53` to Swift
  `UInt64`, Kotlin `ULong`, TS `number`) and `I54` for signed values.
- Enums with data must be adjacently tagged (`tag = "type", content = "content"`); this decodes
  identically in Swift (custom Codable), Kotlin (sealed class, kotlinx discriminator `type`) and
  TS (discriminated union). Unit-only enums are plain strings (no tag/content). Decide each enum's
  shape up front: adding a data variant later changes the wire format.
- Prefer newtype variants wrapping named structs over struct variants (clumsy generated names).
- Generic `LoadState<T>` works in Swift and TS; Kotlin needs a post-process
  (`sealed class X<out T>` and unit variants as `X<Nothing>()`); never name a variant `Nothing`.
- Options: add `skip_serializing_if = "Option::is_none"` so TS optional fields match; an Option
  inside a collection loses nullability in TS. HashMap works; tuples are rejected; send dates as
  RFC 3339 strings; macro-generated types are invisible.
- Swift output needs `[swift] default_decorators = ["Sendable", "Hashable"]` and
  `default_generic_constraints = ["Sendable", "Hashable"]` to compile in Swift 6 strict mode.

## Measurements (M-series Mac)

Home view model: 52.6 KB JSON (3.9 KB gzipped).

| | native Rust | wasm opt-level 3 | Swift (macOS) | JVM |
|---|---|---|---|---|
| `send` | 1-1.5 us | 5-15 us | - | - |
| `resolve` 64 KB body | 180-230 us | 440-545 us | - | - |
| `view(Home)` | 48-53 us | 67-79 us | 50-68 us | 190-530 us |
| shell decode | - | `JSON.parse` 150-170 us | `JSONDecoder` 1.0-1.8 ms | 350-900 us |

wasm `couchverse_core_bg.wasm`: opt-level "z" + `wasm-opt -Oz` 68.8 KB gzip; opt-level 3 +
`wasm-opt -O3` 96.5 KB gzip (both far under the 250 KB budget). opt-level 3 halves `view`, so
the wasm profile uses opt-level 3.

The Rust side of the "home view model under 2 ms on Apple TV 4K (2nd gen)" budget is fine
(estimated 150-250 us on A12); Swift `JSONDecoder` is the real cost (estimated 3-6 ms on A12).
Mitigations: decode off the main actor, fine-grained `Render` (re-read one row, not the home),
lean view models. Measure on the device.

## Recommended layout and profiles

`core/crates/{app,api,ffi,wasm,uniffi-bindgen}` + `core/xtask`. `[profile.release]`: opt-level 3,
`lto = true`, `codegen-units = 1`, `strip = "debuginfo"`, panic unwind. `[profile.release-wasm]`:
inherits release, `panic = "abort"`, `strip = true`.
