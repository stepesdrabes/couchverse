# Couchverse features

Reference for AI-agent development: every feature, where its code lives on each side
of the stack, what it talks to, and how the pieces depend on each other.

Architecture rule: **a feature owns its HTTP handlers, domain logic and SQL together.**
Backend features live in `backend/internal/feature/<name>/` (one Go package each, with a
per-feature `Store` over the shared pgx pool and `Mount*` methods that register routes).
Frontend features live in `frontend/src/lib/features/<name>/` (api.ts, types, rune state
in `*.svelte.ts`, `components/`, `pages/`). Route files in `src/routes/` are thin shells
that render a `XxxPage.svelte` component from the owning feature.

## Feature index

| Feature | Backend package | Frontend module(s) | DB tables |
|---|---|---|---|
| auth | `internal/feature/auth` | `features/auth`, `features/users`, `features/preferences` | `users`, `sessions` |
| catalog | `internal/feature/catalog` | `features/catalog` | `titles`, `seasons`, `episodes`, `genres`, `title_genres`, `watch_progress`, `watchlist` |
| music | `internal/feature/music` | `features/music` | `artists`, `albums`, `tracks`, `album_genres`, `play_history`, `playlists`, `playlist_tracks` |
| playback | `internal/feature/playback` | `features/playback` | (reads `media_files`, `transcode_variants`) |
| library | `internal/feature/library` | `features/library`, `features/uploads` | `libraries`, `media_files`, `upload_sessions`, `transcode_variants` |
| metadata | `internal/feature/metadata` | (admin UI in `features/library`) | (writes catalog tables) |
| subtitles | `internal/feature/subtitles` | (player UI in `features/playback`/`preferences`) | `subtitles` |
| artwork | `internal/feature/artwork` | (via `catalog/api.artworkUrl`) | `artwork` |
| jobs | `internal/feature/jobs` | `features/jobs` | `jobs` |
| system | `internal/feature/system` | `features/settings`, `features/admin` | `settings`, `home_rows` |
| analytics | `internal/feature/analytics` | (charts in `features/admin`) | `watch_time_daily` |

Shared kernel (backend): `internal/config` (env), `internal/db` (pool, migrations,
`ErrNotFound`), `internal/httpx` (JSON responses, param helpers), `internal/media`
(ffprobe, codec compatibility, filename parsing, audio tags, transcode ladder policy,
shared `MediaFile`/`Subtitle` row types), `internal/settings` (settings KV store),
`internal/flags` (admin-toggleable feature flags), `internal/slug`, `internal/server`
(composition root: middleware, feature mounts, SPA fallback).

Shared frontend: `lib/api/client.ts` (fetch wrapper - never hand-write URLs in
components), `lib/components/ui/` (bits-ui primitives), `lib/components/layout/`
(TopNav, GlowBackdrop), `lib/theme.ts` (accent), `lib/utils/`.

## Features

### auth
Login/logout with cookie sessions (CSRF origin check, login rate limiting), the
`/auth/me` identity endpoint, user profiles (display name, avatar), per-user
preferences (subtitle appearance), and the admin user CRUD. Bootstraps the master
admin account on a fresh database.
- Backend: `Module` mounts public (`POST /auth/login|logout`), user (`/auth/me`,
  `/me/profile`, `/me/preferences`, `/me/avatar`) and admin (`/admin/users...`) routes.
  Session middleware (`Load`, `RequireAuth`, `RequireAdmin`, `UserFrom`) lives here and
  is used by the server for every authenticated route group.
- Frontend: `features/auth` (session singleton + 401 handler, LoginPage, ProfilePage),
  `features/users` (AdminUsersPage), `features/preferences` (subtitle settings store).

### catalog
The watchable catalog: movies and series with seasons/episodes and genres, the home
page (featured title, admin-curated rows, continue watching), browse with filters,
full-text search, per-user watch progress and My List (watchlist). Admin side: the
library table, title/season/episode CRUD and bulk actions.
- Endpoints: `/home`, `/titles`, `/titles/{slug}`, `/search`, `/genres`, `/progress`,
  `/me/continue-watching`, `/me/watchlist...`; admin `/admin/library`, `/admin/titles...`,
  `/admin/seasons/{id}...`, `/admin/episodes/{id}`.
