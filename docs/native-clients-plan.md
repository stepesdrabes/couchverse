# Native clients and shared core - plan

Status: **approved** (2026-10-02)
Scope: iOS + iPadOS 27, tvOS 27, Android phone + Google TV, and the existing Svelte web app, all
on one shared, platform-independent client core.

This document is the single source of truth for the effort. It records every decision made
during planning (section 1), the target architecture, the backend work it requires, a phased
roadmap with exit criteria, and the risks and spikes to resolve early.

---

## 1. Decisions

Decisions agreed during planning. IDs are referenced throughout the document.

| ID | Decision |
|---|---|
| D1 | A **shared client core** is used by the web too (not only Apple). Motivation: less total code, one implementation of tricky logic, a typed API contract. The web UI stays Svelte. |
| D2 | **Platforms**: web (existing), iOS + iPadOS (universal), tvOS, Android phone, Google TV. Android is part of this plan, phased after Apple. |
| D3 | **Platform-native UI**: SwiftUI on Apple, Jetpack Compose (+ Compose for TV) on Android, Svelte on web. "Native + Couchverse soul" visual identity (section 12). |
| D4 | **Viewer only** on native, with the full viewer feature set. Admin stays web-only. |
| D5 | **Music is removed from Couchverse entirely** (backend, web, DB, ranks, analytics) in Phase 0. Earned music XP and music achievements are dropped. |
| D6 | **Backend may change freely**, provided the web keeps working (web migrated in the same change). |
| D7 | **Minimum OS**: iOS/iPadOS 27 and tvOS 27 only. Android minSdk 31 (Android 12). |
| D8 | **Playback ambition**: direct-play everything the device can decode (HEVC, HDR10, HLG, Dolby Vision, AC-3/E-AC-3/Atmos); remux before transcode. |
| D9 | **System players** with Couchverse overlays: AVPlayerViewController on Apple, Media3 on Android. **System captions**: subtitles become HLS WebVTT renditions; the per-account Couchverse subtitle style applies to web only. |
| D10 | **Accounts**: multiple servers per install, multiple accounts per server. TV always opens on a "Who's watching?" picker (with a showpiece animation); phones open the last account and switch from the profile tab. No profile PINs. |
| D11 | **Sign-in**: username + password, pairing with a code (TV shows code/QR, approved from a signed-in phone or web), anonymous couch join by 6-digit code. Servers are added by manual URL or by scanning a "Connect a device" QR shown by the web. No LAN discovery. |
| D12 | **Auth model**: device sessions with opaque bearer tokens (sliding expiry, named, revocable from a Devices list). Web keeps its httpOnly cookie. |
| D13 | **Media authorization**: signed, expiring, path-embedded **media grants** for streams, HLS, subtitles and frames; artwork accepts a grant too. Replaces the anonymous couch-cookie stream guard. |
| D14 | **Plain HTTP is allowed** to any host (users build from source); the UI marks unencrypted servers. |
| D15 | **iOS extras**: offline downloads (server-made device-ready MP4), PiP + AirPlay + Now Playing, Live Activity + widgets + App Intents/Spotlight. No CarPlay. |
| D16 | **tvOS extras**: Top Shelf, ranks and profiles on TV, couch hosting on TV with a big QR + code. Sidebar navigation. |
| D17 | **Couch extras**: QR + code on TV, "phone as remote" for the host, iOS Live Activity, SharePlay as a later optional mode. The WebSocket protocol stays primary. |
| D18 | **Profile editing** everywhere, TV included (images handed off to the phone, see 10.6). |
| D19 | **Bio markdown** is parsed by the core into a safe AST rendered natively by every client; the web drops `{@html}`. |
| D20 | **Title logos**: a new TMDB `logo` artwork kind for cinematic heroes on every client. |
| D21 | **Display language** follows the account preference (seeded from the OS language on first sign-in). |
| D22 | **Repo layout**: `backend/`, `core/`, `contract/`, `clients/{web,apple,android}` (the current `frontend/` moves to `clients/web/`). |
| D23 | **Distribution**: open source, built from source. Apple builds must work with a **free personal team** (no paid membership); extras that need paid capabilities degrade gracefully. Android: one app (phone + TV UI) as signed APKs on GitHub Releases. |
| D24 | **Identifiers**: root `io.stepes.couchverse` (overridable per builder in a gitignored config). |
| D25 | **Versioning**: one repo version for server and clients, plus an integer **API level** reported by the server; clients declare the minimum they need. |
| D26 | **Quality**: GitHub Actions CI for every component and layered tests (core unit + scenario + conformance, contract tests, snapshot tests, smoke UI tests). |
| D27 | **Clean code and architecture first**. Comments only where they explain a non-obvious *why*. Existing conventions (no em-dash, conventional commits, no co-author trailers) apply to every new codebase. |
| D28 | **Delivery shape**: thin vertical slices. Each core module lands together with its web adoption and its iOS + tvOS screens; Android follows the Apple slices. |
| D29 | **Core technology**: a hand-rolled, sans-I/O **Rust** core with a **message bridge**: typed messages (events in; effects and view models out) through four bridge functions, exported with UniFFI (Swift, Kotlin) and wasm-bindgen (web); message types generated for all three languages by typeshare. Not Crux, not KMP (7.1). |
| D30 | **Contract source**: code-first OpenAPI 3.1 generated from the Go handlers with **huma v2** on the existing chi router. |

---

## 2. Goals, non-goals, success criteria

**Goals**
- Superb, platform-native Couchverse apps for iPhone, iPad, Apple TV, Android phones and Google TV.
- One shared core that owns the client-side behaviour that must be identical everywhere:
  API access, session and accounts, caching, playback orchestration, couch sync, ranks
  orchestration, markdown policy.
- A typed API contract generated from the Go server, so a backend change breaks every client at
  build time instead of at runtime.
- An Apple-grade streaming pipeline (fMP4 HLS, HEVC/HDR/Atmos passthrough, I-frame
  playlists, subtitle renditions) that also benefits the web.

**Non-goals**
- Admin features on native (D4). Uploads, title editing, jobs, settings stay web-only.
- Music in any form (D5).
- Replacing Svelte on the web, or sharing UI code across platforms.
- App Store / Play Store publishing in this plan (the design does not preclude it).
- LAN auto-discovery, profile PINs, CarPlay, parental controls.

**Success criteria**
- A household can add a server by scanning a QR code, pick a profile on Apple TV and start a
  4K HDR title that direct-plays or remuxes without transcoding when the device supports it.
- A couch session started on Apple TV can be joined from iPhone, Android, Google TV and an
  anonymous web browser, all staying within the drift threshold.
- The core is the only implementation of playback orchestration, couch sync, caching and
  session logic; the web viewer runs on it.
- `make` + CI verify the backend, contract drift, core, web, Apple and Android builds and tests.

---

## 3. Platform baselines and toolchains

| Platform | Baseline | Notes |
|---|---|---|
| iOS / iPadOS | 27.0 | iPhone 11 / SE 2 and newer (same devices as iOS 26). Universal app. |
| tvOS | 27.0 | Apple TV 4K 2nd gen (2021) and 3rd gen (2022). Apple TV HD and 4K 1st gen are dropped by tvOS 27. |
| Android | minSdk 31, target/compile latest stable | Phones + Google TV (Chromecast with Google TV, Google TV Streamer). Ship `arm64-v8a`, `armeabi-v7a` (many TV devices run a 32-bit userland) and `x86_64` (emulator). |
| Web | unchanged | Browsers + Titan OS TV mode as today. |

Toolchains (all present on the dev Mac): Xcode 27.0 (iOS/tvOS 27 SDKs), Swift 6.4, Rust 1.97
(tvOS targets are Tier 2 since Rust 1.95), Go 1.25, Node 22, JDK (Gradle toolchain pinned to
21), Android SDK in `~/Android/sdk`, XcodeGen (not required, see 10.1).

Platform facts that shape the design (sources in Appendix A):
- tvOS has no WebView; JavaScriptCore exists but has no fetch/WebSocket/timers.
- Liquid Glass cannot be opted out of when building with the 27 SDKs; the design embraces it.
- Apps built with the 27 SDKs must use the scene life cycle (a pure SwiftUI `App` already does).
- tvOS gives apps only ~500 KB of guaranteed persistent storage; everything else is purgeable,
  so the TV keeps credentials in the Keychain and treats all other data as cache. No downloads
  on tvOS.
- tvOS 27 adds Dynamic Type: the TV UI uses text styles, never fixed font sizes.
- AVPlayer: MP4/MOV/HLS only (no MKV/WebM); no DTS, TrueHD, Opus, Vorbis; HEVC/AV1 in HLS
  require fMP4; I-frame playlists are required for tvOS scrubbing thumbnails; sidecar SRT is
  not supported, WebVTT renditions are. Custom HTTP headers on `AVURLAsset` are not a
  supported API, which is why media uses path-embedded grants (D13).
