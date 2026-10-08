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

## Phase 5: browse and titles - done

- Done: core `catalog` (stale-while-revalidate, warm start, search, My List, image URLs), title
  logos (backend, TMDB, admin), logos in the core's views.
- Done: the web's catalog pages on the core (home, listings, genres, title, My List, search;
  notices as toasts; the catalog SWR cache and fetchers deleted), with the My List race, the
  shared warm home on sign-out and episode lengths fixed in the core; navigation at parity
  (revisits 40-100 ms, cold visits one round trip).
- Done: Apple browse and titles on iPhone, iPad and Apple TV (tabs, the TV's sidebar, Home with
  the hero and Continue Watching, Movies, Series, genres, My List, Search, the title page with
  seasons and episodes, notices as toasts), with snapshot references for every state (en, cs at
  the largest text) on simulators pinned to the references' screen scale.

## Phase 6: playback - in progress

- Done: core `playback` (player effect, resume, watched-time accounting, progress saves, JIT
  keepalive and stop, preparing poll, qualities, audio and subtitle choices, next episode,
  shuffle, reload on failure).
- Done: the core's device profile (`CapabilitiesReported(DeviceProfile)`, the contract's
  fixtures round-trip unchanged), `resolvePlayback`/`resolveCouchPlayback` by POST, sources by
  tier (Original = the source file or the remuxed HLS, Auto and renditions = the ladder).
- Done: the web player on the core (`ElementPlayer` over the video element and hls.js, the
  browser's device profile, every tier, track and quality switching, next episode, resume),
  with the web's resume, beacons, JIT keepalive and next-episode code deleted.
- Done: the Apple player (`PlayerController` executing the core's player commands on AVPlayer
  inside `AVPlayerViewController`: every tier, sidecar WebVTT and in-stream subtitles, audio and
  subtitles from the system menus, the Quality menu, next episode, shuffle, PiP, AirPlay, Now
  Playing, display criteria on the TV) and the device profile measured from the hardware.
- Done: the celebrations overlay over the Apple player (with Apple ranks, Phase 8).
- Pending: the exit on hardware (docs/apple.md, items 9 to 15 and 18).

## Phase 7: couch - in progress

- Done: couch v2 backend (participant tokens per device, `delivery=body` + `X-Couch-Token`,
  remote role and relay), core `couch` (socket effect, reconnect, host broadcast, follower
  drift sync, remote control, reactions).
- Done: the web couch on the core (socket effect, anonymous guests through a cookie-mode
  guest endpoint, follower drift sync, reactions, a follower's failed player refetched through
  the couch).
- Done: the Android couch (Phase 12).
- Done, on simulators pending: the Apple couch on iPhone, iPad and Apple TV (`CoreRuntime`
  publishes `couch`; hosting from the TV's transport bar and the touch options menu, the panel
  with the join page's QR code and the code, large on TV; joining by code from Settings or the TV's
  Couch tab, by a scanned QR code or a `couchverse://couch` link, as a viewer or a remote; one
  cover for the player, a waiting follower, the phone as a remote and a session's end; floating
  reactions and the status line over the picture; a follower's own pause reported as the core's
  local pause). Compiled for iOS and tvOS and linted; the runtime's couch tests pass on the Mac.
- Done: the host's account on a second device is the player's remote, never a second host
  (plan D17, 8.7, 10.6): the server seats the host's account joining by code as a remote whether
  it asked to or not, so a plain Join no longer broadcasts "nothing playing" over the player or
  ends the session by leaving (the web, Apple and Android shells already follow the role; the
  web's join page gained a remote for it), and a browser tab is refused a seat on the session
  another tab of it plays for (409 `already_hosting`, one couch cookie per browser).
- Done: joining without an account (D11): the core joins a code that names another server than
  the active account's, or comes without an account, as a guest there (https then http for a
  typed address, `X-Couch-Token`, whole media and artwork URLs on that server, forgotten on
  leaving, at the end and on signing in); the web's join page offers "Open in the app"
  (`couchverse://couch/<code>?server=<origin>`); Apple joins as a guest from the Welcome screen
  (server address and code), from such links and from a scanned join page, which now carry
  their server. Android does the same on phones and TVs: Welcome's "Join a couch session" asks
  for the server's address and the code, `couchverse://couch/<code>?server=` links open over
  any screen (one without a server waits for an account) and a scanned join page brings its
  origin; the couch controls show whenever a couch is live, and a guest gets no "Use as a
  remote". Verified by the JVM tests and screenshots (the guest's join form, Welcome).
- Done: the TV player's info panel lists who is on the couch (a follower's has no episodes),
  and the iPhone and iPad reactions popover's "more" button sends any emoji from the system
  emoji keyboard.