- Frontend pages: HomePage, MoviesPage, SeriesPage, GenresPage, GenrePage, MyListPage,
  SearchPage, TitleDetailPage; components HeroMarquee, MediaRow, PosterCard, TitleCard,
  ContinueWatchingCard, BrowseGrid, Artwork. `features/catalog/types.ts` is the shared
  type hub for card/row shapes. The home hero and the title pages are accented from the
  banner palette (`lib/utils/palette.svelte` `bannerAccent` + `lib/theme.accentVars`,
  which also emits a contrast-aware `--color-on-accent`). Episode rows show thumbnails
  (`Episode.thumbId`, from TMDB stills).

### music
Spotify-style music: albums, artists, tracks, playlists (create/rename/reorder),
scrobbling (`POST /plays`, which also feeds analytics) and recently-played rows. The persistent bottom player and
queue live entirely on the frontend. All music routes (user and admin) are gated by
the `musicEnabled` feature flag - the gating lives inside the music module's mounts.
- Endpoints: `/music`, `/music/albums/{id}`, `/music/artists/{id}`, `/plays`,
  `/me/playlists...`; admin `/admin/music`, `/admin/albums/{id}`, `/admin/tracks/{id}`.
- Frontend: `player.svelte.ts` (module-scope Audio element that survives navigation,
  Media Session API), PlayerBar/QueuePanel/TrackList/AlbumCard, pages MusicHomePage,
  AlbumPage, ArtistPage, PlaylistPage. Admin album editing UI lives in
  `features/library` (AdminAlbumPage) because the admin endpoints do.

### playback
Everything that turns a media file into pixels: direct play streaming with range
requests, prepared HLS variants, JIT ("instant play") transcode sessions with
seek-anywhere, the playback-info decision endpoint, the background transcode job
engine (ffmpeg HLS encode, hardware encoder detection/probing) and transcode admin.
- Endpoints: `/stream/{id}`, `/stream/{id}/frame?t=` (seek-preview still, ffmpeg input-seek
  cached under `cache/frames`), `/stream/{id}/hls/...`, `/stream/{id}/sessions`,
  `/stream/sessions/{sid}/...`, `/playback/{kind}/{id}` (its `display.backdropId` accents
  the player); admin `/admin/transcode/info|active`,
  `/admin/media-files/{id}/transcode|variants`, `/admin/transcode-variants/{id}`.
- Job handler: `transcode_hls` (per-type concurrency = `maxConcurrent` setting).
- Frontend: WatchPage + VideoPlayer (HLS.js, subtitles, shortcuts, progress beacons incl.
  watched-seconds deltas, JIT keepalive, banner-accented chrome, bits-ui control tooltips,
  seek-bar time + frame preview - splitting it is a known follow-up).
- Transcode ladder/settings policy lives in the `media` kernel so library's prober can
  auto-prepare variants without importing playback. Rendition bitrates are capped at
  the source bitrate (`Rendition.CappedAt`) so transcodes never outweigh their source;
  variant sizes are measured into `transcode_variants.size_bytes` when a job finishes.

### library
Media ingestion: library folders on disk, the scan -> probe pipeline (filename parsing
to draft titles/episodes, audio tags to artists/albums/tracks, direct-play detection,
auto-prepare of HLS variants), resumable chunked uploads, and ownership of the
`media_files` + `transcode_variants` SQL that playback reads.
- Endpoints (admin): `/admin/libraries...` (+ `/scan`, `/scan-all`), `/admin/uploads...`.
- Job handlers: `scan_library`, `probe`.
- Frontend: `features/library` (AdminLibraryPage, AdminTitleEditorPage, AdminAlbumPage,
  editor components incl. EditorHero with hover poster/backdrop editing, NewTitleModal,
  TmdbSearchModal, FileVariants, AdminMusicTable), `features/uploads` (upload queue
  store, AdminUploadsPage, EditorUploadCard).

