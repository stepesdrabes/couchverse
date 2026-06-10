---EXTEND THIS FILE---

commit rule: do commits in format "feat(frontend): test test", "fix(backend): lmao".
commit rule: NO Co-Authored-By / co-author trailers in commit messages.

code styles rule: do not put over-engineered messages into the code -> only actually helpful

punctuation rule: NEVER write an em-dash (U+2014) anywhere - code, comments, strings, docs,
commit messages. Use a plain hyphen `-`. En-dash (U+2013) is OK in numeric/date ranges.

## Project

Couchverse - self-hosted Netflix-like streaming app (movies, series, music) with an admin panel.
Feature inventory + per-feature docs (endpoints, tables, dependency graph): see `FEATURES.md`.

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
    `httpx.StoreErr` maps it to 404). Shared `MediaFile`/`Subtitle` row types + ffprobe/
    compat/namer/tags + transcode ladder policy live in `internal/media`.
- Frontend features live in `frontend/src/lib/features/<name>/` (admin, auth, catalog, jobs,
  library, music, playback, preferences, settings, uploads, users). Each keeps its types,
  API calls (`api.ts`), rune state (`*.svelte.ts`), `components/` and `pages/` together.
  - **Routes are thin shells**: every `src/routes/**/+page.svelte` only imports its
    `XxxPage.svelte` from the owning feature and passes `data` (typed via `PageProps`).
    Page/markup code never lives in `src/routes/`. Loaders (`+page.ts`/`+layout.ts`) stay
    in routes/ (SvelteKit requirement) and delegate to feature `api.ts`.
  - `lib/components/` keeps only domain-free shared UI: `ui/` (bits-ui primitives) and
    `layout/` (TopNav, GlowBackdrop). Domain components live in their feature.
  - Shared catalog entities live in `features/catalog/types.ts`. The bare fetch wrapper
    stays in `src/lib/api/client.ts`. Never hand-write URLs in components.
  - Forms that edit existing data track dirtiness with `FormState`
    (`lib/utils/form-state.svelte.ts`); Save buttons are `disabled={!form.dirty}`
    (disabled, not hidden).

## Stack

- `backend/` - Go (chi, pgx/v5, goose migrations embedded in `backend/migrations/`). Module
  name `couchverse`. The built SPA is embedded from `backend/web/dist` (gitignored,
  populated by `make build`/Docker).
- `frontend/` - SvelteKit (Svelte 5 runes), static SPA (`adapter-static`, `ssr=false`,
  fallback index.html). **bits-ui** primitives styled with **Tailwind v4** (theme tokens in
  `src/app.css` `@theme` - keep that file plain CSS), **SCSS** for component styles
  (`<style lang="scss">`), svelte-sonner for toasts.
- Postgres 17; job queue is a Postgres table (no Redis). ffmpeg/ffprobe shelled out (not installed on the dev Mac - use docker for media work).

## Dev workflow

- `docker compose up db -d` then `make run-backend` (Go on :8080) + `make run-frontend` (Vite on :5173, proxies /api).
  Dev .env: `DB_PASSWORD=couchverse` so the Makefile default DSN works.
- `make lint` (go vet [+ golangci-lint if installed]), `make check` (svelte-check + prettier), `make test`, `make build` (SPA → embed → binary).
- Sample media: `make sample-media` (lavfi-generated clips covering direct-play/remux/transcode tiers).
- Verify HTTP: `curl localhost:8080/healthz`.
