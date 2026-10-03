# Couchverse features

Reference for AI-agent development: every feature, where its code lives on each side
of the stack, what it talks to, and how the pieces depend on each other.

Architecture rule: **a feature owns its HTTP handlers, domain logic and SQL together.**
Backend features live in `backend/internal/feature/<name>/` (one Go package each, with a
per-feature `Store` over the shared pgx pool and a `Register(httpx.Routes)` method that adds
its typed operations to the shared API - see "API contract" below).
Web features live in `clients/web/src/lib/features/<name>/` (api.ts, types, rune state
in `*.svelte.ts`, `components/`, `pages/`). Route files in `src/routes/` are thin shells
that render a `XxxPage.svelte` component from the owning feature.

## Feature index

| Feature | Backend package | Web module(s) | DB tables |
|---|---|---|---|
| auth | `internal/feature/auth` | `features/auth`, `features/users`, `features/preferences` | `users` (incl. `bio`), `sessions` |
| catalog | `internal/feature/catalog` | `features/catalog` | `titles`, `seasons`, `episodes`, `genres`, `title_genres`, `watch_progress`, `watchlist` |
| playback | `internal/feature/playback` | `features/playback` | (reads `media_files`, `transcode_variants`) |
| library | `internal/feature/library` | `features/library`, `features/uploads` | `libraries`, `media_files`, `upload_sessions`, `transcode_variants` |
| metadata | `internal/feature/metadata` | (admin UI in `features/library`) | (writes catalog tables) |
| subtitles | `internal/feature/subtitles` | (player UI in `features/playback`/`preferences`) | `subtitles` |
| artwork | `internal/feature/artwork` | (via `catalog/api.artworkUrl`) | `artwork` |
| jobs | `internal/feature/jobs` | `features/jobs` | `jobs` |
| system | `internal/feature/system` | `features/settings`, `features/admin` | `settings`, `home_rows` |
| analytics | `internal/feature/analytics` | (charts in `features/admin`) | `watch_time_daily`, `watch_time_hourly`, `couch_watch_time_daily` |
| couch | `internal/feature/couch` | `features/couch` | (in-memory; only `couch_watch_time_daily` via analytics) |
| ranks | `internal/feature/ranks` | `features/ranks` | `user_achievements`, `user_counters` |
| downloads | `internal/feature/downloads` | (native clients only) | `download_files`, `downloads` |

Shared kernel (backend): `internal/app` (wiring of stores, services and the job runner),
`internal/config` (env), `internal/db` (pool, migrations, `ErrNotFound`), `internal/httpx`
(the huma API, error envelope, `Routes` groups, `Tag`/`Localized`/`Guard`/`Raw`), `internal/media`
(ffprobe, codec compatibility, filename parsing, transcode ladder and auto-prepare policy,
shared `MediaFile`/`Subtitle` row types; `media/mp4` reads fMP4 init sections and segments),
`internal/hls` (HLS playlists: writer, parser, WebVTT segmenting and the validator behind
`couchverse validate-hls`), `internal/settings` (settings KV store),
`internal/flags` (admin-toggleable feature flags), `internal/grant` (media and artwork
grants), `internal/slug`, `internal/version`
(build version, API level), `internal/server` (composition root: middleware, API groups,
feature registration, SPA fallback).

Shared web: `lib/api/client.ts` (fetch wrapper - never hand-write URLs in
components) and `lib/api/cache.svelte.ts` (SWR cache), `lib/components/ui/` (bits-ui
primitives), `lib/components/layout/` (TopNav, GlowBackdrop, NavProgress),
`lib/components/{CachedView,StreamedView,NotFound}.svelte` (optimistic page shells),
`lib/theme.ts` (accent), `lib/utils/`, `lib/tv/` (TV mode), `lib/core/` (the shared core
as wasm). See "Optimistic navigation & caching", "TV mode" and "Shared core on the web".

Native clients run the shared core ("Shared client core"); the iPhone, iPad and Apple TV apps
are described in "Apple clients".

## Features

### auth
Sign-in for browsers (cookie sessions, CSRF origin check, login rate limiting) and native
clients (device sessions with bearer tokens, pairing, connect codes), the `/auth/me`
identity endpoint, the devices list, user profiles (display name, avatar, banner,
markdown bio), per-user preferences (subtitle appearance), and the admin user CRUD.
Bootstraps the master admin account on a fresh database.
- Backend: `Register` adds public (`POST /auth/login|logout|token|connect`,
  `/auth/pairings...`), user (`/auth/me`, `/me/profile`, `/me/preferences`, `/me/avatar`,
  `/me/banner`, `/me/devices`, `/me/pairings/{code}...`, `/me/connect-codes`,
  `/me/artwork-grant`) and admin (`/admin/users...`) operations. Session middleware (`Load`,
  `UserFrom`, `SessionFrom`) and the `SignedIn`/`Admin` guards live here.
- **Sessions** (`sessions` table): one row per signed-in browser or app, with a public
  `id`, `kind` (`browser`|`device`), `device_name` and `platform`. Browsers present the
  cookie, apps `Authorization: Bearer`; only the token's SHA-256 is stored. Expiry
  **slides**: a use extends it to 30 days again (written at most daily, and the browser
  cookie is reissued then), so active members are never signed out by a fixed lifetime.
  `GET /me/devices` lists them (browsers named from the user agent) and
  `DELETE /me/devices/{id}` revokes one; logout ends whichever session made the request;
  disabling a user deletes all of theirs.
- **Pairing** (RFC 8628 style, `device_pairings`) for TVs without a keyboard:
  `POST /auth/pairings` returns a secret device code and an `XXXX-XXXX` user code
  (consonants only) plus `verifyPath` for a QR; a signed-in user approves or denies it
  under `/me/pairings/{code}` (optionally renaming the device); the device polls
  `POST /auth/pairings/poll` (429 `slow_down` faster than the interval) and gets its token
  exactly once. **Connect codes** (`connect_codes`) are the reverse: the web shows a one-time
  code as a QR (`couchverse://connect?server=...&code=...`) and a phone redeems it at
  `POST /auth/connect`. Unauthenticated starts and redemptions are rate limited per IP;
  the hourly cleanup sweeps expired rows.
- The **banner** is an ordinary `artwork` row (`owner_kind='user'`, `kind='banner'`),
  so it inherits resizing, caching and accent extraction; the profile hero tints
  itself from `artwork.accent` and only falls back to the rank tier colour when there
  is no banner. Avatar and banner share one `setImage`/`deleteImage` pair.
- The **bio** is markdown in `users.bio` (2000 chars), stored as authored and rendered
  client-side. The security boundary is the core's `markdown` module (the `markdown` surface):
  it parses into a document tree that can only express safe structure (raw HTML arrives as
  text, images as links, links are http(s) or mailto only). On the web
  `lib/components/ui/Markdown.svelte` turns that tree into Svelte elements
  (`MarkdownBlocks`/`MarkdownInlines`, no `{@html}`), and every link gets
  `target="_blank" rel="nofollow noopener noreferrer"`.
- **Preferences** are a jsonb blob served through a typed shape (`Preferences`:
  `subtitles`, `language`, `publicProfile`). `PUT /me/preferences` merges the posted
  keys, and object values (`subtitles`) merge field by field, so keys the server does
  not model stay stored across a client's read-modify-write; they are just not
  served. A stored value of the wrong type reads as unset.