### metadata
TMDB integration: search, one-click apply of metadata + poster/backdrop to a title,
and bulk import of missing seasons/episodes for a series (episode stills are downloaded
as episode `thumb` artwork). API key comes from settings.
- Endpoints (admin): `/admin/metadata/search`, `/admin/titles/{id}/metadata/apply`,
  `/admin/titles/{id}/metadata/seasons`, `/admin/titles/{id}/metadata/import-episodes`.
- Job handlers: `fetch_metadata`, `import_episodes`.

### subtitles
Side-car WebVTT subtitles: automatic extraction of embedded text subs (ffmpeg),
`.srt`/`.vtt` upload with conversion, serving as `<track>` elements.
- Endpoints: `/subtitles/{id}.vtt`; admin `/admin/media-files/{id}/subtitles`,
  `/admin/subtitles/{id}`.
- Job handler: `extract_subtitles`.

### artwork
Posters, backdrops, episode thumbs (TMDB stills, `owner_kind='episode'` `kind='thumb'`),
album covers and avatars: upload + storage under `DATA_DIR/artwork`, on-demand resizing
via ffmpeg (`?size=w342|w780`) with an mtime-keyed cache, and TMDB/embedded-cover
ingestion through `artwork.Service`.
- Endpoints: `/artwork/{id}`; admin `POST /admin/artwork`, `DELETE /admin/artwork/{id}`.

### jobs
The Postgres-backed job queue (no Redis): enqueue/claim with `FOR UPDATE SKIP LOCKED`,
per-type concurrency slots, retries with backoff, progress reporting, admin
cancellation, and the in-process worker runner. Other features register handlers in
`cmd/couchverse/main.go`; the hourly `cleanup` handler lives in `cmd/` too.
- Endpoints (admin): `/admin/jobs` (supports `?mediaFileId=` and returns a `subject`
  per job - the title/episode/track it works on, resolved by SQL joins from the
  payload), `/admin/jobs/{id}/retry|cancel`.
- Frontend: `features/jobs` (AdminJobsPage, MediaFileJobs, job-label helpers,
  overview/storage/system/analytics api calls).

### system
Server-level concerns: the public accent theme endpoint, admin settings KV editing
(with transcode-settings validation), the feature-flags endpoint, storage stats,
catalog overview counts, live host metrics (CPU/RAM/disk, platform-specific, with
per-process attribution to the Go app and ffmpeg children via /proc) and the
home-rows editor.
- Endpoints: `/theme` (public), `/features`; admin `/admin/settings`, `/admin/storage`
  (categories movies/series/music/transcodes/cache - transcodes is the SQL sum of
  ready variant sizes, cache covers images/uploads/JIT session scratch),
  `/admin/overview`, `/admin/system`, `/admin/home-rows`.
- Frontend: `features/settings` (AdminSettingsPage, HomeRowsEditor, feature-flags store),
  `features/admin` (AdminDashboardPage, AdminSidebar, meters/sparkline/BarChart widgets).

### analytics
Watch/listen time measurement behind the admin overview charts. One daily rollup
table (`watch_time_daily`), upserted on every video progress beacon (the player sends
an actually-played `watchedSeconds` delta) and on every music scrobble (counted as
the track duration). Kernel-only imports - catalog and music call `RecordWatch`/
`RecordListen` on its Store, best effort (analytics never fails a beacon).
- Endpoints (admin): `/admin/analytics/overview?days=N` - dense daily series
  (video/music seconds, active users), totals, top titles, top users.
- Frontend: charts on AdminDashboardPage (`features/admin`), api call in
  `features/jobs/api.ts` next to the other overview endpoints.

## Internationalization & multi-language media (cross-cutting)

