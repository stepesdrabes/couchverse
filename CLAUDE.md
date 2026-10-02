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
- **Verify before committing**: `make check` (web: svelte-check + prettier + eslint) and
  `make lint test` (backend) must be clean; run `make build` when touching embed/build paths.
  Write code that reads like the surrounding code.

## Architecture: feature-based on both sides

A feature owns its HTTP handlers, domain logic and SQL together.

- Backend features live in `backend/internal/feature/<name>/` (analytics, artwork, auth,
  catalog, couch, jobs, library, metadata, playback, ranks, subtitles, system). Each is
  one Go package with:
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
  - Shared kernel: `internal/{app,config,db,httpx,media,settings,flags,slug,server,version}`.
    `r.RemoteAddr` is always the bare client IP: `internal/server`'s `clientIP` believes
    `X-Forwarded-For` only from `TRUSTED_PROXIES`, so never read forwarding headers yourself.
    `db.ErrNotFound` is the missing-row sentinel (aliased as `httpx.ErrNotFound`; a wrapped
    one returned from a typed handler becomes a 404 automatically). Shared `MediaFile`/`Subtitle`/`AudioStream` row types +
    ffprobe/compat/namer/tags + transcode ladder policy live in `internal/media`.
  - **Couch sessions** (synced watch parties) keep all session/participant state in-memory in
    a Hub (a server restart ends every session); the only persisted artifact is the per-title
    on-couch watch-time in `analytics`. The feature is gated by the admin `couchEnabled` flag
    (mirror `rankingsEnabled`/`flags.CouchOn`). **Anonymous viewers stream the host's current
    media via a scoped httpOnly couch cookie** (mirrors the auth session cookie). Because
    `playback` owns the stream routes and must not import `couch`, the anonymous-stream guard is
    inverted into the composition root: `internal/server`'s `authOrCouch` guard allows a request
    if it is logged-in **or** the Hub's `AllowsAnon(r, mediaFileID)` accepts it (live session,
    current media only). Reuse `playback.BuildPlayback` (no auth context) to build follower
    payloads. WebSockets use `github.com/coder/websocket`. The protocol's frames are listed once
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
  - **User-authored markdown** (profile bios) renders through `lib/utils/markdown.ts` - one
    shared markdown-it instance with `html: false`. That is the security boundary: raw HTML is
    escaped rather than parsed, so the `{@html}` in `ui/Markdown.svelte` can only emit tags
    markdown-it generated itself. Never enable `html`, and never render user markdown any
    other way.
- **Typed API (huma, the contract's source)**: every route is an operation on one huma API
  (OpenAPI 3.1 at `/api/v1`), registered through `httpx.Routes` groups: `Public`, `User`
  (signed in), `Admin` (paths get `/admin`), `Stream` (signed in or a couch follower of that
  media); flag gates are `httpx.Guard` groups. Declare operations with
  `const tag httpx.Tag = "<feature>"` and `tag.Op/NoContent/Created/Accepted(operationId,
  method, path)`; handlers are `func(ctx, *fooInput) (*fooOutput, error)`. **Operation ids are
  the contract** (camelCase, `admin` prefix for admin ops): every client's function names come
  from them, so never rename one casually. Typing rules: exported, domain-named body types (no
  anonymous body structs, no `any`/`map[string]any`/`json.RawMessage` in a schema); response
  lists are never nil; an optional response field is `omitempty`, a nullable scalar is a pointer
  without it; partial-update fields are `required:"false"`; uuid path params are
  `format:"uuid"` (a malformed path param is a 404); closed sets get `enum:`. Errors: return the
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
- Web features live in `clients/web/src/lib/features/<name>/` (admin, auth, catalog, couch,
  jobs, library, playback, preferences, ranks, settings, uploads, users). Each keeps its
  types, API calls (`api.ts`), rune state (`*.svelte.ts`), `components/` and `pages/` together.
  - **Routes are thin shells**: every `src/routes/**/+page.svelte` only imports its
    `XxxPage.svelte` from the owning feature and passes `data` (typed via `PageProps`).
    Page/markup code never lives in `src/routes/`. Loaders (`+page.ts`/`+layout.ts`) stay
    in routes/ (SvelteKit requirement) and delegate to feature `api.ts`.
  - `lib/components/` keeps only domain-free shared UI: `ui/` (bits-ui primitives plus the
    shared widgets `StatTile`/`SegmentBar`/`RankedList`, extracted from the admin dashboard
    once a second feature needed them), `layout/` (TopNav, GlowBackdrop, LanguageSwitcher,
    NavProgress), and the optimistic page shells `{CachedView,StreamedView,NotFound}.svelte`.
    Domain components live in their feature.
  - Shared catalog entities live in `features/catalog/types.ts`. The bare fetch wrapper
    stays in `src/lib/api/client.ts`. Never hand-write URLs in components.
  - Forms that edit existing data track dirtiness with `FormState`
    (`lib/utils/form-state.svelte.ts`); Save buttons are `disabled={!form.dirty}`
    (disabled, not hidden).
  - Accent colour is extracted server-side per artwork (stored on `artwork.accent`, returned
    in payloads as `posterAccent`/`backdropAccent` and via the public `/theme` endpoint).
    `lib/theme.ts` turns a hex accent into CSS vars: `applyAccent` sets
    `--color-accent[-strong|-soft]` globally, `accentVars(hex)` returns a scoped `style`
    string; both include a contrast-aware `--color-on-accent` (use
    `text-[var(--color-on-accent)]` on `bg-accent`). Tooltips use `ui/Tooltip.svelte`.
  - **Optimistic navigation** (client-only SPA; must feel snappy on a Pi): data pages never
    block the swap. `+page.ts` returns the cache key + an un-awaited `fresh` promise; the
    feature's `XxxPage.svelte` is a thin `CachedView` wrapper (cached `content` -> else
    `skeleton` -> else `notFound`) with the body moved to `XxxContent.svelte`. SWR cache is
    `lib/api/cache.svelte.ts` (`createSwrCache`, cleared on logout via `resetAllCaches`);
    instances in `features/<name>/cache.svelte.ts`. Skeletons reuse `ui/Skeleton.svelte`.
    `StreamedView` is the uncached keep-last-value variant (admin editors: no skeleton flash
    on an `invalidateAll` save). `preloadData` only for side-effect-free routes - never
    `/watch/...` (starts a JIT transcode); watch links use `data-sveltekit-preload-data="tap"`.
    Prefer targeted `invalidate` over `invalidateAll`. Full design in FEATURES.md.
  - **TV mode** (Titan OS smart TVs; `lib/tv/`, full design in FEATURES.md): the same SPA
    with remote-control spatial navigation, Back handling and a 10-foot scale, switched on
    by the `TitanOS/` user agent or `?tv=1`. New viewer UI must stay reachable by arrows:
    use real links/buttons, give hover-only styles a `focus-visible` twin, mark a page's
    starting control `data-tv-autofocus` and fixed chrome `data-tv-pin`, and call
    `preventDefault` on keys a component handles itself.