- Web: `features/auth` (`session`, a thin view of the core's session view, plus the 401
  handler, LoginPage, ProfilePage + the Edit-profile and Change-password modals),
  `features/users` (AdminUsersPage), `features/preferences` (the web-only subtitle style and
  the privacy switch; the display language is the core's). `/profile` renders the *same*
  `ProfileContent` as `/u/you` with owner affordances, reading the same cache entry -
  there is no separate account page and no tabs. With rankings off it falls back to
  `AccountOnlyProfile`.
- Web, devices: `/pair` (`PairPage`) approves or denies a pairing code, typed into the
  segmented `PairCodeInput` (one real input under the cells) or prefilled from the
  device's QR (`?code=`); signed-out visitors go through `/login?next=` and back. Both
  profile variants end with `DevicesSection` (`ProfileContent` takes it as its owner-only
  `children`): the account's sessions from `devicesCache`, revoke behind a confirm (this
  browser's own row is a plain logout, which also clears the cookie), and
  `ConnectDeviceModal`, which shows a one-time connect code as a QR (`ui/QrCode.svelte`
  over `uqr`), replaces it when it expires and closes itself once a new device shows up
  in the list.

### catalog
The watchable catalog: movies and series with seasons/episodes and genres, the home
page (featured title, admin-curated rows, continue watching), browse with filters,
full-text search, per-user watch progress and My List (watchlist). Admin side: the
library table, title/season/episode CRUD and bulk actions.
- Endpoints: `/home`, `/titles`, `/titles/{slug}`, `/search`, `/genres`, `/progress`,
  `/me/continue-watching`, `/me/watchlist...`; admin `/admin/library`, `/admin/titles...`
  (incl. `DELETE /admin/titles/{id}/languages/{lang}` to drop a content language and
  `GET /admin/titles/{id}/storage` for the per-title disk-usage breakdown),
  `/admin/seasons/{id}...`, `/admin/episodes/{id}`.
- **Title logos**: featured items (`GET /home`) and title detail (`GET /titles/{slug}`)
  embed `TitleLogo` - `logoId`, `logoVer` (the `v` token) and `logoAspect` (width / height,
  so a client lays the hero out before the PNG loads), all absent without a logo. The logo is
  picked for the request's display language (`httpx.LangFrom`), else the title's base
  language (`metadataLanguages[0]`), else a language-neutral one, else any (`PickLogo`). Title
  detail's `artwork` list still carries every logo, with its `lang`. The core's catalog turns
  them into `Logo { url, aspect }`; the home hero and the title page show the logo in place
  of the name (kept as screen-reader text in the `h1`), and fall back to the name when it
  does not load.
- Web pages: HomePage, MoviesPage, SeriesPage, GenresPage, GenrePage, MyListPage,
  SearchPage, TitleDetailPage; components HeroMarquee, MediaRow, PosterCard, TitleCard,
  ContinueWatchingCard, BrowseGrid (infinite scroll), Artwork, LoadFailed. The viewer
  pages read the core's catalog view models (`HomeView`, `TitleView`, `BrowseView`,
  `GenresView`, `MyListView`, `SearchView` in `lib/generated/core.ts`; see "Shared core on
  the web"), with image URLs, play/resume decisions, playable seasons only and quality
  ready-made; `features/catalog/labels.ts` localizes the codes they carry (kinds, quality,
  built-in home row labels). `features/catalog/types.ts` keeps only what the admin shares
  (`Genre`, `ContentStatus`); `artworkUrl` remains for the admin, couch and profile images.
  The home hero is accented from its backdrop's accent (`lib/theme.accentVars`) and the
  title page from the palette the core derives (`TitleDetailView.accent`,
  `lib/theme.paletteVars`), both with a contrast-aware `--color-on-accent`. Episode rows
  show stills, progress and the file's length (`EpisodeView.durationSeconds`).

### playback
Everything that turns a media file into pixels: the per-device playback decision, direct
play streaming with range requests, the prepared HLS v2 renditions and their multivariant
playlists, JIT ("instant play") sessions with seek-anywhere, the background transcode job
engine (ffmpeg, hardware encoder detection/probing) and transcode admin. Full design in
"Playback v2" below.
- Endpoints: `POST /playback/{kind}/{id}` (`resolvePlayback`: the body is the device's
  capability profile; the payload carries the tier, the media grant and grant URLs; its
  `display.backdropId` accents the player), `GET /playback/{kind}/{id}` (`getPlayback`, the
  browser baseline plus `?caps=`, kept for older clients) and, under `/media/{grant}/` (see
  "Media grants"): `stream`, `frame?t=` (seek-preview still, ffmpeg input-seek cached under
  `cache/frames`, tone-mapped for HDR), `hls/master.m3u8?video=original|ladder|legacy&surround=`,
  `hls/{variant}/{file}`, `hls/subtitles/{id}/{file}` (sidecar subtitles as WebVTT renditions),
  `jit` (open an instant-play session), `jit/{sid}/{file}`, `jit/{sid}/keepalive`,
  `DELETE jit/{sid}` (stop); admin `/admin/transcode/info|active`,
  `/admin/media-files/{id}/transcode|variants`, `/admin/transcode-variants/{id}`.
- Job handler: `transcode_hls` (per-type concurrency = `maxConcurrent` setting) with the
  variants `package` (the copied source and every audio rendition in one read), `trickplay`
  and a ladder rendition name.
- Web: WatchPage + VideoPlayer (HLS.js, subtitles, shortcuts, progress beacons incl.
  watched-seconds deltas, JIT keepalive, banner-accented chrome, bits-ui control tooltips,
  seek-bar time + frame preview - splitting it is a known follow-up). `deviceProfile()` in
  `playback/api.ts` measures the browser once per page load.
- Transcode ladder/settings policy and the auto-prepare policy (`media.AutoPrepare`) live in
  the `media` kernel and `library.Prepare` queues the jobs, so library's prober can prepare
  renditions without importing playback. Rendition bitrates are capped at the source bitrate
  (`Rendition.CappedAt`) so transcodes never outweigh their source; variant sizes are measured
  into `transcode_variants.size_bytes` when a job finishes.

### library
Media ingestion via resumable chunked uploads -> the probe pipeline (filename parsing
to draft titles/episodes, direct-play detection,
auto-prepare of HLS variants), the managed libraries uploads land in, and ownership of the
`media_files` + `transcode_variants` SQL that playback reads. (There is no folder-scan
ingestion - everything comes in through the browser; the `libraries` rows are managed
upload targets, auto-created on first boot.)
- Endpoints (admin): `/admin/media-files/{id}` (`PATCH` audio lang/role; `DELETE`
  hard-deletes the file plus its source, HLS/frame caches and subtitle files on disk),
  `/admin/uploads...`.
- Job handlers: `probe`, and `reprobe` (enqueued at every start; brings rows an older prober
  read up to `media.ProbeVersion`, from ffprobe or, once the source is gone, from the stored
  probe output, and queues the cheap HLS v2 package for them when auto-prepare is on).
- The probe records, besides container, codecs, size, duration and bit rate, the video's codec
  tag, normalized profile, level, bit depth, frame rate and HDR format (`hdr10`, `hdr10plus`
  from the first frame's SMPTE 2094-40 data, `hlg`, `dolbyVision` with profile and base-layer
  compatibility) on `media_files`, and per audio track the profile (Atmos), channel layout and
  sample rate on `audio_streams`.
- Web: `features/library` (AdminLibraryPage, AdminTitleEditorPage,
  editor components incl. EditorHero with hover poster/backdrop editing, EpisodesTable with
  client-side filters, StorageChart, NewTitleModal, TmdbSearchModal, FileVariants,
  LanguageChips + AddLanguageModal, LogosPanel with a logo slot per content language to
  upload, replace or remove), `features/uploads` (upload queue
  store, AdminUploadsPage, EditorUploadCard).

### metadata
TMDB integration: search, one-click apply of metadata + poster/backdrop/logos to a title,
and bulk import of missing seasons/episodes for a series (episode stills are downloaded
as episode `thumb` artwork). API key comes from settings.
- Endpoints (admin): `/admin/metadata/search`, `/admin/titles/{id}/metadata/apply`,
  `/admin/titles/{id}/metadata/seasons`, `/admin/titles/{id}/metadata/import-episodes`.
- Job handlers: `fetch_metadata`, `import_episodes`.
- **Logos** (`metadata/logos.go`): one `GET /{movie|tv}/{id}/images` call with
  `include_image_language=<content languages>,null`; for each content language the
  best-voted PNG logo in that language (vote average, then vote count), else the best-voted
  language-neutral PNG, is stored as the title's logo in that language (SVGs are skipped; a
  legacy title without content languages asks for English and stores a null-language logo;
  languages sharing the neutral fallback share one download). Apply replaces them and drops
  TMDB logos it did not store (a re-link to another TMDB entry leaves none of the old one's
  behind) while uploaded logos are only ever overwritten; a failure fails the job like a
  poster's. The episode import only fills languages without a logo, best effort, so shows
  linked before logos existed get them from a re-import.

### subtitles
Side-car WebVTT subtitles: automatic extraction of embedded text subs (ffmpeg),
`.srt`/`.vtt` upload with conversion, serving as `<track>` elements (the web) and, through
playback's `hls/subtitles/{id}/...`, as segmented WebVTT renditions of every HLS playlist
(system players).
- Endpoints: `/subtitles/{id}.vtt`; admin `/admin/media-files/{id}/subtitles`,
  `/admin/subtitles/{id}`.
- Job handler: `extract_subtitles`.

### artwork
Posters, backdrops, title logos, episode thumbs (TMDB stills, `owner_kind='episode'`
`kind='thumb'`), avatars and profile banners: upload + storage under `DATA_DIR/artwork`,
on-demand resizing via ffmpeg (`?size=w342|w780`) with an mtime-keyed cache, and TMDB
ingestion through `artwork.Service`. Every save measures the image (`width`/`height`, 0 for
WebP, which the stdlib cannot decode) and extracts its accent (transparent pixels are
skipped and edge pixels un-premultiplied, so a logo accents to its own colour).
- Endpoints: `/artwork/{id}`; admin `POST /admin/artwork` (multipart: `ownerKind`,
  `ownerId`, `kind`, optional `lang`, `file`), `DELETE /admin/artwork/{id}`.
- **Slots** are `(owner_kind, owner_id, kind, lang)` (`UNIQUE NULLS NOT DISTINCT`): `lang` is
  an ISO 639-1 code for language-bound art and null for the rest, so a title holds one poster
  and one backdrop but one **logo** per content language. Original files are named
  `<kind>.<ext>`, or `<kind>-<lang>.<ext>` for a language-bound slot.