- Pending: the couch snapshot references (with the guest's join form) and the Welcome ones,
  removed for re-recording after its new "Join a couch session" button (and the Settings ones,
  re-recorded for its couch row), recorded on the pinned simulators; the couch walkthrough
  (docs/apple.md, steps 1 to 7) on simulators and devices; Android's guest join on an
  emulator against a local server.

## Phase 8: ranks and profiles - in progress

- Done: core `ranks` and `profile` (rank badge, celebrations, profiles with heatmap,
  leaderboards, edits, uploads through the upload effect).
- Done: the web on the core's ranks, profiles, leaderboards (the viewer's own row and the
  board's size), profile edits with uploads through the upload effect, achievement checks
  (a forced check the server throttled is asked once more) and the devices list over the
  cookie; the web's SWR cache is gone, the core is its only cache.
- Done: Android ranks and profiles (Phase 12).
- Done: Apple ranks and profiles on iPhone, iPad and Apple TV, absent with rankings off:
  profiles (rank ring and XP, bio, stats, a 26-week heatmap, the watch clock, most watched,
  achievements by category, XP sources), leaderboards (metric and period, podium, the viewer's
  own place, the hidden notice), the profile editor (PhotosPicker uploads through the new
  `upload` executor on touch devices, a QR to the web profile for pictures on the TV, name,
  bio, visibility, password), the rank beside the profile in Settings and the TV sidebar
  (Profile and Leaderboard there), unlock celebrations over the app and the player. Host tests
  cover the upload executor and the ranks surfaces; the screens compile for iOS and tvOS with
  logic tests and snapshot cases.
- Pending: recording the ranks snapshot references and re-recording Settings' (removed after
  its new profile rows): `make apple-test` twice on the pinned simulators, then review. The
  real-device checks (docs/apple.md, items 24 to 27).

## Phase 9: Apple system integration - in progress

