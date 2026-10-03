# Couchverse

Self-hosted, Netflix-style streaming app (movies and series) with an admin panel.
Feature inventory + per-feature docs (endpoints, tables, dependency graph) live in
`FEATURES.md` - read it before touching a feature.

## Conventions

- **Commits**: conventional `type(scope): summary`, e.g. `feat(web): add language
  switcher`, `fix(backend): correct genre fallback`, `docs: document i18n`. Scope is the
  component (`backend`, `web`, `core`, `contract`, `apple`, `android`); summaries lowercase and
  imperative.
- **No co-author trailers**: never add `Co-Authored-By` (or any co-author trailer) to a commit.
- **No em-dash (U+2014)** anywhere - code, comments, strings, docs, commit messages. Use a
  plain hyphen `-`. En-dash (U+2013) is OK only in numeric/date ranges.
- **Comments earn their place**: explain the non-obvious (the why), never restate the code; no
  decorative banners or over-engineered prose.
- **Verify before committing**: `make check` (web: svelte-check + prettier + eslint + vitest) and
  `make lint test` (backend) must be clean; run `make build` when touching embed/build paths
  and `make e2e` when a change touches a user flow.
  Write code that reads like the surrounding code.

## Architecture: feature-based on both sides

A feature owns its HTTP handlers, domain logic and SQL together.

- Backend features live in `backend/internal/feature/<name>/` (analytics, artwork, auth,
  catalog, couch, downloads, jobs, library, metadata, playback, ranks, subtitles, system).
  Each is one Go package with:
  - a per-feature `Store` struct over the shared pgx pool (`NewStore(pool)`) - SQL stays
    inside the feature;
  - handler files plus `routes.go` with a `Register(rt httpx.Routes)` method (on a `Module` or
    the handler struct) that adds the feature's operations to the shared huma API (see "Typed
    API" below); `internal/server` only composes middleware, the API groups and the SPA
    fallback;
  - wiring (stores, services, the job runner) happens in `internal/app`; `cmd/couchverse` only
    loads config, migrates and serves, and tests build the same graph with `app.New`.
  - Import rules: a feature may import another feature's `Store`/exported services, never
    its handlers. The feature import graph must stay acyclic (current DAG in FEATURES.md).
    SQL may JOIN any table - joins create no Go dependency.
  - Shared kernel: `internal/{app,config,db,grant,hls,httpx,media,settings,flags,slug,server,version}`
    (`hls`: playlist parsing and writing, WebVTT segmenting, the HLS validator; `media/mp4`: the
    fMP4 box reader behind codec strings and segment checks).
    `r.RemoteAddr` is always the bare client IP: `internal/server`'s `clientIP` believes
    `X-Forwarded-For` only from `TRUSTED_PROXIES`, so never read forwarding headers yourself.
    `db.ErrNotFound` is the missing-row sentinel (aliased as `httpx.ErrNotFound`; a wrapped
    one returned from a typed handler becomes a 404 automatically). Shared `MediaFile`/`Subtitle`/`AudioStream` row types +
    ffprobe/compat/namer/tags + transcode ladder policy live in `internal/media`.
  - **Couch sessions** (synced watch parties) keep all session/participant state in-memory in
    a Hub (a server restart ends every session); the only persisted artifact is the per-title
    on-couch watch-time in `analytics`. The feature is gated by the admin `couchEnabled` flag
    (mirror `rankingsEnabled`/`flags.CouchOn`). A scoped httpOnly **couch cookie** identifies a
    participant (incl. anonymous ones) for the couch endpoints; followers stream through
    **media grants bound to their participant** (see below), which the composition root
    re-checks against the Hub (`AllowsMedia`) on every media request, so leaving, a media switch
    or the session's end revokes access at once - `playback` never imports `couch`. Reuse
    `playback.BuildPlayback` (no auth context, a `playback.Viewer`) to build follower payloads. WebSockets use `github.com/coder/websocket`. The protocol's frames are listed once
    in `couch/protocol.go` (`ServerFrames`/`ClientFrames`, exported `Couch*` payload types);
    `couchverse couch-schema` publishes them and `contract/fixtures/couch` holds one golden
    frame per type (rerun `go test ./internal/feature/couch -update` after an intended change).
  - **Ranks** (XP, achievements, public profiles, leaderboards) is a near-leaf: it imports only
    `auth` and `catalog`'s `Localize`/`GenreLabel`, reaching every other table by SQL join, and
    **nothing imports it**. That is what lets `couch` report per-user counters through its own
    `CouchStatsRecorder` interface (satisfied by `*ranks.Store`, wired in `main.go`, mirroring
    `CouchWatchRecorder`). Do **not** put rank on `/auth/me` - `ranks -> auth` already exists, so
    that would be a cycle; the nav badge reads `GET /me/stats`. Evaluation is **pull-based**:
    `POST /me/achievements/check` is the only writer of `user_achievements` and is throttled per
    user, so no SQL is added to the 10s progress beacon. Achievement rules are a pure table
    scored against one `Snapshot` (unit-tested without a database); gated rules are absent, not
    locked, when their feature flag is off. Gated by the admin `rankingsEnabled` flag, except
    `/admin/ranks`, which stays reachable so an admin can retune progression while it is off.
    XP rates and tier thresholds are admin-editable (`ranks.RankConfig` in the `ranks` settings
    key) and threaded through the pure functions as an argument rather than read globally.
  - **User-authored markdown** (profile bios) is parsed by the core's `markdown` module into a
    document tree that can only express safe structure (raw HTML arrives as text, images as
    links, links are http(s)/mailto only); that module is the security boundary. The web
    renders the tree with `lib/components/ui/Markdown.svelte` (`MarkdownBlocks`/
    `MarkdownInlines`), native clients with their own views. Never add an `{@html}` path or
    render user markdown any other way.