- **Logos** (`kind='logo'`, titles only) are transparent PNG wordmarks for cinematic heroes:
  uploads must be `.png` (TMDB's SVG logos are never stored), and their resizes are cached as
  PNG instead of JPEG so the alpha channel survives. `artwork.PickLogo` chooses which one a
  viewer sees (see catalog). Dropping a content language deletes that language's logo
  (`Service.DeleteForLang`).

### jobs
The Postgres-backed job queue (no Redis): enqueue/claim with `FOR UPDATE SKIP LOCKED`,
per-type concurrency slots, retries with backoff, progress reporting, admin
cancellation, and the in-process worker runner. Other features register handlers in
`cmd/couchverse/main.go`; the hourly `cleanup` handler lives in `cmd/` too.
- Endpoints (admin): `/admin/jobs` (supports `?mediaFileId=` and returns a `subject`
  per job - the title/episode/track it works on, resolved by SQL joins from the
  payload), `/admin/jobs/{id}/retry|cancel`.
- Web: `features/jobs` (AdminJobsPage, MediaFileJobs, job-label helpers,
  overview/storage/system/analytics api calls).

### system
Server-level concerns: the public accent theme endpoint, admin settings editing (typed
per settings key: `tmdb.api_key`, `transcode` (validated), `features`, `home`,
`appearance`; other features' keys such as `ranks` go through their own endpoints),
the feature-flags endpoint, storage stats,
catalog overview counts + library insights (total video runtime, resolution/HDR mix,
titles added in the last 30 days), live host metrics (CPU/RAM/disk, platform-specific,
with per-process attribution to the Go app and ffmpeg children via /proc), a
live-presence endpoint and the home-rows editor.
- Endpoints: `/server` (public identity: a stable id created at first boot, the
  admin-editable `server.name`, the build version and the API level clients compare
  against; adding a server in an app starts here), `/theme` (public), `/features`; admin `/admin/settings`, `/admin/storage`
  (categories movies/series/transcodes/cache - transcodes is the SQL sum of
  ready variant sizes, cache covers images/uploads/JIT session scratch),
  `/admin/overview` (counts + `library` insights), `/admin/system`, `/admin/live`,
  `/admin/home-rows`.
- **`/admin/live`** (polled every 3s by the dashboard) reports current streams
  (distinct `watch_progress` rows beaconed in the last 60s), live couch sessions +
  people on them, and active instant-play transcodes. It reads in-memory state through
  small interfaces (`CouchPresence`/`TranscodePresence`) satisfied by `couch.Hub` /
  `playback.SessionManager` and wired at the composition root, so `system` imports
  neither package.
- Web: `features/settings` (AdminSettingsPage, HomeRowsEditor, `features`: the flags as the
  core's session reads them; a save sends `SessionChanged` so the core reads them again),
  `features/admin` (AdminDashboardPage, AdminSidebar, meters/sparkline/BarChart widgets).

### analytics
Watch time measurement behind the admin overview charts. One daily rollup table
(`watch_time_daily`), upserted on every video progress beacon (the player sends an
actually-played `watchedSeconds` delta). A second **hour-of-day rollup**
(`watch_time_hourly`, at most 48 rows per user per day) is written from *inside* the
same `RecordWatch` call, so no caller changed; it is bucketed with `AT TIME ZONE` in the app's local
zone (Postgres runs UTC) so "night" means night, feeds the profile clock and the
night-owl/early-bird achievements, and is pruned past 400 days by the hourly
`cleanup` job. Kernel-only imports - catalog calls `RecordWatch` on its Store, best
effort (analytics never fails a beacon). The
separate **on-couch watch-time** stat lives here too: `RecordCouchWatch(titleId,
seconds)` upserts the `couch_watch_time_daily` per-title rollup (no user
dimension, so anonymous followers count), called best-effort by the couch hub.
- Endpoints (admin): `/admin/analytics/overview?days=N` - dense daily series
  (video/couch seconds, active users), totals, top titles, top couch
  titles, top users (each with `avatarId` for the dashboard leaderboard).
- Web: charts + "Top viewers" and "On Couch watch-time" cards on
  AdminDashboardPage (`features/admin`), api call in `features/jobs/api.ts` next to
  the other overview endpoints.

### couch
Spotify-jam-style synced watch parties. A logged-in host watching a movie/episode
starts a session and shares an unguessable link `/couch/{token}`; friends join
**logged-in or fully anonymous** and land in the existing player in follower mode,
synced to the host (host controls play/pause/seek/episode; followers have no
timeline control, manage their own audio/subtitles, send emoji). All session and
participant state is **in-memory** - a server restart ends every session; the only
persisted artifact is the on-couch watch-time stat (in analytics). Gated by the
admin `couchEnabled` flag (default on, mirrors `rankingsEnabled`).
- Backend (`internal/feature/couch`): an in-process **Hub** (session registry +
  rooms), a scoped httpOnly **couch cookie** (mirrors the auth session cookie), the
  HTTP handlers and the WS endpoint, on `github.com/coder/websocket`. `Hub.LivePresence()`
  exposes live session/viewer counts to the admin dashboard's `/admin/live`.
- Endpoints: `POST /couch` (create/reclaim, host must be logged in),
  `POST /couch/{token}/join` (public, anon OK; `?remote=true` for the host's own account
  on another device, 403 `not_host` for anyone else), `POST /couch/{token}/leave`,
  `POST /couch/{token}/end` (any of the host's devices), `GET /couch/{token}/playback`
  (follower payload for the current media), `GET /couch/{token}/ws` (the sync socket;
  same-origin enforced for browsers).
- **Participant tokens**: create and join mint a token per device (a participant may hold
  several, so a host joining from a phone does not sign out their TV). Browsers get it as the
  couch cookie; native apps pass `?delivery=body`, receive `participantToken` and send it as
  `X-Couch-Token` wherever a browser sends the cookie (socket, playback, leave, end).
- **Remote control**: a remote connection (the host's account joined with `remote=true`)
  receives `host_state` like a follower but plays nothing; its `remote_command` frames
  (`play|pause|seek|next|previous`, `positionSeconds` for seek) go to the host's playing
  connection, the one that last sent `host_state`, which applies them and broadcasts the
  result. A remote leaving keeps the session; a follower or anyone sending it is refused.
- WS protocol: host broadcasts authoritative `{media, playing, positionSeconds,
  serverTimestamp, seq}` (server-stamped) + emoji; followers extrapolate position
  from the last update + local elapsed and hard-seek past ~3s drift. Host identity
  is tied to the user (multi-tab reclaim, 60s reconnect grace).
- **Stream authorization (cross-feature):** followers (incl. anonymous ones) reach only
  the host's current media. `GET /couch/{token}/playback` builds their payload with
  `playback.BuildPlayback` and a `playback.Viewer` naming their participant, so every
  media grant in it is bound to that participant. `playback` must not import `couch`,
  so the check lives in the composition root: `internal/server`'s media-grant
  middleware asks the Hub's `AllowsMedia(participantID, mediaFileID)` on every request
  carrying such a grant, which revokes it the moment the follower leaves, the host
  switches media or the session ends. The couch payloads also carry an artwork grant
  (`artworkGrant`) so anonymous guests can load avatars and backdrops.
- Web (`features/couch`): a singleton rune store (WebSocket + follower sync +
  host broadcast), a public `/couch/[token]` route that
  joins anonymous viewers without tripping the 401 redirect, the assembled
  accent-recoloured `Couch` (seated avatars + host remote), a management popover and
  couch buttons in the player control bar + TopNav, and a bundled (no-CDN) emoji
  picker. `VideoPlayer` composes follower/host modes via the store at its seams; it
  is not forked.

### ranks
Player progression: XP, rank tiers, achievements, public profiles and the global
leaderboard. Gated by the admin `rankingsEnabled` flag (default on, mirrors
`couchEnabled`).
- **XP** defaults to `2/min` video + `100` per finished movie + `20` per finished
  episode + `50`/`25` per couch session hosted/joined + the achievement rewards
  (bronze 50, silver 150, gold 400, platinum 1000). Couch pays per session because
  on-couch watch-time has no user dimension. XP counts all recorded activity
  **regardless of flags** - turning couch off must never demote anyone. Every term is one explainable line of the profile's XP breakdown.
- **Every rate and threshold is admin-tunable** (`ranks.RankConfig` in the `ranks`
  settings key, defaults in `DefaultConfig`, edited on `/admin/ranks`). `RankConfig` is
  threaded through `ComputeXP`/`TierFor`/`ProgressFor` rather than read globally, so
  the pure functions stay testable. `Validate` rejects negative rates and a ladder
  that does not start at 0 or does not strictly ascend. Because XP is always derived,
  a saved change re-levels everyone on their next read - nothing is recomputed or
  migrated.
- **Tiers** (10, couch-themed, `ranks.defaultTiers`): rookie 0, remote 500, snack 1500,
  binger 3500, popcorn 7000, marathoner 13000, sage 23000, cinephile 40000,
  master 70000, legend 120000. Codes, levels and colours are fixed identity (persisted
  in payloads, translated by the clients); only the thresholds are configurable.
- **Achievements** (30) are a pure rules table in `achievements.go`: each is a
  `Target` plus a `Value(Snapshot)`, scored against one `Snapshot` that a single
  batch of SQL fills, so the whole catalogue is unit-testable without a database.
  `Evaluate` runs **two passes** - non-meta rules first, then the meta rules against
  the resulting count - so the badge that rewards ten badges unlocks in the same
  call as the tenth. Rules gated on `couch` are **absent** (not locked) when
  their flag is off, so no unreachable card is ever shown; already-earned hidden
  ones still count toward XP so totals cannot drift. A test asserts meta targets stay
  reachable on a flag-disabled install.
- **Evaluation is pull-based**: `POST /me/achievements/check` is the *only* writer of
  `user_achievements`, so **no SQL is added to the 10s progress beacon** and catalog
  needs no changes at all. It is throttled per user to
  one run per 30s (a mutex-guarded map on the module, bounded by the account count),
  and a throttled call returns `rank: null` so the client keeps the rank it has
  rather than painting a zero. `INSERT ... ON CONFLICT DO NOTHING RETURNING` yields
  exactly the rows actually inserted, so two tabs racing produce one celebration
  between them with no read-then-write window. The client calls it on app load, every
  5 minutes of playback and on playback end.
- **Privacy**: the `publicProfile` boolean in the existing `users.preferences` jsonb
  (absent means public), written through `PUT /me/preferences` - **zero new write
  surface**. Reads filter with `preferences -> 'publicProfile' IS DISTINCT FROM
  'false'::jsonb`, not a `::boolean` cast, which would throw and take the whole
  leaderboard down if any client ever wrote a non-boolean there. An opted-out member
  is a **404, not a 403** (admins included), so "private" is indistinguishable from
  "no such member"; the owner always sees their own.
- Endpoints (all `RequireAuth` + `flags.RequireRankings`): `GET /me/stats` (the only
  progression fetch on app load - it feeds the nav ring *and* the profile page from
  one SWR entry), `GET /users/{username}/profile`, `GET /leaderboard?period=all|week|month`,
  `POST /me/achievements/check`. The leaderboard returns **every metric per row**
  (xp, watch, achievements) unsorted, so the client's metric switcher sorts in
  place with no refetch and `period` is the only cache key; XP is lifetime whatever
  the period, since completions carry no date.
- Admin (`/admin/ranks`, `PUT /admin/ranks/config`) is deliberately **not**
  flag-gated: an admin has to be able to inspect and retune progression in order to
  turn it back on. The overview reports level distribution, achievement rarity
  (rarest first, flag-hidden badges marked), per-member standing including opted-out
  members, and the effective config.
- **Cross-feature:** couch reports the per-user counters nothing else can see
  (`couch_hosted`/`couch_joined`/`couch_party_max`/`couch_emoji`) through the narrow
  `CouchStatsRecorder` interface satisfied by `*ranks.Store`, wired at the
  composition root exactly like `CouchWatchRecorder`. Emoji and party size ride the
  hub's existing 20s accrual tick rather than the socket read path, where a DB round
  trip would stall reads and reaction spam would be unbounded. Counters keep accruing
  even when `rankingsEnabled` is off, so re-enabling does not present an empty
  history. `ranks` imports `catalog` only for the exported `Localize`/`GenreLabel`
  helpers, so profile title names and the favourite-genre label read in the visitor's
  language without restating the translations shape.
- Web (`features/ranks`): `/u/[username]` (hero accented from the member's banner
  via `accentVars`, falling back to the tier colour; markdown bio; 8 stat tiles; a
  53x7 activity heatmap scrolled to today; a 24-slice "when you watch" clock; the XP
  breakdown; most-watched; the achievement grid) and `/leaderboard` (2-1-3 podium in
  1-2-3 DOM order, table, sticky "you are #N"). `/profile` renders the same content
  with edit affordances off the same cache entry. `/admin/ranks` holds the admin
  stats and the XP settings form. The nav avatar gains a rank ring + level chip that
  pulse on level-up. Unlock celebrations use `svelte-sonner` off-player and a bespoke
  overlay **mounted inside the player wrapper** on `/watch` - the root Toaster is a
  fixed root-layout element and would vanish in fullscreen, the same trap `CouchBar`
  documents. Both celebration paths drain the queue in a deferred callback rather
  than inside the effect flush: creating a toast writes sonner's own reactive state,
  and doing that mid-flush corrupts its height bookkeeping.

### downloads
Device-ready MP4s that the native clients keep for offline viewing (plan 8.6).
`POST /me/downloads {kind, id, quality, audio[], profile}` plans the file with the pure
`playback.PlanDownload` (the source video when the profile decodes it, else H.264 at the
quality via the transcode encoder; the requested audio languages in order, copied when the
device takes them and MP4 carries them, else AAC; every subtitle track as `mov_text`;
`+faststart`) and names the plan with `DownloadPlan.Spec()` plus the subtitle ids.
Requests with the same media file and spec share one `download_files` row, so a second
user (or device) asking for the same plan gets the ready file at once; every requester has
a `downloads` row. A model-B sibling file is picked when the first language asked for is
only there. The `prepare_download` job (one slot, `nice`d ffmpeg via `playback.Run`) writes
`DATA_DIR/cache/downloads/<file id>.part`, renames it to `.mp4` when done, reports progress
on the row, and records failures (`requestDownload` again re-queues a failed file).
- Endpoints (signed in): `POST/GET /me/downloads`, `GET/DELETE /me/downloads/{id}`
  (`requestDownload`, `listDownloads`, `getDownload`, `deleteDownload`, all `?lang=` for
  the titles); a ready download carries `url`, the MP4 under a media grant of its file
  (`GET /media/{grant}/downloads/{fileId}`, `fetchDownload`, with ranges so transfers
  resume), and `expiresAt`.
- Retention: the hourly cleanup (`downloads.Sweep`) deletes ready files 72 h after they
  were last requested, failed ones after a day, files nobody has in their list anymore,
  and MP4s on disk without a row. A device keeps its own copy; deleting a download only
  removes it from the server list.
- Gated by the admin `downloadsEnabled` flag (`flags.DownloadsOn` on `requestDownload`, the
  job refuses while it is off; listing, fetching and deleting keep working so devices can
  finish and tidy up). The core reads it as `SessionView.features.downloads` and answers a
  request with the `downloads_disabled` notice while it is off.
- Offline progress: `saveProgress` takes an optional `watchedAt`; a replayed report older
  than the saved position does not replace it.
- Tests: `playback/download_test.go` (plans and ffmpeg arguments), `TestDownloads`
  (real MP4s copied and transcoded, inspected with ffprobe, shared files, range requests,
  grants, the cleanup).

## Internationalization & multi-language media (cross-cutting)

**UI + metadata language (one "display language", Czech + English).** The web client
uses **Paraglide JS** as a compile-only i18n over the shared catalogs in
`contract/i18n/{en,cs}.json` (also compiled into the Apple and Android string catalogs by
`cargo xtask codegen`; counted strings carry CLDR plural variants),
compiled to `src/lib/paraglide/` (gitignored, built by the Vite plugin and the `check`
script). `lib/i18n/locale.svelte.ts` is the single source of truth for Paraglide
(`currentLang`, `setDisplayLang`, `followAccountLang`) and is fed from the core's session:
the header `LanguageSwitcher` (a flag dropdown built on bits-ui, flags from the bundled
`flag-icons` via `lib/components/ui/Flag.svelte` + `lib/i18n/flags.ts`) sends
`DisplayLanguageChanged`, waits for the core to save it to `/me/preferences`, and then
`setLocale` reloads; on load and after sign-in the account's language wins (one reload when it
differs, before anything renders). A first sign-in saves what the visitor was seeing, and
visitors without a session keep Paraglide's localStorage choice (else English).
`lib/api/client.ts` appends `?lang=` to every request; the backend honours it only on public
catalog reads. Per-title TMDB metadata is stored per language in `translations jsonb` columns
on `titles/seasons/episodes/genres` with the base columns as the fallback;
`titles.metadata_languages` is the per-title content set (chosen via `LanguageChips` +
`AddLanguageModal`, which search the bundled `ALL_LANG_CODES` ISO 639-1 list in
`lib/i18n/content-langs.ts`). `metadata/tmdb.go` takes a `lang` param and the fetch/import
jobs loop over the title's languages. Removing a content language
(`DELETE /admin/titles/{id}/languages/{lang}`) is destructive: catalog drops that language's
translations across the title/seasons/episodes and its logo, and promotes the next language
into the base columns when the base one is removed (the last language cannot be removed), and
the editor also deletes that language's alternate-audio files and subtitles from disk via
their own endpoints. Title logos are per content language too (one `artwork` row per
language, see artwork) and are picked by the display language like the text.
Resolution: `httpx.Localized` (an operation modifier that documents `?lang=` and stores it via
`httpx.WithLang`) + `catalog.Localize` overwrite name/overview at scan; admin reads and jobs
leave it empty so they see base text.

**Multi-language audio (two models, both supported).**
- *Model B - separate file per language:* `media_files.audio_lang` + `audio_role`
  (`primary`/`audio_alt`); `PrimaryMediaFileForTitle/ForEpisode` prefer the primary,
  `AudioSiblings` returns the alternates. Tagged via `PATCH /admin/media-files/{id}`
  (AudioLangControl in the editor). The player swaps the source file and re-seeks.
- *Model A - one file, many embedded tracks:* ffprobe records all tracks into the
  `audio_streams` table (prober `ReplaceAudioStreams`); the HLS v2 package prepares AAC
  stereo (and surround) renditions of every track, which every multivariant playlist lists
  as one audio group in stream order (see "Playback v2"), and a client that cannot switch
  the tracks of a progressive file (`audioTrackSwitching` in its profile) plays through
  HLS. The player switches via hls.js `audioTrack` (Safari: native `video.audioTracks`);
  AVPlayer and ExoPlayer show the group as their native audio menu. Files prepared before
  HLS v2 keep their `multiaudio` MPEG-TS remux (`-var_stream_map`, its own master.m3u8).
- Both surface as `playbackInfo.audio` (source `file`|`embedded`); the player shows one
  audio menu, selected independently of the display language (`localStorage cv.audioLang`).

## API contract (cross-cutting)

The HTTP API is described by an OpenAPI 3.1 document generated from the Go handlers
(huma v2 on the chi router), so the spec cannot drift from what the server does, and every
client is generated from it (native clients plan, D30).

- **One API, five groups** (`internal/server`): `Public`, `User` (session required),
  `Admin` (`/admin` prefix, admin role), `Media` (`/media/{grant}`, a valid media grant) and
  `Artwork` (a session or an artwork grant). Feature flags gate their own groups with
  `httpx.Guard`.
- **Operations** are typed (`huma.Register`) with stable camelCase operation ids. The few raw
  routes (byte streams, the couch WebSocket, chunked upload appends, image/VTT files) go through
  `httpx.Raw`, which shares the group's middleware and still documents parameters and content
  types, so generated clients get typed URL builders for them too.
- **Errors** keep the envelope `{"error": {"code", "message"}}`; validation failures are 400
  `bad_request`, malformed path parameters 404, a wrapped `db.ErrNotFound` 404, anything else
  a logged 500 `internal`.
- **Publishing**: `couchverse openapi` and `couchverse couch-schema` print the documents without
  a database; `make contract` writes them to `contract/` and runs `cargo xtask codegen`, which
  renders one parsed model as the core's Rust crate (`core/crates/api`: types, one builder per
  operation returning a `Call<T>` for a shell to execute, couch `ServerFrame`/`ClientFrame`)
  and the web's `src/lib/generated/api.ts` (types plus one function per operation over the
  existing `api()` wrapper, each taking trailing `CallOptions` such as `signal` or
  `skipAuthRedirect`). Response enums tolerate unknown values and frames decode unknown
  types as `Unknown`, so additive server changes never break older clients.
- **Request bodies** are JSON, a multipart form or raw `application/octet-stream` bytes, and
  `requestBody.required: false` makes one optional (`body?: T`, `Option<&T>`). An upload takes a
  `FormData` or a `Blob`/`ArrayBuffer`/`Uint8Array` on the web; in Rust it is still a `Call<T>`,
  whose request carries the marker `Body::Multipart`/`Body::Binary` instead of content, because
  the core never holds file bytes: the shell attaches the file and hands the response to
  `Call::parse`. Defaults, ranges, lengths and a form's parts live in the doc comments.
- **Path builders** (`getArtworkPath`, `adminAppendUploadPath`, ...) exist for raw operations
  and uploads and return the full `/api/v1/...` path, ready for an `<img src>`, a media element
  or a request made without `api()`; call sites never prepend the prefix.
- **Conformance**: `TestAPIConformance` (`internal/server/api_test.go`) builds the real app on
  a throwaway database seeded from `testdata/seed.sql`, runs cases for every typed operation
  (including uploads, captured ids and an anonymous cookie-keeping client) and validates each
  response against the spec; it fails when an operation has no case. The couch protocol has
  golden frames in `contract/fixtures/couch`, asserted by the Go tests and decoded by the
  core's tests.

## Shared client core (cross-cutting)

Every client (the native apps and, slice by slice, the web) runs the same Rust state machine,
`core/crates/app` (`couchverse-core`), so logic is written once (native clients plan, section
7). It is **sans-I/O**: it never touches the network, a clock or storage; shells send it
messages and perform the effects it asks for.

- **The bridge** is four string calls: `new(config)`, `send(message)`, `resolve(resolution)`
  and `view(surface)` (`bridge.rs`), exported by `crates/ffi` (UniFFI: Swift, Kotlin) and
  `crates/wasm` (wasm-bindgen: the web). Every message type carries `#[typeshare]`; `cargo
  xtask codegen` renders them as `Messages.swift` (CouchverseCore package), `Messages.kt`
  (Android `core` module) and `clients/web/src/lib/generated/core.ts`, committed with the
  change. Enums with data are adjacently tagged (`{"type", "content"}`), unit enums are
  strings, ids and milliseconds are `U53`.
- **Messages**: `Message { nowMs, event, wallMs? }` (time is an input, so tests control it;
  `nowMs` is monotonic, `wallMs` the Unix-epoch clock for what depends on the date, such as
  progress replayed after watching offline) and `Resolution { nowMs, id, output }`. `CoreConfig` names the platform and the auth mode:
  `bearer` (native: per-account device tokens in the secure store) or `cookie` (the web: one
  session over the browser cookie, origin-relative URLs).
- **Effects** (`EffectRequest { id, effect }`): `http`, `upload` (a multipart form around a
  file the shell holds; the core only sees the shell's handle), `timer` (one-shot or
  repeating, stopped by `cancelTimer`), `store` and `secureStore` (read/write/delete),
  `player` (load a source, play, pause, seek, select audio or subtitles, stop; the shell
  reports back with `PlayerReported`), `socket` (open resolves `socketOpened`, a
  `socketText` per frame and ends with `socketClosed`; send and close), `download` (a
  background transfer into the app's downloads directory by a stable name: `downloadProgress`
  now and then, ending with `downloadFinished` or `downloadFailed`; starting a name already
  transferring attaches to it; cancel and remove), and `render { surfaces }`, which names the
  view models to re-read. Writes, deletes, cancels and renders are fire-and-forget; a late
  answer for a cancelled or forgotten effect is ignored.
- **View models** are plain data per `Surface` (`app`, `servers`, `accounts`, `signIn`,
  `devices`, `pairingApproval`, `session`, `markdown(source)`, `home`, `browse(key)`,
  `title(slug)`, `genres`, `myList`, `search`, `notices`, `rank`, `profile(username)`,
  `leaderboard(key)`, `profileEditor`, `player`, `couch`, `downloads`), each with a `LoadStatus`
  (`idle|loading|loaded|stale|notFound|failed`: stale beats blank) and a `Problem { code }`
  that shells localize. `AppView.phase` (`starting|welcome|signIn|chooseAccount|ready`) picks
  the root screen; a TV always opens on "Who's watching?".
- **Modules** (`src/modules/`): `servers` (address -> `/server` identity, https then http),
  `accounts` (password, pairing with polling and expiry, connect and pair links, approvals,
  devices, tokens), `session` (user, features, display language, accent; a 401 anywhere signs
  the account out but keeps it), `catalog` (stale-while-revalidate home, listings, titles,
  genres and My List, fresh for 60 s, or 10 s for a visit to home, a title or My List, since
  the viewer's progress and list change on their other devices; search with debounce and supersede, warm-start home per account, deleted
  when that account signs out, image URLs per role with the artwork grant; confirmed My List
  changes are numbered and folded into answers requested before them, so a slow refetch never
  undoes one), `ranks` (rank badge and level-ups, throttled achievement
  checks, celebration queue, profiles with the heatmap, leaderboards), `profile` (edits,
  password, avatar/banner uploads), `playback` (the device profile and `resolvePlayback`,
  sources by tier, resume, watched-time accounting and progress saves, JIT keepalive,
  preparing poll, qualities, tracks, next episode, shuffle; a downloaded title plays from the
  device with source `download` and in-file subtitles), `couch` (the socket protocol,
  reconnects, host broadcast, follower drift sync, remote control, reactions), `downloads`
  (asks the server to prepare an MP4 for the device's profile, polls, fetches it and its
  artwork with the download effect, keeps the account's offline library in the store, plays
  it while `SessionView.offline`, and keeps progress that could not be saved, with the time it
  was watched, until the server answers again; signing out deletes the account's downloads),
  `notices` (transient toasts), plus pure helpers `theme`
  (accent palettes from `contract/design/tokens.json`), `images` and `markdown` (bios as a
  safe tree: no HTML, no images, http(s)/mailto links only). Requests in flight carry a
  generation, so answers for a previous account or display language are dropped.
- **API calls** come from the generated `couchverse-api` crate (`ops::*` returning a
  `Call<T>`); API types never cross the bridge.
- **Tests**: unit tests per module, scenario tests (`src/scenarios/`) that drive whole flows
  through a fake shell with in-memory stores and explicit time, and `tests/bridge.rs` pinning
  the JSON wire format. `make core-test` runs fmt, clippy (pedantic, warnings denied) and all
  tests.
- **Packaging** (build output, gitignored): `make core-apple` (the five-slice
  `CouchverseCoreFFI.xcframework` plus Swift bindings into the CouchverseCore package, see
  "Apple clients"), `make core-android` (per-ABI `libcouchverse_ffi.so`, a
  host library for JVM tests and the Kotlin bindings for the Gradle `core` module; `make
  android-test` runs every JVM test and verifies the screenshots, docs/android.md) and `make
  core-wasm` (the web package in `clients/web/src/lib/core/pkg`, built for size with opt-level
  "z" and `wasm-opt -Oz`, and checked against a 400 KB gzip budget). In CI `core.yml` builds
  the wasm package, `apple.yml` the Apple one with the apps, `android.yml` the Android one with
  the Gradle project.

## Apple clients (cross-cutting)

The iPhone/iPad app and the Apple TV app (`clients/apple/`, build guide in `docs/apple.md`) are
SwiftUI shells over the shared core: they render its view models, perform its effects and own
navigation, nothing else.

- **Layout**: `Couchverse.xcodeproj` (folder-synchronized groups; settings in `Config/*.xcconfig`,
  per-builder team and bundle id in the gitignored `Config/Local.xcconfig`) holds two thin app
  targets and their UI test bundles; the code lives in three local packages. `CouchverseCore`:
  the xcframework, `Generated/Messages.swift`, `CoreRuntime` and the executors (it alone also
  builds for the Mac, for `swift test`). `CouchverseDesign`: tokens, typography, the accent
  environment, components, generated strings. `CouchverseFeatures`: screens by feature folder
  (`Onboarding`, `Accounts`, `Settings`, `Home`) and `CouchverseRoot`, the view both apps show.
- **Runtime**: `CoreRuntime` (`@Observable`, main actor) is the only stateful service. It stamps
  `nowMs` from the continuous clock, runs one executor per effect (`HTTPExecutor` over an
  ephemeral URLSession, `TimerExecutor`, `SocketExecutor` over `URLSessionWebSocketTask`,
  `KeychainStore` with `AfterFirstUnlock` for tokens, `FileStore` in Application Support on iOS
  and `DefaultsStore` on tvOS, whose only guaranteed storage is user defaults), re-reads just the
  surfaces a `Render` names and publishes them as properties, decoded off the main actor (each
  surface keeps the generation it was published at, so a late batch never wins). Until their
  slices land, `upload` effects fail as `httpFailed` (no image picking yet) and `player` commands
  are ignored (nothing starts playback yet). Screens read `core.<surface>` and `core.send(event)`; their own state is
  presentation only (focus, sheets, a field being typed). `CoreRuntime(fixture:)` shows fixed
  view models for previews and snapshots and records what it is sent. UI tests launch with
  `-uiTesting` (in-memory stores: a fresh install every launch).
- **Root**: `AppView.phase` picks the screen: `welcome`/`signIn` -> `OnboardingFlow` (welcome,
  add server, sign in), `chooseAccount` -> "Who's watching?", `ready` -> `MainTabs` (a sidebar
  on TV and iPad, a Liquid Glass tab bar on iPhone; Home is a placeholder until the catalog
  slice). The root also follows the session's display language (`L10n.language`, observable, so
  strings switch without rebuilding the app) and accent, sends `appBecameActive`, and opens
  `couchverse://connect` and `couchverse://pair` links (`DeepLink` decides what to present; the
  core parses the link).
- **Sign-in**: password, or "Sign in with another device" (the code large, a QR code of the
  pairing page and a countdown from `expiresAtMs` on the runtime's clock). A TV shows both side
  by side and starts pairing on its own; a phone shows one at a time. Approving another device
  (Settings, or a link) takes the code typed, from a pair link, or scanned from the TV's QR code
  (VisionKit on iPhone; the core opens a server's `/pair?code=` page as a pair link).
- **"Who's watching?"** (plan 12.3): glass profile tiles with an identicon-hued ring (the web's
  minidenticons, ported in `Identicon`), the backdrop tinted by the focused profile. On TV,
  choosing one runs `ProfileChoreography`: the others blow outward and the backdrop goes dark,
  the chosen avatar flies into the sidebar header (its frame reported by `ProfileAvatar`) while
  home rises from black behind it; with Reduce Motion it is a cross-fade. Phones use a sheet
  (`AccountSwitcherSheet`) with a haptic tick and a cross-fade to the new account. A
  signed-out profile signs in again instead.
- **Design**: `primaryAction()` (glass prominent in the accent, the core's contrast-checked
  `onAccent` label; on TV the system's focused label), `secondaryAction()`, `FormField`,
  `AvatarView`, `GlowBackdrop` (radial gradients, no blur), `Skeleton`, `MarkdownView` (the
  core's safe tree), `QRCodeView`, `InsecureBadge`, `ProblemBanner` (every `Problem.code` in
  words). `Ambience` (`live|still|flat`) quiets the decoration for screenshots and snapshots.
- **Tests**: Swift Testing throughout. `CouchverseCore`: the runtime over the real core with
  fake executors, and the executors. `CouchverseDesign`: identicons against the web's output,
  localization and Czech plurals, colours, QR, markdown. `CouchverseFeatures`: deep links, code
  input, the countdown, the choreography, and `ScreenSnapshots` (swift-snapshot-testing) of every
  key screen and load state on iPhone, iPad and TV, English and Czech at the largest Dynamic
  Type. UI tests: a smoke test per app, and `LiveFlowTests` that pair a TV from a phone against a
  running server (skipped without `CV_LIVE_SERVER`). `make apple-test`, `make apple-uitest` and
  `make apple-lint` (swift-format) run them; `apple.yml` runs them all in CI.

## Android client (cross-cutting)

One app for phones and Google TV (`clients/android/`, docs/android.md), the TV UI picked at
launch from the UI mode, on the shared core.

- **Runtime** (`core/runtime/`): `CoreRuntime` owns the bridge on one serial thread for the
  process's life, stamps `SystemClock.elapsedRealtime`, performs every effect (OkHttp for
  `http`/`upload`/`socket`, coroutine timers, files for `store`, Keystore-sealed files for
  `secureStore`, a no-op `player` until Phase 12) and publishes one `StateFlow` per surface,
  re-read only when a `render` names it. At launch it reports `CapabilitiesReported` from
  `MediaCodecList`, the display's HDR types and HDMI passthrough (`core/device/`).
- **Screens** read view models (`rememberSurface`, which also opens catalog surfaces) and send
  events; they own navigation and nothing else. Each screen has phone and TV layouts behind
  one entry point, takes plain view models, and is rendered from fixtures in Roborazzi
  screenshots (phone and TV, en and cs) and Compose UI tests. A TV screen names where focus
  starts (`focusOnStart`, or `screenFocus`, which also returns focus to the element that had it
  when the screen was left) and its text fields let the arrows out (`remoteLeavesField`).
- **Root navigation** follows `AppView.phase`; the signed-in screens are keyed by account. A
  TV starts on "Who's watching?" (focus lifts a tile, the glow takes the focused account's
  colour, the chosen avatar flies into the sidebar as a shared element); phones open the last
  account and switch from the profile tab's sheet. `couchverse://connect|pair|title` links and
  scanned QR codes (CameraX + ZXing, phones only) go to the core as `LinkOpened`.
- **Design** (`design/`): Material 3 and Compose for TV themes over the tokens, tinted by the
  session's (or a title's) accent palette; spring motion with a cut under "Remove animations";
  the account's display language applied to resources at runtime; Coil for artwork URLs the
  core signs.

## Media grants (cross-cutting)

Players cannot reliably attach a cookie or an `Authorization` header to every request
(AVPlayer forbids custom headers, AirPlay receivers and the browser's media element
fetch on their own), so media is authorized by a capability in the URL.

- `internal/grant` signs `{scope, subject, resource, couch participant, expiry}` with
  HMAC-SHA256 (truncated to 128 bits) under a 32-byte key created at first boot
  (`grant.secret` setting; deleting it revokes every grant), base64url, 83 characters.
- **Media grants** (6 hours) name one media file. Everything that plays it lives under
  `/media/{grant}/...`, so relative HLS URIs inherit the grant; handlers read the file from
  `grant.From(ctx)` and never trust a path id (subtitles and instant-play sessions must
  belong to that file; instant-play sessions also to the grant's viewer). The playback
  payloads return `grant`, `streamUrl`, `originalUrl`, `hlsUrl`, `frameUrl` and subtitle
  URLs already signed. An expired grant answers 403
  `grant_expired` (fetch the payload again), a forged one 403 `invalid_grant`, a revoked
  couch grant 403 `grant_revoked`.
- **Artwork grants** (7 days) unlock artwork images (`?g=`) for system fetches (tvOS Top
  Shelf, AirPlay) via `GET /me/artwork-grant`, and for anonymous couch guests via the couch
  payloads.
- **Couch-bound grants** carry the follower's participant id and are re-checked against the
  live session on every request (see couch).

## Playback v2 (cross-cutting)

Every client says what it can play and the server picks the cheapest delivery that fits;
everything that is not the source file is fMP4 HLS that AVPlayer, ExoPlayer and hls.js play
with native audio and subtitle menus. Spikes: `docs/spikes/s3-apple-hls.md`,
`docs/spikes/s5-dolby-vision.md`.

- **Device profile** (`playback.DeviceProfile`, the body of `resolvePlayback` and
  `resolveCouchPlayback`; examples in `contract/fixtures/device-profiles/`): progressive
  `containers`; `video` codecs with `profiles`, `maxLevel` and `maxBitDepth`; `audio` codecs
  with `maxChannels` and `atmos`; `hdr` modes, Dolby Vision per profile (`dolbyVision5`,
  `dolbyVision8`...); `maxWidth`/`maxHeight`/`maxFrameRate`/`maxBitrate`; `hls` segment
  formats; `sidecarSubtitles` (formats rendered beside a direct-played file; none means
  subtitles need HLS, as on AVPlayer); `audioTrackSwitching` (inside a progressive file).
  The GET forms map `?caps=` onto `LegacyProfile`, the old browser baseline.
- **Decision** (`playback.Decide`, a pure function over profile, source facts, prepared
  state and server abilities; table-tested in `decision_test.go`): **direct** when the
  profile covers container, video (codec, profile, level, bit depth, size, frame rate,
  bitrate, HDR or Dolby Vision profile) and audio and nothing needs HLS renditions;
  **remux** (the copied source, `?video=original`) when the video fits but the container,
  audio, subtitles or languages do not; **transcode** (the ladder, `?video=ladder`; legacy
  MPEG-TS variants for profiles that play TS) otherwise; then `preparing` while jobs run, an
  instant-play session (`mode: jit` with a `jit` plan) when JIT is on, `unsupported` last.
  A file that direct-plays except for its subtitles or extra languages plays directly
  rather than waiting for its package. HDR the profile cannot show falls back to a
  compatible base layer (Dolby Vision 8.1/7 to HDR10, 8.4 to HLG) or the tone-mapped ladder.
  The payload's `tier` says what reaches the client, `originalUrl` is the "Original" quality
  (the file or the copied-source playlist, outside the ABR ladder) and `hlsUrl` the ladder.
- **Packages** (`transcode_variants` rows, `format` `fmp4`; pre-v2 rows are `ts`): `source`
  (the video copied, `hvc1` for HEVC, Dolby Vision signalled where ffmpeg 6+ can, profile 7
  stripped to HDR10), `audio` (directories `audio-<stream>-<codec>`: AAC stereo for every
  track plus AC-3/E-AC-3 copied or E-AC-3 5.1 made from other multichannel audio),
  `trickplay` (one 180p intra frame every 2 s, listed as `iframes.m3u8`) and the ladder rungs
  (H.264 High, tone-mapped and 8-bit, IDR every 2 s of source time). Every output shares one
  timeline (`-copyts`, a 1.4 s offset, `frag_discont`, negative CTS offsets) and 6 s
  segments; each directory gets a `rendition.json` with its codec string, size, range,
  channels and measured peak and average bit rates. `media.AutoPrepare` picks what to
  prepare at probe time: nothing for H.264/AAC MP4 with one audio track and no subtitles,
  the copied source otherwise (cheap), and with auto-prepare on the full ladder for sources
  some clients cannot decode (HEVC, AV1, 10-bit, HDR) or the rungs below an H.264 source.
- **Multivariant playlists** are written per request from the `rendition.json` files: every
  video rendition once per audio group ("surround" when the profile takes AC-3/E-AC-3, then
  "stereo"), with `CODECS`, `SUPPLEMENTAL-CODECS`, `RESOLUTION`, `FRAME-RATE`, `VIDEO-RANGE`,
  `BANDWIDTH`, `AVERAGE-BANDWIDTH`, a subtitle group of segmented WebVTT renditions built
  from the sidecar files (`X-TIMESTAMP-MAP=MPEGTS:126000`) and the I-frame playlist.
- **Instant play** encodes H.264 SDR fMP4 from the requested 6 s segment with the planned
  audio (copied when the profile takes it, else AAC or E-AC-3), restarts on far seeks, is
  bound to the grant's viewer (subject and couch participant) and is served behind a
  one-variant multivariant playlist with the subtitle renditions.
- **Checks**: `couchverse validate-hls <url>` (rules in the S3 spike), `TestPlaybackTiers`
  (every tier on generated media, validated; with `COUCHVERSE_AVPLAYER=1` played by
  `scripts/avplayer-probe.swift`), `make hls-check|hls-apple|hls-server-check|e2e-playback`.

## Optimistic navigation & caching (cross-cutting)

The web client is a client-only SPA (`ssr = false`), so every route's `load` runs in the
browser and used to block the page swap on a network round-trip - visibly slow on a
Raspberry Pi. Data pages are now **optimistic**: navigation swaps in at once and data
fills in behind a cached value or a skeleton.

- **The catalog is core-backed** (home, title, movies, series, genres, a genre, My List,
  search): the core's `catalog` module is the cache (stale-while-revalidate, LRU-capped,
  reset by the core itself on sign-out or an account change, a warm-start home in
  `localStorage`). A page's `+page.ts` only starts loading its screen and returns it
  (`features/catalog/api.ts`): `revisit(screen)` (home, title, My List: `RefreshRequested`,
  so every visit catches up with progress and list changes made elsewhere, as the SWR cache
  always did) or `preload(screen)`/`preloadListing(key)` (genres, listings: `ScreenOpened`
  then `ScreenClosed`, loading only what the core does not hold as fresh, 60 s). The page
  holds the screen open while mounted (`useScreen` from `lib/core/screen.svelte.ts`:
  `ScreenOpened`/`ScreenClosed`, with `revalidate` it reloads unless its load just did) and
  reads the view model reactively. A load cannot own the open/close pair: hover, TV-focus
  and the player's preloads run loads for pages that never mount, and SvelteKit reuses a
  preload's result without running the load again.
- **SWR cache** (`lib/api/cache.svelte.ts`): what the web still fetches itself (a public
  profile and the leaderboard in `ranks`, the devices list) uses a reactive
  stale-while-revalidate store over `svelte/reactivity` `SvelteMap`. `createSwrCache<T>()`
  registers an instance (LRU-capped); `get(key)` is a reactive read, `revalidate(key,
  fetcher)` fetches + stores (only the newest fetch, and only if nothing was written since
  it started), and `resetAllCaches()` (called by `session` on logout/401) drops everything.
  Instances live in `features/<name>/cache.svelte.ts`.
- **Page pattern**: the route shell renders `XxxPage.svelte`, a thin wrapper around
  **`CachedView`** (`lib/components/`), which renders one of its snippets: `content` (the
  cached value paints instantly and updates silently when revalidation lands - stale beats
  blank), `skeleton` (cold visit), `notFound`, or `failed` (a cold visit that could not
  load, with a Retry: `LoadFailed`). Core-backed pages pass the view's content (`shown(view)`,
  or `TitleView.detail`) and its `LoadStatus`; SWR pages their cache value and the load's
  un-awaited `fresh` promise. The page body lives in `XxxContent.svelte`. Skeletons reuse
  `ui/Skeleton.svelte` + `animate-shimmer` and mirror each real layout.
- **`StreamedView`** (`lib/components/`): the same three states but promise-backed and
  **uncached** - it keeps the last resolved value during a same-`key` revalidation
  (`invalidateAll` after a save) so it never flashes the skeleton mid-edit. Used by the
  admin editors (which must stay fresh per visit). The `watch` page uses the same
  keep-last-value shape inline, since it also drives the transcode-preparing poll.
- **`NavProgress`** (`lib/components/layout/`): a thin accent bar driven by `navigating`
  (`$app/state`), mounted once in the root layout with a `view-transition-name` opt-out -
  instant click feedback for any navigation the skeletons do not already cover.
- **Preload rule**: `preloadData` is only ever called for **side-effect-free** routes (the
  player warms `/title/{slug}` on mount so "back" is instant). A `/watch/...` load starts a
  JIT transcode, so watch routes are **never** data-preloaded - links to them use
  `data-sveltekit-preload-data="tap"` (not the global `hover`), so merely hovering an
  episode row or continue-watching card cannot spawn a transcode on the Pi. In-player
  episode navigation dropped `invalidateAll` (the `[id]` change already re-runs the watch
  load; the root layout's core start and preferences fetch stay put), and the transcode poll
  uses a targeted `invalidate('app:playback')`.

## Shared core on the web (cross-cutting)

The web runs the shared client core (see "Shared client core") as wasm for the session, bio
markdown and the viewer's catalog. Its ranks, profile, playback and couch modules wait for
their slices, so the rest of the viewer still calls the API itself.

- **Runtime** (`lib/core/`): `index.ts` creates the one `core` (`CoreRuntime`,
  `runtime.svelte.ts`) in cookie mode and starts downloading and compiling the wasm
  (`wasm.ts`) as soon as the root layout's module loads. The root `load` awaits
  `core.start()` (the wasm plus the session's first load, alongside the web's preferences);
  the guards (`(app)` and `admin` layouts, login, the rankings pages) `await parent()`, so they
  run with the session known. The runtime stamps `nowMs` from `performance.now()`, performs
  effects through `executor.ts` (`fetch` with the cookie, `localStorage` under `cv.core.`,
  timers; secure-store reads are empty and writes refused, the web keeps no secrets; `upload`
  fails, the web still uploads images itself) and keeps `app` and `session` in `$state.raw`,
  re-reading only the surfaces a `render` names and reusing the unchanged parts of a view so
  effects reading them stay quiet. `send(event)` settles once every effect it led to has
  finished (timers aside).
- **Screens**: `core.view(surface)` reads any surface's view model; it stays current (a
  `SvelteMap` entry re-read on every `render` naming it, unchanged parts and array items
  kept) while something watches it: `open(surface)` (a screen: `ScreenOpened` now,
  `ScreenClosed` from the returned close; pages use `useScreen`) or `watch(surface)` (no
  events, for the notices). `revalidate` sends `RefreshRequested`, `prefetch` an open and
  close pair. Surfaces are matched by `surfaceKey` (JSON with sorted keys), since the core's
  renders order a `BrowseKey`'s fields its own way. A restarted core is sent `ScreenOpened`
  for every screen still open.
- **Catalog**: `+page.ts` loads and pages as in "Optimistic navigation & caching". Infinite
  scroll sends `BrowseMoreRequested` when a sentinel under the grid comes within 800 px, the
  search box sends `SearchChanged` on every keystroke (the core debounces, supersedes and
  keeps the last query, so coming back to `/search` shows it again), My List toggles send
  `WatchlistChanged` (shown at once, rolled back by the core). `lib/core/Notices.svelte`
  (root layout) turns the `notices` view into svelte-sonner toasts, localizing each code,
  and sends `NoticeDismissed` when a toast closes.
- **Traps**: a Rust panic aborts the wasm instance. The wasm-bindgen glue keeps its instance
  in module scope, so each core evaluates its own copy of the glue; after a trap a new core
  sends `AppStarted` again while the old views stay up. Markdown that trapped it renders as
  plain text from then on; a second trap while restarting gives up.
- **Session**: `features/auth/session.svelte.ts` reads `core.session` (user, flags via
  `features/settings/features.svelte.ts`, language, accent). The login form posts
  `/auth/login` itself and sends `SessionStarted`; logout sends `SignOutRequested`; a profile
  edit or admin settings save sends `SessionChanged`, and so does a 401 from a web API call
  (the core confirms with its own `/auth/me`); a tab coming back into view sends
  `AppBecameActive`. The session store watches the core's user: when it goes away without a
  sign-out (that confirmation, or a 401 on one of the core's own requests such as the
  catalog's), the web clears its caches and, once the navigation under way has landed, sends
  session-only routes to `/login?next=`. The root layout applies `session.accent` to `:root`
  (`lib/theme.ts` `applyPalette`); scoped accents (`accentVars`) are still derived in TS, the
  same way (a test checks the two agree). In cookie mode the core leaves achievement checks to
  the web's `rank` store, which celebrates the unlocks a check returns, until the web renders
  the core's ranks views.
- **Build**: `make core-wasm` writes `lib/core/pkg/` (gitignored); `make build`, the
  Dockerfile (a `$BUILDPLATFORM` Rust stage with binaryen) and `web.yml` build it before the
  web, and `make check` runs vitest (`npm test`) against the real wasm with a scripted
  executor.

## TV mode (cross-cutting)

The same SPA runs as a Titan OS app (Philips/JVC smart TVs): the TV loads the hosted URL
in its Chromium browser, there is no separate build. `lib/tv/` switches the UI to a
remote-controlled, 10-foot mode when the user agent has `TitanOS/` (or `WhaleTV/`,
`SmartTvA/` on older Philips Linux TVs). `?tv=1` forces it in a desktop browser for
development (persisted in localStorage `cv.tv`; `?tv=0` clears it).

- **Detection + styling** (`lib/tv/tv.ts`): `isTV` is fixed per page load and sets
  `data-tv` on `<html>`. `app.css` scales the root font with the viewport width
  (`1.125vw`, so the layout reads the same at the TV's 720p or 1080p app resolution),
  thickens the focus ring and turns off `backdrop-filter` (TV GPUs stutter on blur);
  `GlowBackdrop` drops its blur too. View transitions are skipped. The TV hides admin
  entry points (nav menu item, upload dock) and the player's volume, fullscreen and
  picture-in-picture controls (the TV remote owns volume; the app is always full screen).
- **Spatial navigation** (`lib/tv/spatial-nav.ts`): arrows move focus to the nearest
  focusable element that way. Vertical moves prefer an element straight ahead unless the
  nearest row is clearly closer (so a short row is never skipped); sideways moves stay in
  the row and stop at its end. Focus scrolls real scroll containers and centres the page
  by hand, never via `scrollIntoView` (that also scrolls overflow-hidden boxes like the
  hero and shifts their art). Opt-in attributes: `data-tv-autofocus` (where a page starts:
  hero/title Play, login username, player seek bar), `data-tv-pin` (fixed chrome: the nav
  bar, corner stack - reached only by leaving the page past its top/bottom
  edge, so it never competes with content scrolling under it), `data-tv-layer` (a custom
  overlay that confines focus like a bits-ui dialog), `data-tv-skip`.
  Open bits-ui overlays (`role=dialog|alertdialog|menu|listbox`) confine focus too.
  Elements fading in count as visible (checked through `document.getAnimations()`), but
  transparent hover-revealed controls do not.
- **`TvShell`** (`lib/tv/TvShell.svelte`, mounted by the root layout on TVs only): owns
  the window keydown listener, which runs last and skips anything a feature already
  handled (`preventDefault`). Back (`Backspace`, keyCode 8 on Philips, 461 on JVC/Vestel)
  closes the open overlay (synthesised Escape), deletes in a non-empty text field, goes
  back in history, and on the main screen (home, login) asks to exit - Titan OS requires
  that confirmation; `exitApp` calls `SmartTvA_API.exit()` or `window.close()`. It tracks
  in-app history depth so Back never leaves the app, restores focus to the card a page
  was left from on popstate, places arrival focus (retrying briefly while content lands),
  moves focus from a freshly opened dialog's container to its first control, keeps arrow
  keys on select/menu triggers from opening them (OK opens), and preloads a focused link
  after 300 ms with the same opt-in as hover (watch links stay `tap`).
- **Player** (`VideoPlayer`): on TVs its keydown listener sits on `document` so it runs
  before the shell. With the controls hidden or the seek bar focused, OK plays/pauses and
  left/right skip 10 s; other keys bring the controls up on the seek bar; once a control
  button has focus the arrows are spatial navigation. Media keys work any time. Controls
  stay up while a player menu is open. HLS always goes through hls.js on TVs (their
  browsers claim native HLS, which would lose the quality and audio menus).
- **Cards** mirror their hover look with `group-focus-visible:` (ring on the artwork, so
  the anchor drops its own outline); the hero carousel holds its slide while focus is
  inside it (advancing would re-create the focused button).
- Getting it onto a TV: Titan OS apps are hosted URLs registered in the Titan OS Partner
  Portal; DevView on the TV launches unpublished ones, and Chrome DevTools attaches to
  `<tv-ip>:9222` or `:7001` with Debug Mode on (docs.titanos.tv).

## Backend dependency graph

A feature may import another feature's `Store` or exported services, never its
handlers. SQL may JOIN any table (joins create no Go dependency). The import graph
must stay acyclic:

```
artwork <- auth <- catalog <- metadata
            ^        ^
            +--- library <- subtitles <- playback <- couch        system
analytics <- catalog (leaf: imports kernel only)
playback -> {auth, catalog, library, subtitles}
ranks -> {auth, catalog}  (near-leaf; nothing imports ranks)
downloads -> {auth, catalog, library, subtitles, playback}  (nothing imports downloads)
couch -> {playback, auth}  (top of the DAG; nothing imports couch)
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
- `couch` sits at the top: it imports `playback` (to reuse `BuildPlayback`) and
  `auth`, reaches analytics and ranks only through the `CouchWatchRecorder`/
  `CouchStatsRecorder` interfaces it declares (wired in `main.go`), and nothing
  imports it. The anonymous-stream guard is inverted into `internal/server` so `playback`
  never depends on `couch`.
- `ranks` reads a dozen other features' tables but imports only `auth` (for
  `UserFrom`) and `catalog` (for the exported `Localize`/`GenreLabel`); everything
  else is a SQL join. **Nothing imports `ranks`**, which is what makes the couch
  counters safe: `couch` declares the `CouchStatsRecorder` interface itself and
  `main.go` hands it `*ranks.Store`. For the same reason the nav rank badge reads
  `GET /me/stats` and is deliberately **not** added to `/auth/me` - that would need
  `auth -> ranks`, and `ranks -> auth` already exists.

## Adding a feature (recipe)

Backend:
1. Create `backend/internal/feature/<name>/` with `store.go` (`type Store struct { db *pgxpool.Pool }`,
   `NewStore`), typed handlers, and `routes.go` with `Register(rt httpx.Routes)`.
2. Add migrations in `backend/migrations/` (next `NNNN_` prefix; goose runs them at boot).
3. Construct the store in `internal/app` (add it to `server.Deps`) and call the feature's
   `Register` in `internal/server/server.go` next to the other features.
4. Register background job handlers (if any) on the runner in `app.Start`.
5. Add conformance cases for every new operation in `internal/server/api_test.go`, run
   `make contract` and commit the regenerated files.

Web:
1. Create `clients/web/src/lib/features/<name>/` with `api.ts` (all endpoint calls),
   optional `types.ts` and `*.svelte.ts` rune stores, `components/`, `pages/XxxPage.svelte`.
2. Add a thin route shell in `src/routes/...` that renders the page component
   (loaders in `+page.ts` stay in routes/ and delegate to the feature's `api.ts`).

Verify: `make lint check test build`, then `make run-backend` + `make run-web`.