## Stack

- `backend/` - Go (chi, pgx/v5, goose migrations embedded in `backend/migrations/`). Module
  name `couchverse`. The built SPA is embedded from `backend/web/dist` (gitignored,
  populated by `make build`/Docker).
- `clients/web/` - SvelteKit (Svelte 5 runes), static SPA (`adapter-static`, `ssr=false`,
  fallback index.html). **bits-ui** primitives styled with **Tailwind v4** (colour and radius
  tokens come from the generated `src/lib/generated/tokens.css`, imported by `src/app.css`,
  which stays plain CSS), **SCSS** for component styles
  (`<style lang="scss">`), svelte-sonner for toasts, **Paraglide JS** for i18n.
- `core/` - Rust workspace (`cargo xtask` lives here): today the generators and the generated
  `couchverse-api` crate; the shared client core follows (docs/native-clients-plan.md).
- `contract/` - the API spec, couch protocol schema, i18n catalogs, design tokens, fixtures.
- Postgres 17; job queue is a Postgres table (no Redis). ffmpeg/ffprobe shelled out.

## Dev workflow

- `docker compose up db -d` then `make run-backend` (Go on :8080) + `make run-web` (Vite on :5173, proxies /api).
  Dev .env: `DB_PASSWORD=couchverse` so the Makefile default DSN works.
- `make lint` (go vet [+ golangci-lint if installed]), `make check` (svelte-check + prettier + eslint), `make test`, `make build` (SPA -> embed -> binary).
- `make contract` after any change to an operation, a schema type, the couch protocol, i18n or
  design tokens; commit the regenerated files with the change.
- CI (`.github/workflows/`): `backend.yml` (gofmt, vet, golangci-lint, tests incl. API
  conformance against a Postgres service), `web.yml` (check, lint, build), `contract.yml`
  (xtask fmt/clippy/tests, `make contract`, no drift), `repo.yml` (`scripts/check-no-emdash.sh`).
- Sample media: `make sample-media` (lavfi-generated clips covering direct-play/remux/transcode tiers).
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
  subtitles on disk) and subtitle tracks, behind a warning modal.
- **Multi-language audio** (chosen in the player, independent of the display language): model B
  is a separate file per language (`media_files.audio_lang`/`audio_role`, tagged via
  `PATCH /admin/media-files/{id}`; the player swaps source + re-seeks); model A is one file
  with embedded tracks (`audio_streams` table, ffmpeg `-var_stream_map` `multiaudio` HLS
  variant, switched via hls.js `audioTrack`). Both surface as `playbackInfo.audio`.
- ffmpeg/HLS paths run locally when ffmpeg is installed (Homebrew); the Docker image's ffmpeg
  (Debian) is the reference for HLS output, exercised with `make sample-media`.
