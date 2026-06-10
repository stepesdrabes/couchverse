---EXTEND THIS FILE---

commit rule: do commits in format "feat(frontend): test test", "fix(backend): lmao".
commit rule: NO Co-Authored-By / co-author trailers in commit messages.

code styles rule: do not put over-engineered messages into the code -> only actually helpful

## Project

Couchverse - self-hosted Netflix-like streaming app (movies, series, music) with an admin panel.
Full plan: see README + `/Users/prace/.claude/plans/make-the-design-crispy-rabbit.md` history.

## Stack & layout

- `backend/` - Go (chi, pgx/v5, goose migrations embedded in `backend/migrations/`). Module name `couchverse`.
  Domain logic in `internal/*` packages; HTTP handlers in `internal/api` stay thin; all SQL lives in `internal/store`.
  The built SPA is embedded from `backend/web/dist` (gitignored, populated by `make build`/Docker).
- `frontend/` - SvelteKit (Svelte 5 runes), static SPA (`adapter-static`, `ssr=false`, fallback index.html).
  - **bits-ui** headless primitives wrapped in `src/lib/components/ui/*`, styled with **Tailwind v4** (theme tokens in `src/app.css` `@theme` - keep that file plain CSS, Tailwind is its own preprocessor).
  - **SCSS** for component styles: `<style lang="scss">` (vitePreprocess + sass-embedded).
  - svelte-sonner for toasts.
  - **Feature modules** in `src/lib/features/<feature>/` - each feature keeps its types, API calls
    (`api.ts`) and rune state (`*.svelte.ts`) together (auth, catalog, library, jobs, users, settings;
    later: playback, music, uploads). Shared catalog entities live in `features/catalog/types.ts`.
    The bare fetch wrapper stays in `src/lib/api/client.ts`. Never hand-write URLs in components.
- Postgres 17; job queue is a Postgres table (no Redis). ffmpeg/ffprobe shelled out (not installed on the dev Mac - use docker for media work).

## Dev workflow

- `docker compose up db -d` then `make run-backend` (Go on :8080) + `make run-frontend` (Vite on :5173, proxies /api).
  Dev .env: `DB_PASSWORD=couchverse` so the Makefile default DSN works.
- `make lint` (go vet [+ golangci-lint if installed]), `make check` (svelte-check + prettier), `make test`, `make build` (SPA → embed → binary).
- Sample media: `make sample-media` (lavfi-generated clips covering direct-play/remux/transcode tiers).
- Verify HTTP: `curl localhost:8080/healthz`.