**UI + metadata language (one "display language", Czech + English).** The frontend
uses **Paraglide JS** as a compile-only i18n: messages in `frontend/messages/{en,cs}.json`,
compiled to `src/lib/paraglide/` (gitignored, built by the Vite plugin and the `check`
script). `lib/i18n/locale.svelte.ts` is the single source of truth (`currentLang`,
`setDisplayLang`, `applySavedLang`); the header `LanguageSwitcher` persists the choice to
`/me/preferences` and `setLocale` reloads. `lib/api/client.ts` appends `?lang=` to every
request; the backend honours it only on public catalog reads. Per-title TMDB metadata is
stored per language in `translations jsonb` columns on `titles/seasons/episodes/genres`
with the base columns as the fallback; `titles.metadata_languages` is the per-title content
set (chosen via `LanguageChips` in NewTitleModal/editor). `metadata/tmdb.go` takes a `lang`
param and the fetch/import jobs loop over the title's languages. Resolution: `httpx.Lang`
+ `httpx.WithLang` (set by a `withLang` route wrapper on public reads) + `catalog.localize`
overwrite name/overview at scan; admin reads and jobs leave it empty so they see base text.

**Multi-language audio (two models, both supported).**
- *Model B - separate file per language:* `media_files.audio_lang` + `audio_role`
  (`primary`/`audio_alt`); `PrimaryMediaFileForTitle/ForEpisode` prefer the primary,
  `AudioSiblings` returns the alternates. Tagged via `PATCH /admin/media-files/{id}`
  (AudioLangControl in the editor). The player swaps the source file and re-seeks.
- *Model A - one file, many embedded tracks:* ffprobe records all tracks into the
  `audio_streams` table (prober `ReplaceAudioStreams`); a multi-audio h264 file is remuxed
  to a single `multiaudio` HLS variant via ffmpeg `-var_stream_map` (copied video + AAC
  audio renditions, master.m3u8) and is forced onto HLS. The player switches via hls.js
  `audioTrack` (Safari: native `video.audioTracks`).
- Both surface as `playbackInfo.audio` (source `file`|`embedded`); the player shows one
  audio menu, selected independently of the display language (`localStorage cv.audioLang`).

## Backend dependency graph

A feature may import another feature's `Store` or exported services, never its
handlers. SQL may JOIN any table (joins create no Go dependency). The import graph
must stay acyclic:

```
artwork <- auth <- music <- catalog <- metadata
   ^        ^       ^         ^
   +--------+-------+---- library <- subtitles <- playback        system
analytics <- music, catalog (leaf: imports kernel only)
            (anything may import the kernel: jobs, settings, media,
             flags, db, httpx, slug, config)
```

Notes that keep it acyclic:
- `media_files`/`subtitles` row types live in the `media` kernel; library/subtitles own
  the SQL, catalog and playback read the rows.
- Transcode variants SQL lives in library (the prober writes variant rows at probe
  time); the transcode ladder/settings policy lives in the `media` kernel.
- `home_rows` is touched by two features (catalog reads, system edits) - SQL-only
  overlap, intentional.

## Adding a feature (recipe)

Backend:
1. Create `backend/internal/feature/<name>/` with `store.go` (`type Store struct { db *pgxpool.Pool }`,
   `NewStore`), handler files, and `routes.go` exposing `Mount*`/`Module` methods.
2. Add migrations in `backend/migrations/` (next `NNNN_` prefix; goose runs them at boot).
3. Construct the store in `cmd/couchverse/main.go`, pass it to `server.New`, mount the
   routes in `internal/server/server.go` next to the other features.
4. Register background job handlers (if any) on the runner in `main.go`.

Frontend:
1. Create `frontend/src/lib/features/<name>/` with `api.ts` (all endpoint calls),
   optional `types.ts` and `*.svelte.ts` rune stores, `components/`, `pages/XxxPage.svelte`.
2. Add a thin route shell in `src/routes/...` that renders the page component
   (loaders in `+page.ts` stay in routes/ and delegate to the feature's `api.ts`).

Verify: `make lint check test build`, then `make run-backend` + `make run-frontend`.