- Done: `couchverse://title/<slug>` and `couchverse://play/<movie|episode>/<id>` links on iPhone,
  iPad and Apple TV (Android's shapes), through one `OpenRequest` the signed-in tabs take once an
  account is in, which links, Spotlight, intents, the widget and Top Shelf all use.
- Done: the shelf snapshot (`CouchverseShared`, a library without the core): the active account's
  Continue Watching and My List with the extensions' words in the display language, kept from the
  core's views and written to the App Group container, else the app's caches.
- Done, always on (no capability): App Intents and App Shortcuts on iPhone and iPad (open a title,
  continue watching, open My List, join a couch by code; phrases in English and Czech, titles from
  `contract/i18n`, which codegen also writes into the app's own catalog), and Spotlight (Continue
  Watching and My List titles in the account's domain, dropped on sign-out or a switch).
- Done behind `EXTENSIONS_ENABLED` (off by default, docs/apple.md "Outside the app"): the widget
  extension (Continue Watching widget, couch Live Activity with local updates and a stale date)
  and the Top Shelf extension (Continue Watching through the artwork grant URLs). With the switch
  off the extensions compile but stay out of the apps, with no entitlements. Both apps and both
  extensions build for the simulators with it on and off; host tests cover the snapshot, its file
  format, the links and the Spotlight entries; logic tests (compiled, to run with `make
  apple-test`) the links, the requests, the intents' parameters and the Live Activity's content.
- Pending: `make apple-test` on the pinned simulators, and the real-device checks (docs/apple.md,
  items 31 to 37): links, Siri and Shortcuts in both languages, Spotlight, the widget, the Live
  Activity on the Lock Screen and in the Dynamic Island, Top Shelf.
- Blocked on spike S1 (the user's Apple ID): whether a free personal team can provision the App
  Group and the extensions' App IDs, which decides whether the widget, the Live Activity and Top
  Shelf can be on for free-team builds or stay a paid-team option; and spike S6 (Top Shelf images
  through artwork grants, also over plain http) on a real Apple TV. Top Shelf shows Continue
  Watching only; a featured carousel waits for S1/S6 (plan 16).

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
- Done: Android downloads (Phase 12, WorkManager).
- Done: iOS/iPadOS downloads: `DownloadExecutor` over a background `URLSession` (one task per
  file, an earlier launch's transfers picked up by name, resume data kept and refused resume data
  started over, the system waking the app for a transfer that ended while it was gone), the title
  page's download buttons with the quality menu and each state, the Downloads screen from Settings
  (progress, size, storage used, play, retry, remove), offline mode in place of the tabs. Host
  tests cover the executor over fake transfers and a download through the real core.
- Pending: the snapshot references of the downloads screens and of the title and settings screens
  that gained a download control (removed; the next `make apple-test` records them for review),
  and the exit on hardware (docs/apple.md, items 16 to 19).

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

## Phase 12: Android playback, couch, ranks, downloads - in progress

- Done: the Media3 player executing the core's player commands (direct, remux and HLS,
  sidecar and in-stream subtitles, audio by index then language, the quality cap, resume, the
  next-episode countdown, shuffle) on phone and TV layouts with the media session and
  picture-in-picture; the couch (host panel with QR, join by code, QR or link, follower,
  remote, reactions, the ongoing notification); ranks, profiles, leaderboards, celebrations
  and the profile editor with photo-picker uploads; downloads through WorkManager with the
  quality choice, the Downloads screen and offline mode; Watch Next on Google TV and a
  continue-watching widget on phones; signed APKs on release tags (docs/android.md). The core
  now hands native players whole media URLs and reads the rank from the viewer's stats when a
  check is throttled. Verified by the JVM tests (a real ExoPlayer on Robolectric, WorkManager's
  test driver), the Roborazzi screenshots, Android lint and the R8 release build.
- Done: "Who's watching?" as plan 12.3 draws it on Android: each avatar in its rank ring in
  the tier's colour, the rank title revealed on focus, the glow tinted with the focused
  account's banner accent (the phone's picker and switcher sheet too). The core keeps each
  account's rank and banner accent as last seen on the device and persists them with the
  account (`AccountCard.rank` and `.accent`, optional, so the shells' initializers stand);
  the accent comes from the account's profile, read once per banner. Also the leaderboard's
  pinned "where you stand" row (`LeaderboardView.me`, as on the web) and the download
  notification in the display language.
- Pending: a pass on Android 16 phone and TV emulators against a local server with the sample
  media (a web browser joining the TV's couch).
- Blocked: the first signed release needs a keystore in the repository secrets
  (docs/android.md "Signing and releases").

## Phase 13: polish and release - in progress

- Done: accessibility audit of the web (axe on the main pages in `e2e/a11y.spec.ts`): the
  `faint` token and accent-coloured text (the palette's new `ink`, derived by the core for
  every client) meet WCAG AA on every surface, and every control has an accessible name.
- Done: the admin can turn downloads off (`downloadsEnabled`).
- Done: localization audit of the web: every error reaches the user in the display language
  through `problemMessage` (shared `problem_<code>` strings), no hard-coded English left.
- Done: core property tests (proptest: markdown link safety, time text, accent ink contrast,
  bridge robustness) and a rendering budget; the README describes the native apps, playback
  v2, downloads and the Pi notes for them.
- Done: Apple CI runs lint and the core's host tests on GitHub's Xcode 26.5 image and the
  simulator suites wherever Xcode 27 exists (GitHub has no Xcode 27 image yet).
- Done: Android cleanup: the couch and ranks screens in `phone/` and `tv/` packages like the
  other features (screenshots unchanged), Android lint and the Kotlin compiler without
  warnings (picture-in-picture shrinks from the picture, notifications ask for permission on
  Android 13 and newer only, the media session service is exported for the system's
  controls on purpose), the couch XP sources named by the keys the core sends (checked
  against `contract/` by a test), and a focused chosen chip that stays readable on TV.
- Done: the Apple accessibility pass (plan 10.10), every screen audited in code on iPhone, iPad
  and Apple TV: VoiceOver labels, values and headers (episodes' progress, skeletons as loading,
  couch codes digit by digit, rows that name someone without their picture's label, the podium
  read in order, the reactions popover a group), announcements for what appears away from the
  focus (problems, notices, saves, approvals, the couch's status and end), the hero held still
  under VoiceOver and Switch Control with adjustable dots, the devices list reachable with the
  remote; layouts that wrap rather than clip at the accessibility text sizes and the Large
  Content Viewer on the player's small buttons; Reduce Motion (eased rather than sprung layout
  changes, fades instead of slides), Reduce Transparency (the TV's couch panel) and Increase
  Contrast (brighter secondary text and lines, progress tracks and heatmap levels); sidecar
  subtitles in the viewer's caption style. Compiled for iOS and tvOS with logic tests.
- Done: "Who's watching?" on Apple as plan 12.3 draws it: each avatar in its rank ring in the
  tier's colour, the rank title revealed with the focus on TV, the glow tinted with the focused
  profile's banner accent (its identicon's hue without one) and cross-faded as the focus moves;
  the phone's picker, switcher sheet and Settings show the rings too.
- Done: card zoom transitions on iPhone and iPad (posters and backdrops into titles and back),
  and `CoreRuntime` lets go of title pages, listings, profiles and boards no screen has held for
  five minutes, as Android's runtime drops the surfaces nobody collects (host tests).
- Pending: recording the snapshot references removed for re-recording (Who's watching; on TV the
  devices list and the largest-text genres; on iPhone the largest-text genres and next episode)
  and the new ones (Who's watching and the switcher sheet with rank rings); the Apple checks on
  devices (docs/apple.md, items 38 to 44); the TalkBack pass, the HTML overview with
  screenshots of every client, cleanup.

## Release automation (part of Phase 13) - done

`release.yml` publishes the multi-arch server image (amd64, arm64) to GHCR on a `v*` tag and
creates the GitHub release; `docker.yml` keeps the image building on main and pull requests. The
Rust, Node and Go stages build on the build host (Go cross-compiles), so only the slim runtime
stage runs per platform. The release also carries the signed Android APK (keystore from
repository secrets; debug-signed when they are absent, docs/android.md "Signing and releases").

## Done outside the plan

- The web build is served precompressed (brotli/gzip): the wasm core goes out as 282 KB of
  brotli instead of 1.1 MB (2026-10-08, with ranks and devices on the core; 382 KB gzipped,
  95% of the 400 KB budget `core.yml` enforces). Each account's rank and banner accent for
  "Who's watching?" took it to 385 KB gzipped (96%), most of it the code that reads a rank
  back from the store.
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
- Apple couch (docs/apple.md, items 20 to 23 and 28 to 30): the TV's QR panel read across the
  room, an iPhone as the TV's remote, follower drift on an iPhone and an Apple TV over an hour,
  reactions and the emoji keyboard, joining without an account from the web's "Open in the app",
  the couch in the TV's info panel.
- iPhone: QR scan of a Connect-a-device code, Keychain persistence across reinstalls.
- Apple apps (full list in docs/apple.md): free-team signing and trusting the certificate,
  wireless pairing to the Apple TV in Xcode, the Local Network prompt and the not-encrypted
  badge, camera QR scans (Connect a device, the TV's pairing QR), approval within the 5 s
  poll and a new code after expiry, the Reduce Motion cross-fade, Keychain and storage across
  the weekly reinstall and a delete-and-reinstall, the display language surviving a relaunch.
- Apple ranks (Phase 8, docs/apple.md items 24 to 27): profile pictures from the photo
  library (HEIC, PNG, iCloud photos), the TV's QR hand-off to the web profile, celebrations
  over the player on TV and iPhone (Reduce Motion too), profiles and leaderboards with the
  remote, rankings switched off.
- Downloads (Phase 10): a background transfer finishing while the app is suspended or killed,
  playback in airplane mode, progress syncing on reconnect, storage and removal.
- Free personal team: 7-day provisioning, at most 3 apps, wireless pairing to the TV (spike S1).
- Apple system integration (Phase 9, docs/apple.md items 31 to 37): title and play links, Siri
  and Shortcuts phrases in English and Czech, Spotlight results, and with `EXTENSIONS_ENABLED` the
  widget on the home screen, the couch Live Activity on the Lock Screen and in the Dynamic Island,
  Top Shelf on the TV (spike S6) and whether a free team can provision them at all (spike S1).
- Apple accessibility and polish (Phase 13, docs/apple.md items 38 to 44): Who's watching's rank
  rings and banner tints with the remote, zoom transitions from cards, VoiceOver on the iPhone
  and the Apple TV, the largest text sizes, Reduce Motion, Reduce Transparency, Increase
  Contrast and caption styles.
- Android (full list in docs/android.md): a Google TV device (launcher banner, D-pad focus and
  Back on every screen, the keyboard beside fields, pairing QR scanned across the room), phone
  camera scans, Keystore persistence across reboots and updates, Czech and the largest font,
  TalkBack, Remove animations; playback on the TV's decoders with HDR and passthrough, the
  remote's media keys and Watch Next; phone PiP, lock-screen and Bluetooth controls; downloads
  across leaving the app and a reboot, offline playback and progress syncing back; the couch
  notification and a TV follower with a phone host; the widget; updating a signed release.
- Raspberry Pi 4 server: HLS v2 with the V4L2 encoder (spike S3): package, trickplay and 720p
  job timings, whether `h264_v4l2m2m` honours `-force_key_frames`, JIT first-segment and
  far-seek latency.
- Apple TV 4K playback v2: HDR10, HLG and Dolby Vision mode switches, E-AC-3/Atmos reaching the
  receiver, native audio and subtitle menus, forced subtitles, trick-play thumbnails, Original
  and ladder startup times, real Dolby Vision 7 FEL and 8.1 releases.