- **Auth**: browsers use the httpOnly session cookie (CSRF Origin check); native clients are
  named **device sessions** sending `Authorization: Bearer` (`POST /auth/token`, RFC 8628-style
  pairing under `/auth/pairings` approved at `/me/pairings/{code}`, one-time connect codes).
  Both are `sessions` rows with a public id (`GET/DELETE /me/devices`); every session's expiry
  slides on use. `auth.SessionFrom(ctx)` is the authenticating session. On the web, `/pair`
  approves pairing codes (signed-out visitors go through `/login?next=`) and the owner's
  profile ends with `DevicesSection` (cached list, revoke, Connect-a-device QR); revoking this
  browser's own row is `session.logout()`, not `DELETE /me/devices/{id}`, so the cookie is
  cleared. Per-IP limits (login 5/min, pairing and connect 10/min) apply to scripted tests too.
- **Media grants** (`internal/grant`): media is authorized by a signed, expiring capability in
  the path, never by cookie or header, because players (AVPlayer, ExoPlayer, AirPlay, the
  browser's media element) cannot attach either reliably. Everything that plays one file lives
  under `/media/{grant}/...` (`Routes.Media`; the file is `grant.From(ctx).Resource`, never a
  path id), and the playback payload hands out the grant and grant URLs (`streamUrl`,
  `originalUrl`, `hlsUrl`); a JIT session is bound to its grant's viewer. Artwork accepts a
  session or an artwork grant (`?g=`). New media routes go on `Routes.Media` and must only
  touch the granted file.
- **Playback v2** (full design in FEATURES.md "Playback v2"): clients POST a `DeviceProfile` to
  `resolvePlayback`/`resolveCouchPlayback` and it is authoritative (the `GET` variants keep a
  browser baseline). `playback.Decide` is a pure function picking the tier (direct, remux,
  transcode); any decision change gets a case in `decision_test.go`. New ffmpeg HLS outputs go
  through `hlsOutput`/`inputArgs`, which keep every rendition on one fMP4 timeline (`-copyts`,
  the 1.4 s offset, `frag_discont`, negative CTS offsets). Multivariant playlists are written
  in Go from `rendition.json`, never by ffmpeg. Any HLS change must pass `make hls-check` (the
  validator, plus AVFoundation on macOS). Bump `media.ProbeVersion` when the prober learns a
  new fact; the `reprobe` job backfills it.
- **Typed API (huma, the contract's source)**: every route is an operation on one huma API
  (OpenAPI 3.1 at `/api/v1`), registered through `httpx.Routes` groups: `Public`, `User`
  (signed in), `Admin` (paths get `/admin`), `Media` (`/media/{grant}`), `Artwork` (session or
  artwork grant); flag gates are `httpx.Guard` groups. Declare operations with
  `const tag httpx.Tag = "<feature>"` and `tag.Op/NoContent/Created/Accepted(operationId,
  method, path)`; handlers are `func(ctx, *fooInput) (*fooOutput, error)`. **Operation ids are
  the contract** (camelCase, `admin` prefix for admin ops): every client's function names come
  from them, so never rename one casually. Typing rules: exported, domain-named body types (no
  anonymous body structs, no `any`/`map[string]any`/`json.RawMessage` in a schema); response
  lists are never nil; an optional response field is `omitempty`, a nullable scalar is a pointer
  without it; partial-update fields are `required:"false"`; uuid path params are
  `format:"uuid"` (a malformed path param is a 404); closed sets get `enum:`, on response
  fields too (a DB CHECK constraint is a closed set). Parameters must be direct fields of the
  input struct: huma reads none from embedded structs. A multipart body is documented as
  required by `httpx.NewAPI`'s `requireForm` hook. Errors: return the
  plain error for internal failures (logged, generic 500), domain failures with
  `httpx.Fail(status, code, message)` keeping stable codes. `httpx.Localized(op)` for `?lang=`
  reads. `httpx.Raw` only for byte streams, WebSockets and chunked uploads, still fully
  documented (parameters, content types, `httpx.ErrorResponse`). Every typed operation needs at
  least one case in `internal/server/api_test.go` (`TestAPIConformance` runs it against a
  seeded database and validates the response against the spec; `make test` uses the dev
  Postgres).
- **Contract** (`contract/`): `openapi.json` and `couch-protocol.schema.json` are generated from
  the Go code; `i18n/` and `design/tokens.json` are hand-authored; `fixtures/` holds golden
  payloads. `make contract` regenerates the specs and runs `cargo xtask codegen` (`core/xtask`),
  which emits the core's typed API crate (`core/crates/api`), the web's typed client
  (`clients/web/src/lib/generated/api.ts`), the web token CSS/TS, and the Apple/Android
  strings and tokens. Generated files are never edited by hand; CI fails when any is stale.
  Request bodies are JSON, multipart (`FormData` on the web; in Rust a `Call` whose request
  carries `Body::Multipart`) or octet-stream (`Body::Binary`): the core never holds file bytes,
  the shell attaches the file and parses the answer with `Call::parse`. Every generated TS
  function takes a trailing `CallOptions`; raw and upload operations also get an `xxxPath`
  builder returning the full `/api/v1/...` path, so never prepend `/api/v1` by hand.
- **Shared client core** (`core/crates/app`, full design in FEATURES.md): sans-I/O and
  deterministic. Events and effect outputs come in with the shell's `nowMs`; all I/O goes out
  as effects; shells read view models per `Surface`. Rules: no clock, randomness, threads or
  network inside the core (time and any randomness arrive in messages); no UI words (stable
  codes in `Problem`/`Notice`, localized by shells); API types never cross the bridge (map
  them to view models); a module owns its state, its `Pending` continuations and its view
  models, and the model in `core.rs` only routes; requests in flight carry a generation so
  answers for a previous account or language are dropped. Every new flow gets a scenario test
  in `src/scenarios/` (fake shell, explicit time); `make core-test` must be clean. A change to a
  `#[typeshare]` type changes the wire format for every shell: run `make contract` and keep the
  Swift/Kotlin/TS runtimes handling every `Effect` variant.
- Web features live in `clients/web/src/lib/features/<name>/` (admin, auth, catalog, couch,
  jobs, library, playback, preferences, ranks, settings, uploads, users). Each keeps its
  types, API calls (`api.ts`), rune state (`*.svelte.ts`), `components/` and `pages/` together.
  - **Routes are thin shells**: every `src/routes/**/+page.svelte` only imports its
    `XxxPage.svelte` from the owning feature and passes `data` (typed via `PageProps`).
    Page/markup code never lives in `src/routes/`. Loaders (`+page.ts`/`+layout.ts`) stay
    in routes/ (SvelteKit requirement) and delegate to feature `api.ts`.
  - `lib/components/` keeps only domain-free shared UI: `ui/` (bits-ui primitives plus the
    shared widgets `StatTile`/`SegmentBar`/`RankedList`, extracted from the admin dashboard
    once a second feature needed them, and `QrCode`, an SVG over `uqr` with the quiet zone
    drawn in), `layout/` (TopNav, GlowBackdrop, LanguageSwitcher,
    NavProgress), and the optimistic page shells `{CachedView,StreamedView,NotFound}.svelte`.
    Domain components live in their feature.
  - **The core on the web** (`lib/core/`): `core.session` is the only source of the signed-in
    user, feature flags, display language and site accent; read them through `session`/
    `features` and never fetch `/auth/me`, `/features` or `/theme` yourself. After a web API
    call that changes what the session shows (profile, server settings), call
    `session.refresh()` (`SessionChanged`). Loads that read the session `await parent()`; the
    session store follows the core's user, so a session the core finds gone redirects to sign
    in. Viewer catalog pages never fetch `/home`, `/titles`, `/search`, `/genres` or
    `/me/watchlist`: they read the core's views, and My List changes go through
    `WatchlistChanged`. A load must not own `ScreenOpened`/`ScreenClosed` (hover and TV-focus
    preloads run loads for pages that never mount): a mounted page opens its screen with
    `useScreen`. New notice codes need a message in `lib/core/Notices.svelte`. The runtime
    (`runtime.svelte.ts`) stamps `wallMs` and must handle every `Effect` variant: its `never`
    check fails the build when the core gains one.
  - The viewer's catalog shapes are the core's generated view types; `features/catalog/types.ts`
    keeps only the admin-shared `Genre` and `ContentStatus`. The bare fetch wrapper
    stays in `src/lib/api/client.ts`. Never hand-write URLs: use the generated functions or `xxxPath` builders.
  - Forms that edit existing data track dirtiness with `FormState`
    (`lib/utils/form-state.svelte.ts`); Save buttons are `disabled={!form.dirty}`
    (disabled, not hidden).
  - Accent colour is extracted server-side per artwork (stored on `artwork.accent`, returned
    in payloads as `posterAccent`/`backdropAccent` and via the public `/theme` endpoint).
    `lib/theme.ts` turns a hex accent into CSS vars: `applyAccent` sets
    `--color-accent[-strong|-soft]` globally, `accentVars(hex)` returns a scoped `style`
    string and `paletteVars(p)` does the same for a palette the core already derived (the
    title page's `TitleDetailView.accent`); all include a contrast-aware `--color-on-accent` (use
    `text-[var(--color-on-accent)]` on `bg-accent`). Tooltips use `ui/Tooltip.svelte`.
  - **Optimistic navigation** (client-only SPA; must feel snappy on a Pi): data pages never
    block the swap. Catalog pages are core-backed: `+page.ts` calls `features/catalog/api.ts`
    `revisit`/`preload`/`preloadListing` and returns the screen; the page uses `useScreen` and
    passes `shown(view)` and `status` to `CachedView` (content -> skeleton -> notFound ->
    failed with Retry). Other data pages return the cache key + an un-awaited `fresh` promise
    and the feature's `XxxPage.svelte` is a thin `CachedView` wrapper with the body moved to
    `XxxContent.svelte`. The SWR cache (`lib/api/cache.svelte.ts`, `createSwrCache`, cleared on
    logout via `resetAllCaches`; instances in `features/<name>/cache.svelte.ts`) now serves only
    the ranks profile, the leaderboard and the devices list. Skeletons reuse `ui/Skeleton.svelte`.
    `StreamedView` is the uncached keep-last-value variant (admin editors: no skeleton flash
    on an `invalidateAll` save). `preloadData` only for side-effect-free routes - never
    `/watch/...` (starts a JIT transcode); watch links use `data-sveltekit-preload-data="tap"`.
    Prefer targeted `invalidate` over `invalidateAll`. Full design in FEATURES.md.
  - **E2E specs** (`clients/web/e2e/`, Playwright) find elements by role and label, with UI
    strings taken from `contract/i18n` through `t()` in `e2e/fixtures.ts`; prefer real a11y
    hooks (`aria-pressed`, labels) over `data-testid`. Every spec fails on console errors, so an
    expected 401 must be allowed explicitly. Each signed-in spec uses its own seeded account
    (nora, otto, vera, admin) and resets any state it writes, so specs stay parallel- and
    retry-safe. A new user flow gets a spec.
  - **TV mode** (Titan OS smart TVs; `lib/tv/`, full design in FEATURES.md): the same SPA
    with remote-control spatial navigation, Back handling and a 10-foot scale, switched on
    by the `TitanOS/` user agent or `?tv=1`. New viewer UI must stay reachable by arrows:
    use real links/buttons, give hover-only styles a `focus-visible` twin, mark a page's
    starting control `data-tv-autofocus` and fixed chrome `data-tv-pin`, and call
    `preventDefault` on keys a component handles itself.

## Stack

- `backend/` - Go (chi, pgx/v5, goose migrations embedded in `backend/migrations/`). Module
  name `couchverse`. The built SPA is embedded from `backend/web/dist` (gitignored,
  populated by `make build`/Docker) with precompressed `.br`/`.gz` siblings, which the SPA
  handler serves by `Accept-Encoding`.
- `clients/web/` - SvelteKit (Svelte 5 runes), static SPA (`adapter-static`, `ssr=false`,
  fallback index.html). **bits-ui** primitives styled with **Tailwind v4** (colour and radius
  tokens come from the generated `src/lib/generated/tokens.css`, imported by `src/app.css`,
  which stays plain CSS), **SCSS** for component styles
  (`<style lang="scss">`), svelte-sonner for toasts, **Paraglide JS** for i18n.
- `core/` - Rust workspace: the shared client core (`crates/app`), its UniFFI and wasm exports
  (`crates/ffi`, `crates/wasm`), the generated `couchverse-api` crate and `cargo xtask`
  (codegen and packaging). Design in FEATURES.md "Shared client core".
- `clients/apple/`, `clients/android/` - the native clients (docs/native-clients-plan.md).
  Android is one Gradle project (Kotlin DSL, version catalog; AGP 9 built-in Kotlin, JDK 21
  daemon). Swift and Kotlin message types come from typeshare via `cargo xtask codegen`: fix
  quirks in `core/xtask/src/messages.rs`, never by hand; every `Effect`, `Event` and `Surface`
  variant needs a case in the Android `WireFormatTest`, whose coverage check fails otherwise.
  Apple (`clients/apple/`, build guide docs/apple.md): SwiftUI apps for iPhone/iPad and Apple
  TV over three packages (`CouchverseCore`: the bridge, `CoreRuntime` and the effect executors;
  `CouchverseDesign`; `CouchverseFeatures`); Swift 6 with complete strict concurrency, main
  actor by default, warnings as errors. `CoreRuntime` is the only state: screens keep
  presentation state only and send events. New UI strings go in `contract/i18n` and are used
  through `L10n`. TV screens must stay reachable by the remote (focus sections,
  `defaultFocus`). Snapshot references are re-recorded by deleting the old ones after an
  intended visual change; `make apple-lint` before committing.
- `contract/` - the API spec, couch protocol schema, i18n catalogs, design tokens, fixtures.
- Postgres 17; job queue is a Postgres table (no Redis). ffmpeg/ffprobe shelled out.

## Dev workflow

- `docker compose up db -d` then `make run-backend` (Go on :8080) + `make run-web` (Vite on :5173, proxies /api).
  Dev .env: `DB_PASSWORD=couchverse` so the Makefile default DSN works.
- `make lint` (go vet [+ golangci-lint if installed]), `make check` (svelte-check + prettier +
  eslint + vitest), `make test`, `make build` (wasm core -> SPA -> embed -> binary). `make
  run-web`/`check` build the wasm core once if `lib/core/pkg` is missing; rerun `make
  core-wasm` after a core change.
- `make contract` after any change to an operation, a schema type, the couch protocol, i18n,
  design tokens or a `#[typeshare]` type in the core; commit the regenerated files with the
  change.
- Core: `make core-test` (fmt, clippy pedantic with warnings denied, tests); `make
  core-apple|core-android|core-wasm` package it for each shell (gitignored output; rustup
  targets and, for Android, the NDK and `cargo-ndk`); `make apple-test` runs the Swift
  package tests (`CouchverseCore` on the host, `CouchverseDesign` and `CouchverseFeatures` on
  the iOS and tvOS simulators via `xcodebuild -scheme <package>`), `make apple-uitest` the
  apps' UI tests, `make apple-lint`/`apple-format` swift-format, and `make android-test` the
  Kotlin binding tests on the JVM, all against the real core (build-from-source details in
  docs/apple.md and docs/android.md).
- CI (`.github/workflows/`): `backend.yml` (gofmt, vet, golangci-lint, tests incl. API
  conformance against a Postgres service; installs ffmpeg for `TestPlaybackTiers`), `hls.yml`
  (every tier validated and played in AVFoundation on macOS), `e2e.yml` (Playwright in
  Chromium against a Postgres service; traces uploaded on failure), `web.yml` (wasm core, check, lint, vitest, build),
  `contract.yml` (xtask fmt/clippy/tests, `make contract`, no drift), `core.yml` (the wasm package
  built and checked against its budget), `apple.yml` (lint, the package tests and the apps
  built for the simulators), `android.yml` (`make core-android`, the core's JVM tests, the
  Gradle modules assembled), `repo.yml` (`scripts/check-no-emdash.sh`), `docker.yml` (the
  image builds) and `release.yml` (multi-arch image to GHCR on a `v*` tag).
- Sample media: `make sample-media` (lavfi-generated clips covering every tier; Dolby Vision
  samples need `dovi_tool` and `mkvmerge`). Against a running server: `make ingest-samples
  hls-server-check e2e-playback SERVER=...`.
- `make e2e` runs the Playwright smoke suite (Chromium then WebKit) against the built binary on
  a throwaway database on the dev Postgres (`scripts/e2e-server.sh`: `seed.sql` +
  `e2e/setup.sql`, a generated clip, placeholder artwork; needs the compose `db` and ffmpeg;
  `E2E_SKIP_BUILD=1` reuses the last build). A change to `seed.sql` must keep it green.
- Verify HTTP: `curl localhost:8080/healthz`.

## Internationalization & multi-language media

Full design in `FEATURES.md`; the conventions to follow:

- **One display language** (en/cs) drives both UI strings and shown metadata. UI strings live
  in `contract/i18n/{en,cs}.json` for every client (Paraglide on the web, generated string
  catalogs on Apple/Android); both files must define the same keys, shapes and parameters
  (`make contract` checks). Counted strings use plural variants (Czech needs one/few/many/
  other; see `catalog_season_count`); a message with two counts takes pluralized fragments as
  parameters. Admin-only keys (`admin_`, `jobs_`, `library_`, `settings_`, `uploads_`,
  `users_`) are left out of the native catalogs. On the web call `m.key()`
  (`import * as m from '$lib/paraglide/messages'`); generated `src/lib/paraglide/` is
  gitignored and compiled by `make check`/`build`. **Parameterize, never concatenate** (Czech
  word order). The language source of truth is `lib/i18n/locale.svelte.ts`
  (`currentLang`/`setDisplayLang`); `lib/api/client.ts` appends `?lang=` to requests.
- **Content metadata** is stored per language in `translations jsonb` on
  `titles/seasons/episodes/genres`, base columns are the default/fallback;
  `titles.metadata_languages` is the per-title content set. The backend resolves at scan via
  `catalog.Localize` keyed off `httpx.LangFrom` (set by `httpx.Localized` on public reads;
  admin reads and background jobs leave it empty -> base text). Admins edit each
  language in the title editor / episode modal (`PATCH /admin/titles/{id}/translations/{lang}`,
  `.../episodes/{id}/translations/{lang}`); TMDB fetch loops over the title's languages.
  Genres keep their English `name` as the URL/filter identity - only the label is translated
  (`genreLabel`). Content-language list lives in `lib/i18n/content-langs.ts` (`CONTENT_LANGS`
  endonyms + the bundled `ALL_LANG_CODES` ISO 639-1 set + `searchLangs` for the
  `AddLanguageModal`); language flags use the bundled `flag-icons` via
  `lib/components/ui/Flag.svelte` + `lib/i18n/flags.ts` (`langToCountry`, e.g. en->gb, cs->cz).
  Removing a content language is destructive and cross-feature: catalog's
  `DELETE /admin/titles/{id}/languages/{lang}` drops its translations and promotes the next
  language to base (the last one is protected), and the editor also hard-deletes that
  language's alternate-audio files (`DELETE /admin/media-files/{id}` - source + caches +
  subtitles on disk) and subtitle tracks, behind a warning modal; the server also deletes that
  language's logo.
- **Title logos** are per content language: `artwork.lang` (ISO 639-1, null for art not tied
  to a language), one `kind='logo'` row per (title, language), a slot being `(owner_kind,
  owner_id, kind, lang)`. Viewer reads expose `logoId`/`logoVer`/`logoAspect` picked by
  `artwork.PickLogo` (display language, base language, language-neutral, any). Logos are PNG
  only and resize to PNG; TMDB apply replaces its own logos and keeps uploaded ones. Every
  artwork save records the image's width and height (0 for WebP). Clients show them through
  the core's `Logo` (the web on the hero and the title page, the name kept as screen-reader
  text and as the fallback).
- **Multi-language audio** (chosen in the player, independent of the display language): model B
  is a separate file per language (`media_files.audio_lang`/`audio_role`, tagged via
  `PATCH /admin/media-files/{id}`; the player swaps source + re-seeks); model A is one file
  with embedded tracks (`audio_streams` table), packaged as an HLS audio group of AAC stereo
  plus surround renditions and switched via hls.js `audioTrack` (`-var_stream_map`
  `multiaudio` is legacy only). Both surface as `playbackInfo.audio`.
- ffmpeg/HLS paths run locally when ffmpeg is installed (Homebrew, which lacks zscale); the
  Docker image's ffmpeg (Debian trixie, 7.1) is the reference for HLS output. Dolby Vision
  signalling needs ffmpeg 6+.