- Generated subtitles (on-device transcription) appear in AVKit automatically on 27.

---

## 4. Target architecture

### 4.1 Layers

```
                 +-----------------------------+
                 |  backend (Go)               |  features own handlers + SQL
                 |  huma operations -> OpenAPI |
                 +--------------+--------------+
                                | generates
                 +--------------v--------------+
                 |  contract/                  |  openapi.json, couch protocol schema,
                 |                             |  i18n catalog, design tokens, fixtures
                 +--------------+--------------+
                                | generates types for
                 +--------------v--------------+
                 |  core/ (Rust)               |  model, events, effects, view models
                 |  pure, no I/O               |  one implementation of client behaviour
                 +---+-----------+-----------+-+
        UniFFI/Swift |    wasm   |   UniFFI/Kotlin
          +----------v-+  +------v------+  +-v-------------+
          | clients/   |  | clients/    |  | clients/      |
          | apple      |  | web         |  | android       |
          | SwiftUI    |  | Svelte      |  | Compose + TV  |
          +------------+  +-------------+  +---------------+
            shells: render view models, execute effects (HTTP, WebSocket, timers,
            secure storage, player, downloads), own navigation and platform integration
```

### 4.2 The core is sans-I/O

The core never performs I/O. It is a deterministic state machine:

```
shell --send(event)-------------> core  --> effect requests (Http, Socket, Timer, SecureStore, Store, Player, Download, Render)
shell --resolve(effect id, out)-> core  --> more effect requests
shell --view(surface)-----------> core  --> that surface's view model (plain data)
```

Why this shape:
- **One binding surface for three languages.** Events, effects, effect outputs and view models
  are plain serializable data. The FFI is four functions over JSON strings, identical for
  Swift, Kotlin and wasm; the message types are generated per language.
- **Native networking everywhere.** URLSession, OkHttp and `fetch` do the HTTP: system TLS
  trust and proxies, background download sessions, cookie auth on the web, Local Network
  handling, no Rust TLS stack in the apps.
- **Testability.** Every behaviour (pairing, playback beacons, JIT keepalive, couch drift
  correction, cache revalidation) is tested in Rust by feeding events and asserting effects,
  with explicit time and no network.
- **The player stays native.** The core issues player commands and consumes coarse player
  events (state, position about once per second, seeks, end, errors). Frame-rate concerns never
  cross the boundary.

The bridge that carries these messages is described in 7.1.

### 4.3 Dependency rules

- Backend rules are unchanged (feature DAG in FEATURES.md).
- `contract/` is generated from the backend plus a few hand-authored sources (i18n, tokens,
  fixtures). Nothing in `contract/` is hand-edited if it is generated.
- `core/` depends only on `contract/` outputs. It knows nothing about any UI toolkit.
- Shells depend on the core's generated bindings, never on each other.
- **Feature names line up across layers**: backend feature `catalog` <-> core module
  `catalog` <-> `clients/*/.../catalog`. A reader can follow one feature top to bottom.
- Generated code lives in clearly named `generated/` locations, is never hand-edited, and is
  verified by a drift check in CI.

### 4.4 Engineering principles (D27)

- **Small, single-purpose units.** One type or one screen per file; feature folders, not
  layer folders (`catalog/TitleScreen.swift`, not `views/` + `viewmodels/`).
- **One source of truth per concern.** State lives in the core; shells hold only ephemeral UI
  state (focus, scroll, sheet presentation). No duplicated caches, no ad-hoc singletons.
- **Explicit boundaries.** The core speaks in domain terms (no "SwiftUI", "Compose", "DOM"
  or "AVPlayer" in its vocabulary); shells never parse API payloads.
- **Names over comments.** Comments only for a non-obvious *why* (a platform bug, a protocol
  constraint, a security boundary). No banners, no restating code, no TODO graveyards.
- **Formatting and linting are not optional**: `gofmt` + golangci-lint, rustfmt + clippy with
  warnings denied, `swift format` + Swift 6 strict concurrency, ktfmt + detekt + Android lint,
  prettier + eslint + svelte-check. CI also rejects the em-dash (U+2014) anywhere.
- **Tests describe behaviour**, named as sentences, with fixtures shared through `contract/`.

---

## 5. Repository layout

```
backend/                      Go server (unchanged structure)
contract/
  openapi.json                generated from the server (make contract)
  couch-protocol.schema.json  generated from the couch Go types
  i18n/{en,cs}.json           source of truth for UI strings (moved from frontend/messages)
  design/tokens.json          colours, radii, spacing, type ramp, motion
  fixtures/                   golden payloads shared by Go tests and core tests
core/
  Cargo.toml                  workspace
  crates/
    api/                      types + operation builders generated from openapi.json
    app/                      the core: modules, model, events, effects, view models
    ffi/                      UniFFI exports (Swift, Kotlin)
    wasm/                     wasm-bindgen exports (web)
  xtask/                      codegen and packaging (xcframework, Android .so, wasm pkg)
clients/
  web/                        the current frontend/ (moved)
  apple/                      Xcode project, Swift packages, app + extension targets
  android/                    Gradle project: app (phone + TV), feature and design modules
docs/                         plans and design docs (this file)
.github/workflows/            CI
```

The move of `frontend/` to `clients/web/` happens in Phase 0 together with the Makefile,
Dockerfile, `backend/web/dist` embed path, CLAUDE.md and FEATURES.md updates.

---

## 6. The contract layer

### 6.1 OpenAPI generated from Go (huma, code-first, D30)

The server adopts **huma v2** on the existing chi router (`humachi`), so the spec is derived
from the handler types and cannot drift.

- `internal/server` creates one `huma.API` for `/api/v1` and passes it (or a `huma.Group` with
  middleware) to each feature's `Mount*` methods. Features register typed operations next to
  their handlers: `huma.Register(api, operation, h.getTitle)` with
  `func(ctx, *GetTitleInput) (*GetTitleOutput, error)`.
- Auth groups map to huma groups with middleware (`RequireAuth`, `RequireAdmin`, flags), plus
  OpenAPI security schemes (`cookieSession`, `bearerDevice`, `mediaGrant`).
- `huma.NewError` is overridden so errors keep today's envelope `{"error":{"code","message"}}`.
  Request bodies reject unknown fields, as `httpx.Decode` does today.
- Byte-stream, WebSocket and multipart routes stay plain chi handlers and are described in the
  spec manually (paths, parameters, content types) so clients still get typed URLs.
