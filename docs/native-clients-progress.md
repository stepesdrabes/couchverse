# Native clients: progress

Tracks `docs/native-clients-plan.md` phase by phase on the `native-clients` branch. Updated as work
lands; the plan stays the design, this file says where things stand.

Legend: done, in progress, pending, blocked (needs the user).

## Phase 0: cleanup and restructure - done

Music removed, `frontend/` moved to `clients/web/`, CI for backend, web and the repo.

## Phase 1: contract - done

huma on the router, every route an operation, `contract/openapi.json`, the couch protocol schema
and golden frames, i18n in `contract/i18n` with plurals, design tokens, the generators
(`cargo xtask codegen`: Rust API crate, web typed client, Apple/Android strings and tokens), the web
admin on the typed client, typed enums on response fields, upload bodies, path builders.

## Phase 2: backend platform for devices - done

Server identity, device sessions with sliding expiry, pairing, connect codes, devices list, media
and artwork grants, JIT stop. Web: `/pair`, Connect-a-device QR, Devices list. Exit verified with a
scripted client (ffprobe and AVFoundation playing a grant URL).

## Phase 3: core foundation and first Apple slice - done

- The core (`servers`, `accounts`, `session`, `theme`, `markdown`, plus the later modules
  below), bindings and packaging (`make core-apple|core-android|core-wasm`, `core.yml`,
  `android.yml`, `apple.yml`), the wasm budget (400 KB gzip, size build), the web on the core's
  session and markdown (no `{@html}`), the Android `core` and `design` Gradle modules with JVM
  tests.
- Apple: the Xcode project (iPhone/iPad and Apple TV apps), `CoreRuntime` with every executor,
  design system v0 (Liquid Glass actions, identicon avatars, glow backdrop, skeletons,
  markdown, QR), onboarding, password sign-in and pairing (the phone approves the TV's code,
  also by scanning its QR), Who's watching with the profile-switch choreography, Settings,
  129 snapshot references (en/cs, largest Dynamic Type), live UI flows on the simulators.
- The Playwright web smoke suite (`make e2e`, `e2e.yml`; 19 flows in Chromium and WebKit
  covering auth, browse, My List, playback and resume, couch, admin, i18n, ranks and TV mode),
  with the six app bugs it found fixed.
- Blocked: spike S1 (free personal team capabilities) needs the user's Apple ID and devices.

## Phase 4: playback v2 - done

Probe enrichment, the device-profile decision (`resolvePlayback`), the remux tier, fMP4 HLS with
audio and subtitle renditions and I-frame playlists, the own HLS validator (`make hls-check`,
`hls.yml` green on GitHub), the AVFoundation harness, the web on the new decision, the Docker
runtime on Debian trixie (ffmpeg 7.1 for Dolby Vision signalling). Spikes S3 and S5 documented.

Known limits, kept for later: JIT never copies video (needs a keyframe index), Dolby Vision
profile 5 tone-mapped for non-DV clients has wrong colours (needs libplacebo), no HEVC/HDR
ladder, model-B audio siblings stay outside the multivariant playlists, legacy TS variants are
not re-transcoded automatically, Homebrew's ffmpeg cannot tone-map (no zscale),
`mediastreamvalidator` never run (needs an Apple ID download; `make hls-apple`).

## Phase 5: browse and titles - in progress

- Done: core `catalog` (stale-while-revalidate, warm start, search, My List, image URLs), title
  logos (backend, TMDB, admin), logos in the core's views.
- Done: the web's catalog pages on the core (home, listings, genres, title, My List, search;
  notices as toasts; the catalog SWR cache and fetchers deleted), with the My List race, the
  shared warm home on sign-out and episode lengths fixed in the core; navigation at parity
  (revisits 40-100 ms, cold visits one round trip).
- In progress: Apple browse and titles together with the Apple player (Phases 5 and 6).

## Phase 6: playback - in progress

- Done: core `playback` (player effect, resume, watched-time accounting, progress saves, JIT
  keepalive and stop, preparing poll, qualities, audio and subtitle choices, next episode,
  shuffle, reload on failure).
- Done: the core's device profile (`CapabilitiesReported(DeviceProfile)`, the contract's
  fixtures round-trip unchanged), `resolvePlayback`/`resolveCouchPlayback` by POST, sources by
  tier (Original = the source file or the remuxed HLS, Auto and renditions = the ladder).
- In progress: the web player and couch on the core, the Apple player.

## Phase 7: couch - in progress

- Done: couch v2 backend (participant tokens per device, `delivery=body` + `X-Couch-Token`,
  remote role and relay), core `couch` (socket effect, reconnect, host broadcast, follower
  drift sync, remote control, reactions).
- Pending: web couch on the core, Apple couch.

## Phase 8: ranks and profiles - in progress

- Done: core `ranks` and `profile` (rank badge, celebrations, profiles with heatmap,
  leaderboards, edits, uploads through the upload effect).
- Pending: web and Apple adoption.

## Phase 9: Apple system integration - pending

