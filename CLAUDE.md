# Couchverse

Self-hosted, Netflix-style streaming app (movies, series, music) with an admin panel.
Feature inventory + per-feature docs (endpoints, tables, dependency graph) live in
`FEATURES.md` - read it before touching a feature.

## Conventions

- **Commits**: conventional `type(scope): summary`, e.g. `feat(frontend): add language
  switcher`, `fix(backend): correct genre fallback`, `docs: document i18n`. Scope is usually
  `frontend` or `backend`; summaries lowercase and imperative.
- **No co-author trailers**: never add `Co-Authored-By` (or any co-author trailer) to a commit.
- **No em-dash (U+2014)** anywhere - code, comments, strings, docs, commit messages. Use a
  plain hyphen `-`. En-dash (U+2013) is OK only in numeric/date ranges.
- **Comments earn their place**: explain the non-obvious (the why), never restate the code; no
  decorative banners or over-engineered prose.
- **Verify before committing**: `make check` (frontend: svelte-check + prettier) and
  `make lint test` (backend) must be clean; run `make build` when touching embed/build paths.
  Write code that reads like the surrounding code.

## Architecture: feature-based on both sides

A feature owns its HTTP handlers, domain logic and SQL together.

- Backend features live in `backend/internal/feature/<name>/` (analytics, artwork, auth,
  catalog, jobs, library, metadata, music, playback, subtitles, system). Each is one Go
  package with:
  - a per-feature `Store` struct over the shared pgx pool (`NewStore(pool)`) - SQL stays
    inside the feature;
  - handler files plus `routes.go` exposing `Mount*` methods (or a `Module`) that register
    chi routes; `internal/server` only composes middleware + feature mounts + SPA fallback;
  - wiring happens in `cmd/couchverse/main.go` (stores/services constructed there).
  - Import rules: a feature may import another feature's `Store`/exported services, never
    its handlers. The feature import graph must stay acyclic (current DAG in FEATURES.md).
    SQL may JOIN any table - joins create no Go dependency.
  - Shared kernel: `internal/{config,db,httpx,media,settings,flags,slug,server}`.
    `db.ErrNotFound` is the missing-row sentinel (aliased as `httpx.ErrNotFound`;
    `httpx.StoreErr` maps it to 404). Shared `MediaFile`/`Subtitle`/`AudioStream` row types +
    ffprobe/compat/namer/tags + transcode ladder policy live in `internal/media`.
- Frontend features live in `frontend/src/lib/features/<name>/` (admin, auth, catalog, jobs,
  library, music, playback, preferences, settings, uploads, users). Each keeps its types,
  API calls (`api.ts`), rune state (`*.svelte.ts`), `components/` and `pages/` together.
  - **Routes are thin shells**: every `src/routes/**/+page.svelte` only imports its
    `XxxPage.svelte` from the owning feature and passes `data` (typed via `PageProps`).
    Page/markup code never lives in `src/routes/`. Loaders (`+page.ts`/`+layout.ts`) stay
    in routes/ (SvelteKit requirement) and delegate to feature `api.ts`.
  - `lib/components/` keeps only domain-free shared UI: `ui/` (bits-ui primitives) and
    `layout/` (TopNav, GlowBackdrop, LanguageSwitcher). Domain components live in their feature.
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

## Stack

- `backend/` - Go (chi, pgx/v5, goose migrations embedded in `backend/migrations/`). Module
  name `couchverse`. The built SPA is embedded from `backend/web/dist` (gitignored,
  populated by `make build`/Docker).
- `frontend/` - SvelteKit (Svelte 5 runes), static SPA (`adapter-static`, `ssr=false`,
  fallback index.html). **bits-ui** primitives styled with **Tailwind v4** (theme tokens in
  `src/app.css` `@theme` - keep that file plain CSS), **SCSS** for component styles
  (`<style lang="scss">`), svelte-sonner for toasts, **Paraglide JS** for i18n.
- Postgres 17; job queue is a Postgres table (no Redis). ffmpeg/ffprobe shelled out (not
  installed on the dev Mac - use docker for media work).

## Dev workflow

- `docker compose up db -d` then `make run-backend` (Go on :8080) + `make run-frontend` (Vite on :5173, proxies /api).
  Dev .env: `DB_PASSWORD=couchverse` so the Makefile default DSN works.
- `make lint` (go vet [+ golangci-lint if installed]), `make check` (svelte-check + prettier), `make test`, `make build` (SPA -> embed -> binary).
- Sample media: `make sample-media` (lavfi-generated clips covering direct-play/remux/transcode tiers).
- Verify HTTP: `curl localhost:8080/healthz`.

## Internationalization & multi-language media

Full design in `FEATURES.md`; the conventions to follow:

- **One display language** (en/cs) drives both UI strings and shown metadata. UI strings use
  Paraglide: add keys to `frontend/messages/{en,cs}.json` and call `m.key()`
  (`import * as m from '$lib/paraglide/messages'`); generated `src/lib/paraglide/` is
  gitignored and compiled by `make check`/`build`. **Parameterize, never concatenate** (Czech
  word order). The language source of truth is `lib/i18n/locale.svelte.ts`
  (`currentLang`/`setDisplayLang`); `lib/api/client.ts` appends `?lang=` to requests.
- **Content metadata** is stored per language in `translations jsonb` on
  `titles/seasons/episodes/genres`, base columns are the default/fallback;
  `titles.metadata_languages` is the per-title content set. The backend resolves at scan via
  `catalog.localize` keyed off `httpx.LangFrom` (set by the `withLang` wrapper on public read
  routes; admin reads and background jobs leave it empty -> base text). Admins edit each
  language in the title editor / episode modal (`PATCH /admin/titles/{id}/translations/{lang}`,
  `.../episodes/{id}/translations/{lang}`); TMDB fetch loops over the title's languages.
  Genres keep their English `name` as the URL/filter identity - only the label is translated
  (`genreLabel`). Content-language list lives in `lib/i18n/content-langs.ts`.
- **Multi-language audio** (chosen in the player, independent of the display language): model B
  is a separate file per language (`media_files.audio_lang`/`audio_role`, tagged via
  `PATCH /admin/media-files/{id}`; the player swaps source + re-seeks); model A is one file
  with embedded tracks (`audio_streams` table, ffmpeg `-var_stream_map` `multiaudio` HLS
  variant, switched via hls.js `audioTrack`). Both surface as `playbackInfo.audio`.
- ffmpeg/HLS paths cannot run on the dev Mac - exercise them with Docker + `make sample-media`.