- Migration order: viewer endpoints first (they feed the core), then admin endpoints (they feed
  the web admin's typed client). `httpx.Decode`/`httpx.JSON` remain for routes not yet migrated.
- `couchverse openapi` (a subcommand of the binary) prints the spec; `make contract` writes
  `contract/openapi.json` and runs all generators. CI fails if regeneration produces a diff.

### 6.2 Couch protocol schema

The WebSocket messages (`internal/feature/couch/protocol.go`) are registered as named schemas
and exported to `contract/couch-protocol.schema.json`, so the core's protocol types are
generated rather than restated. Golden frames in `contract/fixtures/couch/` are asserted by both
the Go `couch` tests and the core's couch tests.

### 6.3 i18n catalog

- `contract/i18n/{en,cs}.json` becomes the single source (the Paraglide project points at it).
- The format gains **plural variants** (Czech needs one/few/many: "1 sezóna", "2 sezóny",
  "5 sezón"); existing fixed forms are fixed in the move.
- Generators: Paraglide (web, as today), `Localizable.xcstrings` (Apple, with plural
  variations), `strings.xml` + `plurals` (Android). Admin-only keys are excluded from native
  outputs by key prefix.
- The core never returns user-facing text. It returns codes (error codes, achievement codes,
  tier codes) that shells localize.

### 6.4 Design tokens

`contract/design/tokens.json` holds colours (`bg #07080d`, `surface`, `surface-2`, `edge`,
`text`, `muted`, `faint`, default `accent #e50914`, `success`, `danger`), radii, spacing, the
type ramp intent and motion curves. Generators emit a CSS `@theme` include for the web (app.css
stays plain CSS and imports it), `DesignTokens.swift` and `Tokens.kt`. The accent maths
(`shade`, WCAG luminance, `readableTextOn`) moves into the core so every client derives
identical `accent-strong`, `accent-soft` and `on-accent` values from an artwork accent.

---

## 7. The shared core (Rust)

### 7.1 The message bridge (D29)

**Why hand-rolled Rust.** The pattern is Crux-shaped, but Crux itself shipped five breaking
releases in 2026, replaced its FFI layer in June, has open stream-lifecycle bugs that would hit
the couch WebSocket, documents no tvOS packaging and has no Svelte shell. Kotlin Multiplatform
is first-class on Android but its TypeScript export of suspend functions and Flows is still
experimental. Swift-everywhere ships a ~60 MB runtime on Android and multi-megabyte wasm. A
small bridge we own, built from mature pieces (Rust Tier 2 targets, UniFFI, wasm-bindgen,
typeshare), is the lowest-risk path; it is the shape Bitwarden and 1Password use.

**The whole FFI surface** (one Rust struct, exported twice):

```rust
pub struct CoreBridge { .. }

impl CoreBridge {
    pub fn new(config: String) -> Self;                 // CoreConfig JSON: platform, auth mode, capabilities
    pub fn send(&self, inbound: String) -> String;      // Inbound { now_ms, event }        -> Vec<EffectRequest>
    pub fn resolve(&self, inbound: String) -> String;   // Inbound { now_ms, id, output }  -> Vec<EffectRequest>
    pub fn view(&self, surface: String) -> String;      // Surface                         -> the surface's view model
}
```

- `#[uniffi::export]` produces the Swift and Kotlin classes; `#[wasm_bindgen]` produces the
  TypeScript class. Only strings cross; there are no async functions, callbacks or foreign
  traits across the boundary, so both binding layers stay trivial.
- Messages are JSON (serde). Every message type (`Event`, `EffectRequest`, effect outputs,
  `Surface`, view models) carries `#[typeshare]`, which generates Swift `Codable` +
  `Sendable` types, Kotlin `@Serializable` types and TypeScript types. Enums with data are
  adjacently tagged (`{"type": ..., "content": ...}`) so all three languages decode them the
  same way.
- **Time is an input.** Every inbound message carries the shell's monotonic `now_ms`, so the
  core never reads a clock and every test controls time exactly.
- **Effect lifecycles are explicit.** Each `EffectRequest` has an `id`. One-shot effects (HTTP,
  secure-store reads) are resolved once. Streaming effects (WebSocket, repeating timers,
  downloads) are resolved repeatedly and end with a terminal output (`Closed`, `Cancelled`,
  `Completed`); the core cancels them with an explicit effect (`Socket::Close { id }`,
  `Timer::Cancel { id }`). Nothing leaks and nothing arrives after its terminator.
- `Render { surfaces }` names the view models that changed, so a shell re-reads only those.
- The core is single-threaded by contract: each shell calls it from one serial context (the
  main actor on Apple, one dispatcher on Android, the main thread on the web). Typical
  `update` work is far below a millisecond; JSON (de)serialization of view models is the main
  cost and is measured in spike S2.
- Inside the core, modules are plain state machines (`fn update(&mut self, ev, ctx) ->
  Vec<Effect>`), with pending effects tracked by id in a small registry that routes outputs
  back to the module that asked. No async runtime, no executor, identical behaviour on wasm.

**Shell runtimes** (the only code each platform writes against the bridge): a `CoreRuntime`
that owns the `CoreBridge`, stamps `now_ms`, runs a loop over returned effects with one
executor per effect kind, re-reads changed surfaces, and publishes them to the UI
(`@Observable` on Apple, `StateFlow` on Android, `$state.raw` on the web).

### 7.2 API layer generated from the contract

`core/crates/api` is generated by `cargo xtask codegen` from `contract/openapi.json`: Rust
types for every schema (serde, `deny_unknown_fields` off for responses so additive server
changes never break older clients) and one typed builder per operation that produces an HTTP
request description and parses its response (`ops::get_title(slug) -> Request<TitleDetail>`).
The generator is our own small `xtask` module rather than progenitor (OpenAPI 3.0 only and
tied to reqwest) or typify (self-described work in progress); huma's schemas are regular Go
structs, so the subset to support is small and the output stays deterministic and readable.
API types never cross the bridge: shells only ever see core events, effects and view models.

### 7.3 Modules

Core modules mirror backend features and are the only place their client logic exists.

| Module | Responsibility |
|---|---|
| `servers` | Server registry, URL normalisation, `/server` identity check (id, name, version, API level, http/https badge), compatibility verdict. |
| `accounts` | Password sign-in, pairing (code shown on device, polling), connect-QR redemption, token storage via secure-store effects, Who's watching list, sign-out, device list + revoke. |
| `session` | Active account context: `/auth/me`, feature flags, preferences, display language (D21), 401 handling, theme accent. |
| `catalog` | Home, browse (movies/series/genre/sort/pagination), title detail, search (debounce + supersede), genres, My List, continue watching. Owns the SWR cache. |
| `playback` | Playback sessions: capability profile -> playback-info -> source selection, resume rules, progress accounting (watched-seconds deltas, 10 s cadence, final beacon), JIT keepalive and stop, preparing-poll, audio/subtitle defaults, next-episode and shuffle decisions, ranks check triggers. |
| `couch` | Protocol client (hello/state/participants/media/emoji/away/ended), reconnect with backoff, follower drift sync, host broadcast cadence, remote-control role, emoji throttling, recent emojis. |
| `ranks` | Stats, profile, leaderboard (client-side metric sort), throttled achievement checks, unlock celebration queue, level-up detection, heatmap buckets. |
| `profile` | Profile edits, avatar/banner upload orchestration, public-profile toggle, password change. |
| `downloads` | Download requests, server preparation polling, platform download effects, offline library, offline progress queue replayed on reconnect (iOS and Android only). |
| `markdown` | Bio markdown to a safe AST (no HTML, no images, safe link schemes, linkify), D19. |
| `theme` | Accent derivation (6.4). |

### 7.4 Model, events, effects, view models

- **Model**: one tree per core instance: servers, accounts, active session, per-module state,
  caches. Never exposed directly.
- **Events**: user intents and lifecycle (`ScreenOpened(Title{slug})`, `PlayRequested`,
  `PlayerReported(state)`, `CouchJoinRequested(code)`, `AppBecameActive`...).
- **Effects** (executed by shells):
  - `Http` (method, URL, headers, body; response status/headers/body), with the core attaching
    bearer tokens (native) or relying on cookies (web, `AuthMode::Cookie`).
  - `Socket` (open, send, close; opened/frame/closed resolve the open effect repeatedly).
  - `Timer` (after/every, cancel; ticks resolve the timer effect).
  - `SecureStore` (tokens) and `Store` (non-secret persistence: server list, warm-start
    snapshots, offline progress queue, recent emojis).
  - `Player` (load sources, play, pause, seek, rate, select audio/subtitle option).
  - `Download` (enqueue, cancel; progress and completion as events).
  - `Render` (view models changed).
- **View models**: plain data per surface (`HomeView`, `TitleView`, `PlayerView`, `CouchView`,
  `ProfileView`, `LeaderboardView`, `AccountsView`, `DownloadsView`...), each with an explicit
  load state (`Loading | Loaded | Stale(value) | NotFound | Failed(code)`), which maps directly
  onto the web's `CachedView` semantics (stale beats blank).
- **Navigation is owned by shells** (NavigationStack, Compose Navigation, SvelteKit routes).
  Shells tell the core which surfaces are visible; the core keeps them fresh.

### 7.5 Caching and warm start

The SWR cache moves into `catalog` (keys as today: title by slug, home by language, browse by
kind|genre|sort, profile by username|language, leaderboard by period), LRU-capped, cleared on
sign-out and on account switch. Shells persist a small warm-start snapshot (home and the last
title) through `Store`, so a cold launch paints content before the network answers.

### 7.6 Device capability profile

The shell measures capabilities at runtime and hands them to the core once per launch:
containers, video codecs with profiles/levels, HDR modes (HDR10, HLG, Dolby Vision profiles
5/8.1), max resolution and frame rate, audio codecs with channel counts (AAC, AC-3, E-AC-3,
E-AC-3 JOC), subtitle formats, HLS segment formats, and a bitrate ceiling. Apple measures via
`VTIsHardwareDecodeSupported`, `AVPlayer.availableHDRModes` and
`AVURLAsset.isPlayableExtendedMIMEType`; Android via `MediaCodecList` and `AudioCapabilities`;
the web via `MediaSource.isTypeSupported` (today's `clientCaps()`). The core sends the profile
with every playback-info request (8.5).

### 7.7 Bindings and packaging

- Apple: static libraries for `aarch64-apple-ios`, `aarch64-apple-ios-sim`,
  `aarch64-apple-tvos`, `aarch64-apple-tvos-sim` and `aarch64-apple-darwin` (host-side Swift
  tests), Swift bindings from `uniffi-bindgen-swift` (which never formats, so the swift-format
  hang does not apply), combined with `xcodebuild -create-xcframework` into
  `CouchverseCoreFFI.xcframework`. Xcode 27 merges every xcframework's headers into one include
  directory, so the headers and a plain `module` map (not `--xcframework`'s `framework module`)
  live in `Headers/CouchverseCoreFFI/`, and the Clang module name differs from the Swift module
  (`CouchverseCoreFFI` vs `CouchverseCore`). The ffi crate depends on `uniffi` with
  `default-features = false`; only the bindgen crate enables `cli`. Wrapped by the
  `CouchverseCore` Swift package together with the typeshare-generated Swift types. cargo-swift
  is not used (no tvOS support). Recipe and gotchas: `docs/spikes/s2-core-bridge.md`.
- Android: `cargo-ndk` for `arm64-v8a`, `armeabi-v7a`, `x86_64`; UniFFI Kotlin bindings (JNA)
  and typeshare-generated Kotlin types in the `core` Gradle module.
- Web: `wasm32-unknown-unknown` with wasm-bindgen (its library, pinned with the crate, run by
  `cargo xtask wasm`) and `wasm-opt -Oz`, emitted as a local package that `clients/web`
  imports, plus typeshare TypeScript types. Profile `release-wasm`: opt-level "z", LTO,
  `panic = "abort"`; no `regex`. (S2 chose opt-level 3, which halves `view` latency, while
  the core was small; with the catalog the serde code for API payloads put opt-level 3 at
  258 KB gzip against 204 KB for "z", so size won, 2026-10-03.)
- `cargo xtask` builds all three; `make core-apple|core-android|core-wasm` wrap it. A clean
  clone builds with `rustup` plus the documented targets; no prebuilt binaries are committed.
  The Docker image gains a Rust stage that builds the wasm package for the web build.
- Budgets enforced in CI: wasm under 400 KB gzip (raised from 250 KB on 2026-10-03 once the
  catalog showed every slice adds tens of KB of payload decoding; cached after the first
  visit), `send`/`resolve` under 1 ms for typical
  messages, home view model serialization under 2 ms on an Apple TV 4K (2nd gen). The budget
  covers the core's side; the shell's JSON decode is measured separately (S2: Swift
  `JSONDecoder` is the dominant cost, so shells decode off the main actor and `Render` names
  rows rather than whole screens).

### 7.8 Core testing

- Unit tests per module (pure functions: markdown policy, accent maths, drift maths, heatmap).
- Scenario tests driving events and asserting effects with explicit `now_ms`: sign-in and pairing,
  401 handling, SWR revalidation, playback session lifecycle (resume, beacons, JIT keepalive,
  stop, preparing poll, next episode), couch (join, follower correction, host away/return,
  reconnect, remote control), downloads with offline progress replay.
- Property tests for drift sync and progress accounting (no double counting across seeks,
  pauses and reconnects).
- Conformance: golden fixtures from `contract/fixtures` (couch frames, playback payloads).

---

## 8. Backend changes

Grouped by area; phases in section 14.

### 8.1 Remove music (Phase 0)

- Delete `internal/feature/music`, `flags.musicEnabled`/`RequireMusic`, the music settings
  toggle, the catalog `recently_played_music` home row and music search results, artwork kinds
  `album_cover`/`artist_photo`, `media/tags.go` and the `dhowden/tag` dependency, the `music`
  upload library kind and prober path, the music storage category, analytics `RecordListen`
  and `musicSeconds`, the ranks music XP rate, the four music achievements, the music
  leaderboard metric and music profile totals.
- Migration `0019_drop_music.sql`: drop `playlist_tracks`, `playlists`, `play_history`,
  `album_genres`, `tracks`, `albums`, `artists`; delete music `media_files` rows, music artwork
  rows and `watch_time_*` rows of kind `music`; remove the `ranks` config keys for music.
  Earned music achievements are deleted (D5).
- Files on disk are not deleted automatically; the release notes document the music library
  folder that can be removed by hand.
- Web: delete `features/music`, its routes, nav entry, admin music pages, player bar, the music
  pause hook in `VideoPlayer`, leaderboard/profile music UI and the music message keys.
- Docs: README, FEATURES.md (feature index, ranks, analytics, i18n), CLAUDE.md.

### 8.2 Server identity and API level

`GET /api/v1/server` (public): `{id, name, version, apiLevel, accent}`. `id` is a UUID persisted
in settings at first boot, `name` is admin-editable (default host name), `version` is stamped
at build time (`-ldflags -X`), `apiLevel` is a constant bumped whenever clients need new
behaviour. Clients refuse servers below their minimum and show a clear "update your server"
or "update the app" screen. The add-server flow checks this endpoint, so the SPA's HTML
fallback can never be mistaken for a server.

### 8.3 Authentication for devices

- **Device sessions**: the `sessions` table gains `kind` (`browser|device`), `device_name`,
  `platform`; device tokens are presented as `Authorization: Bearer <token>`. `Middleware.Load`
  accepts the cookie or the bearer. Expiry becomes **sliding** for both kinds (extended on use,
  at most once a day), fixing today's hard 30-day logout.
- `POST /auth/token` (password -> token + user) for native; `/auth/login` stays for the web.
- **Pairing** (RFC 8628 style): `POST /auth/pairings` (public) returns `{deviceCode, userCode,
  verifyUrl, expiresIn, interval}`; the device polls `POST /auth/pairings/poll`; a signed-in user
  approves at `/pair` on the web or from the app (`POST /me/pairings/{userCode}/approve`, which
  names the device). The TV renders the code and a QR of `verifyUrl`.
- **Connect a device** (web -> phone): the web profile shows a QR with
  `couchverse://connect?server=<url>&code=<one-time>`; the app adds the server and redeems the
  one-time code for a device token (`POST /auth/connect`). Short TTL, single use.
- **Devices**: `GET /me/devices`, `DELETE /me/devices/{id}`, shown on the web profile and in
  every app's settings. Disabling a user revokes all sessions (as today).
- **Hardening**: `RealIP` trusts forwarding headers only from configured `TRUSTED_PROXIES`, so
  the login rate limit cannot be bypassed by a spoofed `X-Forwarded-For`.
- The Origin-based CSRF check stays as is; bearer requests carry no Origin and are not
  CSRF-exposed.

### 8.4 Media grants

- A kernel package `internal/grant` signs and verifies grants: HMAC-SHA256 over
  `{scope, subject, mediaFileId | artworkId | sessionId, exp}` with a server secret persisted in
  settings, base64url-encoded.
- Media routes move under the grant prefix so relative HLS URIs inherit it:
  `/api/v1/media/{grant}/stream`, `/hls/master.m3u8`, `/hls/{variant}/{file}`,
  `/jit/{sid}/{file}`, `/frame`, `/subtitles/{id}.vtt`. Playback-info returns these URLs.
  Grants last a few hours and are refreshed by re-requesting playback-info.
- Artwork keeps `GET /artwork/{id}` with cookie or bearer and also accepts `?g=<grant>` with an
  account-scoped artwork grant, for system-fetched images (tvOS Top Shelf, AirPlay receivers).
- `requireAuthOrCouch` is replaced: the couch Hub issues grants for the host's current media to
  followers, so anonymous followers also get subtitles and artwork (today they cannot).
- The web moves to grant URLs too, so there is one media path.

### 8.5 Playback v2

**Probe enrichment** (library prober): codec tag (`hvc1`/`hev1`/`dvh1`), profile, level, bit
depth, frame rate, HDR format (HDR10, HDR10+, HLG, Dolby Vision profile and compatibility id),
audio channel layouts, per-stream languages. Stored on `media_files`/`audio_streams`.

**Decision** (`POST /playback/{kind}/{id}` with the capability profile from 7.6; the GET form
remains during the web migration):
1. **Direct**: the container, video and audio are all in the profile and no subtitle/audio
   switching needs HLS. Caps are authoritative: the server never says "direct" for something
   the profile cannot play (today VP9/WebM/Opus would be wrongly "direct" for Apple).
2. **Remux**: the video is decodable but the container or audio is not (MKV, DTS, TrueHD),
   or subtitles/alternate audio require HLS. Video is copied into fMP4 HLS; audio is copied if
   supported, otherwise transcoded (E-AC-3 5.1 plus an AAC stereo fallback). Prepared at probe
   time when cheap, or produced JIT.
3. **Transcode**: the prepared ladder, else JIT, as today.
4. `preparing` and `unsupported` as today.

**HLS v2** (validated in CI by our own validator, `couchverse validate-hls`, and played by an
AVFoundation harness on macOS; Apple's `mediastreamvalidator` runs locally when installed):
- fMP4 segments (`-hls_segment_type fmp4`), HEVC tagged `hvc1` (`dvh1` for Dolby Vision),
  IDR every 2 s, 6 s segments, `EXT-X-INDEPENDENT-SEGMENTS`.
- Multivariant playlists with `CODECS`, `RESOLUTION`, `FRAME-RATE`, `BANDWIDTH` (peak),
  `AVERAGE-BANDWIDTH`, `VIDEO-RANGE` (`SDR|PQ|HLG`), audio groups (`EXT-X-MEDIA TYPE=AUDIO`
  with language, channels, default/autoselect) and subtitle groups (`TYPE=SUBTITLES`, segmented
  WebVTT with `X-TIMESTAMP-MAP`, forced flag) built from the existing sidecar VTT files.
- I-frame playlists from a low-resolution trick-play rendition, so the tvOS transport bar
  shows scrubbing thumbnails.
- The source-copy variant is offered as an explicit "Original" quality, not mixed into the ABR
  ladder (its segment boundaries cannot be aligned with transcoded rungs).
- HDR: copied for HDR-capable profiles; tone-mapped to SDR H.264 for SDR-only profiles
  (fixes today's 10-bit-to-libx264 problem). Dolby Vision profile 7 falls back to its HDR10
  base layer (spike S5).
- Explicit content types for `.m4s`, `.mp4` and `.ts`; fixed multiaudio master URL bug.
- JIT: fMP4 output, codec copy where the profile allows, `DELETE /stream/sessions/{sid}` stop
  endpoint, sessions bound to the requesting user or couch grant.

The web benefits too: hls.js plays fMP4, HEVC goes to browsers that report it, and the quality
menu keeps working from the new master playlist attributes.

### 8.6 Downloads

- `POST /me/downloads {kind, id, quality, audio[]}` creates or reuses a **device-ready MP4**
  (faststart; video copied when the profile allows, else transcoded at the chosen quality;
  selected audio tracks; text subtitles muxed as `mov_text` so AVPlayer and ExoPlayer show them
  offline). Produced by a `prepare_download` job, reported through `GET /me/downloads`, served
  through a media grant, removed by the hourly cleanup after a retention period.
- Progress watched offline is replayed by the core through the normal progress endpoint.

### 8.7 Couch v2

- WebSocket auth: the participant token from create/join, as the couch cookie (web) or, for
  native clients (`delivery=body`), the `X-Couch-Token` header on the socket and the couch
  endpoints. (Planned as a bearer for signed-in native clients; one token for every native
  client turned out simpler and covers anonymous ones too, 2026-10-03.) A participant holds a
  token per device, so a host joining from a second device keeps the first signed in.
- Followers receive grant URLs in their playback payloads (8.4).
- **Remote control**: a host's extra connection (their phone) joins with role `remote` and
  sends `remote_command {action: play|pause|seek|next|previous, positionSeconds?}`; the Hub
  relays it to the host's playing connection, which applies it and broadcasts `host_state` as
  usual. The playing connection is the one that most recently sent `host_state`.
- **Host handover** (decided 2026-10-08): the host's account starting the session on another
  device while one plays for it hands the session over. The new device takes the host seat
  with its first `host_state` (sent as its socket opens) and plays its own title for everyone,
  since "Start a couch session" means "the couch watches what this device plays" on every
  device; the session's media follows it. The old device stops broadcasting from that moment
  and becomes the account's remote (D17): a fresh `hello` with `role: remote` tells it, its
  token comes back as a remote's, and its late `host_state` is dropped. Followers keep their
  seats and sockets. Tabs of one browser share the couch cookie, so a tab cannot take over from
  another tab playing for the session (409 `already_hosting`). The change is additive (a later
  `hello`, a dropped report), so the API level stays: an older client keeps playing on its own
  with its reports ignored, and a newer client against an older server sees no handover.
- QR payload for TV hosting: the public couch URL (`/couch/{code}`), which opens the web join
  page or, through an "Open in app" button, the app's `couchverse://couch/{code}` link.
- Protocol changes are versioned through the API level; golden frames updated in
  `contract/fixtures/couch`.

### 8.8 Catalog and metadata additions

- `logo` artwork kind: TMDB logo images per content language, fetched by metadata apply and
  import; exposed as `logoId`/`logoVer` on featured items and title detail (D20).
- Any viewer read the native screens need beyond today's (for example a lightweight
  "up next" for widgets and Top Shelf) is added as a typed operation in the same slice.

---

## 9. Web client changes

- Phase 0: move to `clients/web/`; delete music.
- Contract phase: Paraglide reads `contract/i18n`; plural variants; theme tokens include;
  admin API calls move to a typed client generated from `openapi.json` by `cargo xtask
  codegen` (the same model that emits the core's Rust types, so names match across clients;
  `openapi-typescript` was dropped because it requires TypeScript 5 and the web is on 6).
- Auth phase: `/pair` approval page, "Connect a device" QR, Devices list on the profile.
- Core adoption, slice by slice (D28): a `lib/core/` runtime loads the wasm core during the root
  layout load, executes effects (fetch with cookies, WebSocket, timers, localStorage, the
  hls.js player adapter) and exposes the current view models as `$state.raw`. Pages keep the
  optimistic `CachedView` pattern but read core view models instead of the SWR cache;
  `+page.ts` loaders send `ScreenOpened` events instead of fetching. Admin pages are untouched.
- The TV mode (Titan OS) keeps working unchanged; it only changes where data comes from.
- Markdown: `ui/Markdown.svelte` renders the core AST with Svelte components; `{@html}` and
  `markdown-it` are removed, and CLAUDE.md's security note is updated to point at the core.

---

## 10. Apple clients

### 10.1 Project and tooling

- One committed `Couchverse.xcodeproj` using **folder-synchronized groups**, so adding files
  never touches the project file. Logic lives in local Swift packages so agents can
  `swift build`/`swift test` without the project. Convert to Xcode's JSON project format once
  it is out of beta.
- `Config/Base.xcconfig` (shared settings) plus a gitignored `Config/Local.xcconfig` (template
  committed) for `DEVELOPMENT_TEAM` and `BUNDLE_ID_ROOT` (default `io.stepes.couchverse`) and
  the `EXTENSIONS_ENABLED` switch for capability-dependent extras.
- Swift 6 language mode, strict concurrency, `SWIFT_DEFAULT_ACTOR_ISOLATION=MainActor` for app
  and feature targets. `swift-format` (bundled with the toolchain) enforced in CI.
- Agents can drive Xcode through its MCP bridge (`xcrun mcpbridge`) to build, run simulators
  and take screenshots.

### 10.2 Targets and packages

| Target | Contents |
|---|---|
| `Couchverse` (iOS, iPadOS) | App entry, scenes, platform wiring. |
| `Couchverse TV` (tvOS) | App entry, scenes, platform wiring. |
| `CouchverseWidgets` (iOS ext) | Continue Watching widget, couch Live Activity. Optional (10.9). |
| `CouchverseTopShelf` (tvOS ext) | Top Shelf content provider. Optional (10.9). |

| Package | Contents |
|---|---|
| `CouchverseCore` | The xcframework, generated types, `CoreRuntime` (`@Observable`, owns the core, publishes view models) and the effect executors: `HTTPExecutor` (URLSession), `SocketExecutor` (URLSessionWebSocketTask), `TimerExecutor`, `KeychainStore`, `FileStore`, `DownloadExecutor` (background URLSession, iOS), `CapabilityProbe`. |
| `CouchverseDesign` | Tokens, typography, accent environment, components: `PosterCard`, `BackdropCard`, `HeroCarousel`, `ShelfRow`, `RankRing`, `AvatarView`, `GlowBackdrop`, `Skeleton`, `MarkdownView` (renders the core AST), `EmojiBurst`. Idiom-aware (focus on TV, touch on phone). |
| `CouchverseFeatures` | Screens grouped by feature folder (`Accounts`, `Catalog`, `Playback`, `Couch`, `Ranks`, `Profile`, `Downloads`, `Settings`). Shared views where the layout is the same; `+TV`/`+Phone` files where idioms diverge. |

The apps are thin: they compose the runtime, the root navigation and the platform hooks.

### 10.3 App architecture

- `CoreRuntime` is the only stateful service; screens receive it through the SwiftUI
  environment, read view models and send events. No other singletons, no view-model classes
  duplicating core state.
- Navigation: tvOS uses a `TabView` with the adaptable **sidebar** (Home, Movies, Series,
  My List, Search, Couch, Profile; Leaderboard when ranks are on; Settings). iPhone uses a
  Liquid Glass tab bar (Home, Browse, My List, Profile, with Search as `Tab(role: .search)`);
  iPad uses the same tabs as `.sidebarAdaptable`. Detail navigation via `NavigationStack` with
  zoom transitions from cards on iOS.
- Deep links: `couchverse://title/{slug}`, `play/{kind}/{id}`, `couch/{code}`,
  `connect?...`, `pair?...` (custom scheme, because Associated Domains need a paid team).
  (`play/` rather than the planned `watch/`, the shape Android shipped first and every client
  now uses, 2026-10-08.)

### 10.4 Screens per idiom

| Surface | iPhone | iPad | Apple TV |
|---|---|---|---|
| Who's watching | account switcher sheet | same | full-screen picker on launch (12.3) |
| Add server / sign in | URL entry, QR scanner, password, approve-pairing | same | URL entry, password, pairing code + QR |
| Home | hero carousel, continue watching, admin rows | wider shelves | cinematic hero with title logo, shelves with focus parallax |
| Movies / Series / Genres | grids with sort/genre filters | multi-column | focus grids |
| Title | backdrop header, logo, play/resume, My List, seasons + episodes | split layout | full-bleed backdrop, episode shelf per season |
| Search | system search tab | same | `.searchable` with dictation |
| Player | system player + overlays (10.5) | same | system player + transport bar items (10.5) |
| Couch | start/share sheet, join by code or link | same | host with QR + code panel, join by code |
| Profile / Leaderboard | full profile, edit, devices | same | full profile, edit, devices |
| Downloads | list, storage, offline mode | same | not available |
| Settings | servers, accounts, language, devices, about | same | same |

### 10.5 Player

- `AVPlayerViewController` wrapped for SwiftUI (the SwiftUI `VideoPlayer` lacks the needed
  hooks). The core picks sources; a `PlayerController` executes player effects and reports
  coarse events.
- tvOS: `transportBarCustomMenuItems` (Couch: start, share code, end; Episodes),
  `contextualActions` ("Next episode" countdown in the last 20 s), `customInfoViewControllers`
  (Episodes, Couch participants), and an overlay controller for floating emoji, couch avatars
  and achievement celebrations. Audio and subtitle menus are the native ones, fed by HLS
  renditions. Display criteria matching (frame rate, dynamic range) on.
- iOS/iPadOS: the system player with PiP (`canStartPictureInPictureAutomaticallyFromInline`),
  AirPlay (works because URLs carry grants), Now Playing through the iOS 27 Now Playing
  framework, plus a SwiftUI overlay layer for couch, next episode and celebrations (spike S4).
- Followers: `requiresLinearPlayback = true` removes timeline control, matching the web.
- Resume, beacons, keepalive, next episode, shuffle and ranks checks are all core behaviour.

### 10.6 Couch on Apple

- Host from any device, including the TV (a panel with a large QR and the 6-digit code).
- Join by code (TV remote friendly digit pad), by scanning the QR, or from a `couchverse://`
  link; anonymous join works without an account.
- Followers sync through the core; corrections use `seek(to:toleranceBefore:toleranceAfter:)`.
- Emoji: a curated reaction set plus the user's recent emojis (shared with the web through the
  core's `Store`), as a compact bar on phones (with a "more" button that opens the system emoji
  keyboard) and a transport bar menu on TV, with floating reactions over the video.
- Phone as remote: when the host's account also has the couch open on a phone, the phone shows
  remote controls (play/pause, scrub, next) instead of a second player.
- Live Activity on iOS (10.9).
- Image editing on TV (D18): avatar and banner changes on tvOS show a QR that opens the edit
  sheet on the phone app (or the web profile); text fields are edited on the TV directly.

### 10.7 Accounts and onboarding

- First launch: welcome -> add server (URL field with validation against `/server`, or scan a
  "Connect a device" QR on iPhone) -> sign in (password, or "Sign in with another device",
  which shows a pairing code and QR) -> profile picker or home.
- Server list and accounts per server in Settings; tokens in the Keychain
  (`kSecAttrAccessibleAfterFirstUnlock`); a lost or revoked token drops the account to
  "Signed out" without losing the server.
- Unencrypted (`http://`) servers show a small "not encrypted" badge in Settings and on the
  sign-in screen.

### 10.8 Downloads (iOS/iPadOS)

- Download button on titles and episodes with a quality choice; the core requests preparation,
  polls readiness, then a background `URLSession` downloads the MP4 into Application Support
  (excluded from backup).
- Downloads screen with progress, storage used and delete; offline mode when the server is
  unreachable shows only downloaded content.
- Offline playback uses local files; progress is queued and replayed when back online.

### 10.9 System integration and free-team constraints

| Feature | Needs | Plan |
|---|---|---|
| Now Playing, PiP, AirPlay, background audio | Info.plist background mode only | Always on. |
| App Intents, App Shortcuts, Spotlight (continue watching, My List, join couch) | none | Always on. |
| Continue Watching widget | data sharing with the app (App Group or Keychain Sharing) | On when spike S1 confirms a free team can provision it; else disabled via `EXTENSIONS_ENABLED`. |
| Couch Live Activity | ActivityKit, local updates only (push needs a paid team) | Updated while the app runs (playback keeps it alive); a stale date covers suspension. |
| Top Shelf (carousel of continue watching + featured) | extension + shared credential, artwork grants | Same gating as widgets. |
| Universal links | Associated Domains (paid) | Not used; custom scheme instead. |
| SharePlay (later) | Group Activities | Later phase, optional mode alongside the WebSocket couch. |

Free personal team realities, documented in the build guide: 7-day provisioning (reinstall
weekly), at most 3 apps per device, App ID limits per week, TV installs over Xcode's wireless
pairing.

### 10.10 Accessibility and localization

VoiceOver labels on every control, Dynamic Type on iOS and tvOS 27, Reduce Motion alternatives
for every animation (12.4), Reduce Transparency respected, focus order audited on TV, minimum
contrast through the core's `on-accent` derivation. Strings from the generated
`Localizable.xcstrings` (en, cs) with plural variations; dates and durations formatted with
`FormatStyle` in the display language.

---

## 11. Android and Google TV clients

- One Gradle project, one application (`io.stepes.couchverse`) with a phone UI and a TV UI over
  shared modules (D23); `LEANBACK_LAUNCHER` and `android.software.leanback` (not required) make
  it appear on Google TV; the TV UI is chosen at launch from `UiModeManager`.
- Modules: `core` (UniFFI Kotlin, `.so` per ABI, `CoreRuntime` exposing view models as
  `StateFlow`, effect executors: OkHttp, OkHttp WebSocket, coroutine timers, Keystore-backed
  encrypted storage, DataStore, Media3 player, WorkManager/Media3 downloads), `design` (tokens,
  Material 3 phone components, Compose for TV components, shared motion), `feature-*` (accounts,
  catalog, playback, couch, ranks, profile, downloads, settings) with `phone/` and `tv/` UI
  packages, `app`.
- Kotlin 2.4, Compose (Material 3) for phones, Compose for TV (`tv-material` 1.1 and
  `tv-foundation` 1.0, stable since May 2026, with the standard lazy layouts; Leanback is
  deprecated) for Google TV, Media3 1.11 ExoPlayer + MediaSession (background, PiP,
  notification controls) with `media3-ui-compose`, Coil for images, Glance for the widget, an
  ongoing notification for the couch session (Live Updates where available).
- Continue watching on the Google TV home: the "Watch Next" row through `androidx.tvprovider`.
  Google's Engage SDK requires allowlisting, which an open-source sideloaded app cannot rely on.
- Same deep links, same onboarding, same Who's watching experience on Google TV.
- Signed APKs built by CI on tags and attached to GitHub Releases; builders can sign their own.

---

## 12. Design language: "Native + Couchverse soul"

### 12.1 Principles

- System navigation, controls, typography (SF Pro / Roboto Flex) and Liquid Glass chrome; the
  Couchverse personality lives in content: artwork, accents, cards, the rank ring, the couch.
- Dark-first canvas from the web (`#07080d`, surfaces `#11131c`/`#191c27`), artwork-driven
  accent tinting per title and per profile banner, default accent `#e50914`.
- Big imagery, few words. Title logos over backdrops; episode stills; avatars everywhere.
- Every screen has a designed skeleton mirroring its layout; stale content beats a spinner.

### 12.2 Components shared by every native client

Poster card, backdrop card (continue watching with progress bar), hero carousel (8 s per slide,
pauses while focused or touched), shelf row, episode row with still, rank ring with level chip,
avatar (image or identicon seed), glow backdrop (blurred accent orbs; no blur on TV GPUs that
struggle), achievement toast and in-player celebration, couch "sofa" of seated avatars, emoji
burst, markdown view.

### 12.3 "Who's watching?" (TV showpiece, also used on phones as a sheet)

- Canvas: the glow backdrop slowly drifting, tinted by the focused profile's banner accent;
  the tint cross-fades as focus moves.
- Profiles: large circular glass tiles with the avatar, the rank ring in the tier colour and the
  name below. Focus lifts the tile (scale ~1.12 with the system parallax), brightens the ring
  and reveals the rank title. A "+" tile adds an account.
- Selection: the chosen avatar flies (matched geometry) into its place in the sidebar header
  while the others dissolve outward and the home hero rises from black behind it, so the
  transition reads as "the room fills with your stuff". Phones play a lighter version with a
  haptic tick.
- Reduce Motion: a simple cross-fade.

### 12.4 Motion

Spring-based, short, interruptible. Zoom transitions from cards into titles (iOS), focus
parallax and lift (TV), shared-element transitions on Android, no gratuitous loops. Every
animation has a Reduce Motion fallback. Unlock celebrations and level-ups reuse one choreography
across platforms.

---

## 13. Quality: tests and CI

| Layer | Tests | Tooling |
|---|---|---|
| Backend | unit + integration (Postgres service), contract tests (every huma operation validated against the spec), HLS output of every tier on generated media validated by `couchverse validate-hls` and played in AVFoundation (macOS job) | `go test`, golangci-lint, `make hls-check` |
| Contract | drift check (regenerate, diff must be empty), i18n completeness (en and cs have identical keys and variants) | `make contract` |
| Core | unit, scenario, property, conformance fixtures; size and latency budgets | `cargo test`, clippy (deny warnings), rustfmt, proptest |
| Web | existing svelte-check, prettier, eslint; core-adapter tests; Playwright smoke (sign in, browse, play, couch join) | vitest, Playwright |
| Apple | runtime and executor tests (Swift Testing); snapshot tests of key screens fed with fixture view models (iPhone, iPad, TV; en and cs; largest Dynamic Type); XCUITest smoke on simulators | `xcodebuild test`, swift-snapshot-testing |
| Android | unit tests, Compose UI tests, screenshot tests (phone and TV) | Gradle, Roborazzi |

Because screens render view models, snapshot tests need no server: fixture view models drive
every state (loading, stale, loaded, not found, failed).

GitHub Actions: `backend.yml`, `contract.yml`, `core.yml`, `web.yml`, `apple.yml` (macOS runner
with Xcode 27: iOS + tvOS simulators), `android.yml`, `release.yml` (multi-arch Docker image to
GHCR, signed APKs). Building multi-arch images in CI also spares Raspberry Pi users the slower
native build now that the image includes a Rust/wasm stage.

`make` gains `contract`, `core-test`, `core-apple`, `core-android`, `core-wasm`, `apple-test`,
`android-test`; `make check lint test build` keep their meaning for the web and backend.

---

## 14. Roadmap

Each phase ends with green CI, updated docs (FEATURES.md, CLAUDE.md where conventions change)
and its exit criteria met. Phases 3 to 9 are vertical slices (D28); within a slice iOS and tvOS
ship together, with the TV layout designed first.

### Phase 0: Cleanup and restructure
- Remove music (8.1) across backend, web, DB and docs.
- Move `frontend/` to `clients/web/`; update Makefile, Dockerfile, embed path, docs.
- Bootstrap CI for backend and web.
- **Exit**: no music references remain (grep-clean), migration applied cleanly on a copy of a
  real database, web and backend CI green.

### Phase 1: Contract
- huma on the router; all viewer operations migrated, admin operations migrated; error envelope
  preserved; `couchverse openapi`; `contract/openapi.json`; couch protocol schema; fixtures.
- i18n moved to `contract/i18n` with plural variants; xcstrings and Android generators;
  design tokens and generators; web admin on the generated typed client.
- **Exit**: spec covers every route; drift check in CI; web unchanged in behaviour.

### Phase 2: Backend platform for devices
- `/server` identity and API level; device sessions, bearer, sliding expiry, devices list,
  `TRUSTED_PROXIES`; pairing and connect-QR; media grants (web switched over; anonymous couch
  followers get subtitles and artwork); JIT stop endpoint; explicit content types.
- Web: `/pair` approval, Connect-a-device QR, Devices list.
- **Exit**: a scripted client can add a server, pair, fetch home and play a grant URL in
  `ffprobe` and AVPlayer (macOS test harness).

### Phase 3: Core foundation + first Apple slice
- Spikes S1 (free-team capabilities) and S2 (core bridge end to end) first.
- Core workspace, codegen from the contract, runtime, effects, `servers`, `accounts`,
  `session`, `theme`, `markdown`; bindings for Swift, Kotlin (built and smoke-tested in CI,
  no UI yet) and wasm.
- Web adopts `session` and `markdown` (drops `{@html}`).
- Apple: project, packages, `CoreRuntime` and executors, design system v0, onboarding (URL and
  QR), password sign-in, pairing, Who's watching (TV) and account switcher (phone), Settings
  (servers, accounts, language, devices).
- **Exit**: on a real Apple TV and iPhone, add a server, pair the TV from the phone, switch
  profiles with the full animation.

### Phase 4: Playback v2 (backend + web)
- Spikes S3 (Apple-grade HLS from ffmpeg, including on the Pi) and S5 (Dolby Vision) first.
- Probe enrichment, capability-profile decision, remux tier, fMP4 HLS, attributes, audio and
  subtitle renditions, I-frame playlists, HDR handling, "Original" quality, JIT fMP4.
- Web sends its capability profile (through the core once Phase 6 lands, directly until then).
- **Exit**: the HLS validator clean for every tier on sample media and AVFoundation plays
  them (`mediastreamvalidator` where installed); the web plays every sample; a remuxed MKV with E-AC-3 and subtitles plays on AVPlayer with native menus.

### Phase 5: Slice - browse and titles
- Core `catalog` (SWR, warm start, search). Title logos (8.8).
- Web viewer pages (home, browse, genres, My List, search, title) read core view models.
- Apple: Home, Movies, Series, Genres, My List, Search, Title on iPhone, iPad and TV.
- **Exit**: web behaviour unchanged (Playwright smoke), Apple screens complete with skeletons,
  snapshot tests for every state.

### Phase 6: Slice - playback
- Core `playback`; web `VideoPlayer` delegates orchestration to the core.
- Apple player (10.5): resume, beacons, next episode, shuffle, audio/subtitles, PiP, AirPlay,
  Now Playing, celebrations overlay, display criteria.
- **Exit**: watched time and completion on Apple match the web for the same session; HDR and
  Atmos samples play natively on Apple TV 4K.

### Phase 7: Slice - couch
- Couch v2 backend (auth modes, grants, remote control), core `couch`, web adoption.
- Apple: host and join on every idiom, TV QR + code panel, emoji, phone as remote, follower
  linear playback.
- **Exit**: a session hosted on Apple TV with an iPhone remote, an anonymous web follower and a
  web host handover stays within the drift threshold for an hour.

### Phase 8: Slice - ranks and profiles
- Core `ranks` and `profile`; web adoption.
- Apple: profile, leaderboard, achievements grid, heatmap, watch clock, unlock celebrations,
  editing (photos on iOS, phone handoff on TV), devices.
- **Exit**: unlocks celebrate once across devices; editing round-trips on every platform.

### Phase 9: Apple system integration
- Top Shelf, Continue Watching widget, couch Live Activity (per S1 outcome), App Intents,
  App Shortcuts, Spotlight.
- **Exit**: each extra works on a free-team build or is cleanly disabled with a documented reason.

### Phase 10: Downloads
- Backend download preparation (8.6); core `downloads`; iOS/iPadOS downloads UI and offline mode.
- **Exit**: download on Wi-Fi, watch in airplane mode, progress syncs on reconnect.

### Phase 11: Android foundation
- Gradle project, `core` bindings module, design system (phone + TV), onboarding, accounts,
  Who's watching on Google TV, browse and titles.
- **Exit**: parity with Phase 5 on a phone and a Google TV device.

### Phase 12: Android playback, couch, ranks, downloads
- Media3 player, couch, ranks and profiles, downloads, widget, couch notification, Watch Next.
- **Exit**: parity with the Apple slices; signed APK published by CI.

### Phase 13: Polish and release
- Accessibility and localization audit, performance on a Raspberry Pi server, build-from-source
  guides (Xcode free team, Android signing), release automation, docs.
- Optional follow-ups: SharePlay mode, JSON Xcode project conversion.

---

## 15. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Scope is very large for one maintainer with agents | Vertical slices with shippable exits; Android strictly after Apple; music removal shrinks the surface first. |
| Shared-core tooling complexity (FFI, wasm, codegen) | Spike S2 before committing; one sans-I/O bridge for all languages; `cargo xtask` hides packaging; CI builds every binding from day one. |
| The core leaks one platform's assumptions | Two consumers per slice (web + Apple); core reviews check for platform words in core APIs. |
| AVPlayer strictness with ffmpeg HLS | Spike S3; HLS validator and AVFoundation harness in CI; fMP4 everywhere; real-device checks on Apple TV. |
| Raspberry Pi cannot transcode HEVC/HDR | Remux and copy paths first; HEVC output only where hardware encoders exist; SDR tone-mapping only when the profile needs it. |
| Free personal team limits (7-day expiry, capability gaps) | Spike S1; `EXTENSIONS_ENABLED` switch; graceful degradation; documented reinstall flow. |
| Web regressions while adopting the core | Per-slice adoption behind Playwright smoke and existing checks; admin untouched. |
| iOS overlays fighting the system player controls | Spike S4; fall back to fewer in-player overlays on iPhone if needed. |
| Liquid Glass changes in later SDKs | Use system components; keep custom glass to a few design-system components. |
| tvOS storage purge | Keychain for credentials, everything else treated as cache with warm-start snapshots. |
| 32-bit Android TV devices | Build `armeabi-v7a` in CI and test on such a device. |

---

## 16. Spikes and open questions

**Spikes** (timeboxed, at the start of the phase that needs them):
- **S1** Free personal team: can it provision App Groups and Keychain Sharing for an iOS app +
  widget and a tvOS app + Top Shelf extension? How do the 3-app and App ID limits count
  extensions?
- **S2** Core bridge end to end: one event -> HTTP effect -> view model on iOS simulator, tvOS
  simulator, a tvOS device, an Android emulator and the Svelte app; a Socket stream effect with
  cancellation; the xcframework with five slices under Xcode 27; typeshare output compiling in
  Swift 6 strict mode, Kotlin and TypeScript; measured wasm size and serialization cost of a
  realistic home view model.
- **S3** ffmpeg fMP4 HLS with HEVC copy (`hvc1`), E-AC-3 passthrough, WebVTT renditions and an
  I-frame playlist, validated and played on Apple TV 4K; repeat on a Pi 4 with the V4L2
  encoder. Done on the Mac and in Docker (`docs/spikes/s3-apple-hls.md`); the Apple TV and Pi
  runs are on its checklist.
- **S4** iOS `AVPlayerViewController` with SwiftUI overlays: visibility coordination, PiP,
  follower `requiresLinearPlayback`.
- **S5** Dolby Vision profile 7 and 8.1 handling with the ffmpeg in the Docker image. Done with
  synthetic samples (`docs/spikes/s5-dolby-vision.md`): Dolby Vision signalling needs ffmpeg 6+,
  so the Docker image runs on Debian trixie (ffmpeg 7.1).
- **S6** Top Shelf images through artwork grants.

**Open questions** (decided during implementation, recorded here when settled)
- Download retention period and the quality choices offered (proposal: Original when the
  device can play it, 1080p, 720p; prepared files kept 14 days after the last download).
- Whether Top Shelf shows a continue-watching row, a featured carousel, or both (decided after
  S1/S6 with real artwork on a TV).
- Re-evaluate Crux once it reaches 1.0 with stable stream semantics; the message bridge keeps
  a migration possible because module state machines stay framework-free.

---

## 17. Documentation and convention updates

- CLAUDE.md: new layout (`clients/`, `core/`, `contract/`), core rules (sans-I/O, no UI words,
  feature names aligned across layers, generated code never edited), contract workflow
  (`make contract`), Swift/Kotlin/Rust style rules, the markdown security boundary moving to the
  core, the device-auth and media-grant rules, removal of music conventions.
- FEATURES.md: music removed; new sections for device auth and pairing, media grants, playback
  v2, downloads, the core and each client; updated dependency graph.
- README: native apps, build-from-source guides, capability notes for free teams.

---

## Appendix A: research notes

- iOS 27 device support: https://www.macrumors.com/2026/09/14/ios-27-compatible-iphones/
- tvOS 27 release notes and supported models: https://www.macrumors.com/2026/09/25/tvos-27-release-notes/
- tvOS Dynamic Type (WWDC26-221): https://developer.apple.com/videos/play/wwdc2026/221/
- Generated subtitles (WWDC26-256): https://developer.apple.com/videos/play/wwdc2026/256/
- What's new in HLS 2026: https://developer.apple.com/streaming/Whats-new-HLS.pdf
- Now Playing framework (WWDC26-312): https://developer.apple.com/videos/play/wwdc2026/312/
- SwiftUI updates: https://developer.apple.com/documentation/updates/swiftui
- Scene life cycle requirement: https://developer.apple.com/documentation/technotes/tn3187-migrating-to-the-uikit-scene-based-life-cycle
- HLS Authoring Specification: https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices
- Rust tvOS Tier 2: https://github.com/rust-lang/rust/pull/152021
- Kotlin/Native targets: https://kotlinlang.org/docs/native-target-support.html
- swift-openapi-generator: https://github.com/apple/swift-openapi-generator
- huma OpenAPI generation: https://huma.rocks/features/openapi-generation/
- Swiftfin player trade-offs: https://github.com/jellyfin/Swiftfin/blob/main/Documentation/players.md
- Swiftfin AVPlayer device profile: https://github.com/jellyfin/Swiftfin/blob/main/Shared/Objects/VideoPlayerType/VideoPlayerType%2BNative.swift
- Apple TV 4K specs: https://support.apple.com/en-us/111839
- AVURLAsset headers not supported: https://developer.apple.com/forums/thread/20421
- Local network privacy (not on tvOS): https://developer.apple.com/documentation/technotes/tn3179-understanding-local-network-privacy
- ATS local networking: https://developer.apple.com/documentation/bundleresources/information-property-list/nsapptransportsecurity/nsallowslocalnetworking
- Membership comparison: https://developer.apple.com/support/compare-memberships/
- Synchronized folders and project churn: https://tuist.dev/blog/2025/03/21/git-conflicts
- Xcode 27 release notes: https://developer.apple.com/documentation/xcode-release-notes/xcode-27-release-notes
- Xcode 27 linker and Clang module changes: https://blakecrosley.com/blog/xcode-27-build-failures-ld64-clang-modules
- Crux releases and changelog: https://github.com/redbadger/crux/blob/master/crux_core/CHANGELOG.md
- Crux stream lifecycle issues: https://github.com/redbadger/crux/issues/621, https://github.com/redbadger/crux/issues/622
- BoltFFI packaging (no tvOS): https://www.boltffi.dev/docs/packaging/
- UniFFI changelog: https://github.com/mozilla/uniffi-rs/blob/main/CHANGELOG.md
- uniffi-bindgen-swift xcframework flow: https://mozilla.github.io/uniffi-rs/next/swift/uniffi-bindgen-swift.html
- uniffi-bindgen swift-format hang under Xcode 27: https://github.com/asmuelle/agent-ssh/pull/60
- Bitwarden sdk-internal (Rust core for web, iOS, Android): https://github.com/bitwarden/sdk-internal/pull/1503
- typeshare: https://github.com/1Password/typeshare
- cargo-ndk: https://github.com/bbqsrc/cargo-ndk
- Rust tvOS platform support: https://doc.rust-lang.org/nightly/rustc/platform-support/apple-tvos.html
- Kotlin/JS suspend export (experimental): https://kotlinlang.org/docs/whatsnew23.html
- Swift on Android (Skip, runtime size): https://skip.dev/blog/swift-63-android-support/
- progenitor (OpenAPI 3.0 only): https://github.com/oxidecomputer/progenitor
- typify: https://github.com/oxidecomputer/typify
- Compose for TV releases: https://developer.android.com/jetpack/androidx/releases/tv
- Media3 releases: https://developer.android.com/jetpack/androidx/releases/media3
- Engage SDK allowlisting: https://developer.android.com/guide/playcore/engage/faq
- Watch Next (TvProvider): https://developer.android.com/training/tv/discovery/watch-next-add-programs
- tvOS storage limits: https://developer.apple.com/library/archive/documentation/General/Conceptual/AppleTV_PG/index.html

## Appendix B: current-state findings that drive backend work

- Auth is cookie-only; no bearer, no pairing; fixed 30-day expiry; `RealIP` trusts spoofable
  headers for the login rate limit.
- Media routes are cookie-only; anonymous couch followers cannot load subtitles or artwork.
- Playback caps can only add codecs, so VP9/WebM/Opus files are reported as direct for clients
  that cannot play them; no audio caps; no MOV; HDR range probed but unused.
- HLS: MPEG-TS only, H.264 only, stereo AAC only, no `CODECS`/`AVERAGE-BANDWIDTH`/`FRAME-RATE`,
  no I-frame playlists, subtitles only as sidecar VTT, source-copy and ladder segments not
  aligned, no explicit `.ts` content type, a broken multiaudio master URL, 10-bit HDR sent
  through libx264.
- JIT: no stop endpoint; 3 global sessions; segment requests block up to 25 s.
- No server identity endpoint; the SPA fallback returns HTML with 200 for unknown paths.
- Viewer client logic worth sharing: ~50 HTTP calls plus one WebSocket protocol, progress
  accounting, couch drift sync (3 s threshold, 2 s host heartbeat, 500 ms to 8 s backoff),
  ranks check throttle, accent maths, title-detail derivations (next-up episode, progress
  percentages).