Top Shelf, widget, Live Activity (per spike S1), App Intents, Shortcuts, Spotlight.

## Phase 10: downloads - in progress

- Done: backend download preparation (`requestDownload` plans a device-ready MP4 for the
  profile, `prepare_download` makes it, shared between requests with the same plan, fetched
  through a media grant with ranges, removed after retention; `saveProgress.watchedAt` so
  offline progress never replaces newer progress), verified with real copies and transcodes
  (`TestDownloads`).
- Done: core `downloads` (download effect with relaunch re-attach, preparation polling, the
  offline library per account, artwork kept beside it, playback from the device while offline,
  unsaved progress replayed with the time it was watched, sign-out deleting the account's
  files), the `wallMs` message clock, `PlayerSource.download`, in-file subtitles.
- Pending: the iOS/iPadOS downloads UI and offline mode (background `URLSession` executor),
  Android downloads (Phase 12).

## Phase 11: Android foundation - done

One APK for phones and Google TV (docs/android.md): `CoreRuntime` with OkHttp, WebSocket,
timer, file and Keystore-sealed store executors, `wallMs`, the device profile from
`MediaCodecList`, `player` and `download` answered for now; Material 3 and Compose for TV
design systems; welcome, add server, QR scanning (CameraX + ZXing), password and pairing
sign-in (QR, countdown), Who's watching with the shared-element avatar, the phone account
switcher, settings (servers with the not-encrypted badge, accounts, language, devices,
approving a device); Home, Movies, Series, Genres, My List, Search and the title page on both
idioms with skeleton, stale and failed states, play opening a placeholder. TV screens keep the
remote's place across Back. JVM, Compose UI and Roborazzi tests (phone and TV, en and cs), and
`android.yml` (tests, lint, APK; screenshots verified on macOS). Checked on Android 16 phone and
TV emulators against a local server, the TV signed in by pairing.

## Phase 12: Android playback, couch, ranks, downloads - pending

## Phase 13: polish and release - pending

Accessibility and localization audit, Raspberry Pi performance, build-from-source guides,
release automation, docs, an HTML overview with screenshots of every client.

## Release automation (part of Phase 13) - done

`release.yml` publishes the multi-arch server image (amd64, arm64) to GHCR on a `v*` tag and
creates the GitHub release; `docker.yml` keeps the image building on main and pull requests. The
Rust, Node and Go stages build on the build host (Go cross-compiles), so only the slim runtime
stage runs per platform. Signed APKs join once the Android app exists.

## Done outside the plan

- The web build is served precompressed (brotli/gzip), the wasm core 232 KB instead of 885 KB.
- Fixed by the e2e suite: couch followers now receive `session_ended` before their socket
  closes, guests' artwork keeps its grant with a size, stale SWR answers no longer overwrite a
  newer My List toggle, unhandled data-promise rejections.

## Known issues (backlog)

- Fixed since they were found: sign-in errors are localized by code, a genre page is headed by
  its label (from the core), a couch follower that missed the end learns of it on reconnect,
  and a signed-out page asks for `/me/preferences` once.
- Also fixed: the core refreshes home, a title and My List on a visit after 10 s (listings
  and genres after 60 s), so native clients catch up with changes made on other devices.

## Real-device checklist (for the user)

Collected from the phases as they land; each item is verified on simulators/emulators/Docker
first.

- Apple TV 4K: pairing from an iPhone, Who's watching animation, HDR/Atmos playback (Phase 6).
- iPhone: QR scan of a Connect-a-device code, Keychain persistence across reinstalls.
- Apple apps (full list in docs/apple.md): free-team signing and trusting the certificate,
  wireless pairing to the Apple TV in Xcode, the Local Network prompt and the not-encrypted
  badge, camera QR scans (Connect a device, the TV's pairing QR), approval within the 5 s
  poll and a new code after expiry, the Reduce Motion cross-fade, Keychain and storage across
  the weekly reinstall and a delete-and-reinstall, the display language surviving a relaunch.
- Downloads (Phase 10): a background transfer finishing while the app is suspended or killed,
  playback in airplane mode, progress syncing on reconnect.
- Free personal team: 7-day provisioning, at most 3 apps, wireless pairing to the TV (spike S1).
- Android (full list in docs/android.md): a Google TV device (launcher banner, D-pad focus and
  Back on every screen, the keyboard beside fields, pairing QR scanned across the room), phone
  camera scans, Keystore persistence across reboots and updates, Czech and the largest font,
  TalkBack, Remove animations.
- Raspberry Pi 4 server: HLS v2 with the V4L2 encoder (spike S3): package, trickplay and 720p
  job timings, whether `h264_v4l2m2m` honours `-force_key_frames`, JIT first-segment and
  far-seek latency.
- Apple TV 4K playback v2: HDR10, HLG and Dolby Vision mode switches, E-AC-3/Atmos reaching the
  receiver, native audio and subtitle menus, forced subtitles, trick-play thumbnails, Original
  and ladder startup times, real Dolby Vision 7 FEL and 8.1 releases.
