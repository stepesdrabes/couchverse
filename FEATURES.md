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
| library | `internal/feature/library` | `features/library`, `features/uploads` | `libraries`, `media_files`, `upload_sessions`, `transcode_variants`, `media_markers` |
| metadata | `internal/feature/metadata` | (admin UI in `features/library`) | (writes catalog tables) |
| subtitles | `internal/feature/subtitles` | (player UI in `features/playback`/`preferences`) | `subtitles` |
| artwork | `internal/feature/artwork` | (via `catalog/api.artworkUrl`) | `artwork` |
| jobs | `internal/feature/jobs` | `features/jobs` | `jobs` |
| system | `internal/feature/system` | `features/settings`, `features/admin` | `settings`, `home_rows` |
| analytics | `internal/feature/analytics` | (charts in `features/admin`) | `watch_time_daily`, `watch_time_hourly`, `couch_watch_time_daily`, `tv_watch_time_daily` |
| couch | `internal/feature/couch` | `features/couch` | (in-memory; only `couch_watch_time_daily` via analytics) |
| ranks | `internal/feature/ranks` | `features/ranks` | `user_achievements`, `user_counters`, `user_tv_viewing` |
| downloads | `internal/feature/downloads` | (native clients only) | `download_files`, `downloads` |
| couchtv | `internal/feature/couchtv` | `features/couchtv` (the admin section and title tools; the flag in `features/settings`; the viewer pages are next, on the core's `couchtv` module) | `couchtv_airings`, `couchtv_strips`, `couchtv_titles`, `couchtv_soundtracks`, `couchtv_media`, `couchtv_clips`, `couchtv_breaks`, `couchtv_guests`, `couchtv_bulletins` (the room is in memory) |

Shared kernel (backend): `internal/app` (wiring of stores, services and the job runner),
`internal/config` (env), `internal/db` (pool, migrations, `ErrNotFound`), `internal/httpx`
(the huma API, error envelope, `Routes` groups, `Tag`/`Localized`/`Guard`/`Raw`), `internal/media`
(ffprobe, codec compatibility, filename parsing, transcode ladder and auto-prepare policy,
shared `MediaFile`/`Subtitle` row types, the analysis measurements in `analyze.go`:
chromaprint, EBU R128 loudness and its steady stretches, small decoded frames; `BackgroundCmd`,
which runs background ffmpeg work (transcodes, analysis) at nice 19 in the idle I/O class where
`ionice` exists; `media/mp4` reads fMP4 init sections and segments),
`internal/hls` (HLS playlists: writer, parser, WebVTT segmenting and the validator behind
`couchverse validate-hls`), `internal/settings` (settings KV store),
`internal/flags` (admin-toggleable feature flags), `internal/grant` (media, artwork and
channel grants), `internal/slug`, `internal/version`
(build version, API level), `internal/server` (composition root: middleware, API groups,
feature registration, SPA fallback).

Shared web: `lib/api/client.ts` (fetch wrapper - never hand-write URLs in
components) and `lib/api/problem.ts` (an error in the display language), `lib/components/ui/` (bits-ui
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
  disabling a user deletes all of theirs. A device names its platform: `ios`, `ipados`,
  `tvos`, `android`, `androidtv`, or `desktop` for an app on a computer (the CouchPush
  uploader signs in as one, "CouchPush on <computer>").
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
  `features/users` (AdminUsersPage), `features/preferences` (the web-only subtitle style;
  the display language is the core's). `/profile` renders the *same* `ProfileContent` as
  `/u/you` with owner affordances, off the same core screen (`profile(you)`) - there is no
  separate account page and no tabs. With rankings off it falls back to
  `AccountOnlyProfile`. The modals save through the core's profile events
  (`ProfileEditSubmitted`, `PasswordChangeSubmitted`, `ImageChosen` with a handle to the
  picked file, `ImageRemoved`, and `ProfileVisibilityChanged` for the privacy switch, which
  shows your profile's `public` as the core holds it), await the send and read how it went
  from the `profileEditor` view; the core then shows the new profile everywhere (session,
  nav, profile) without the web reloading anything.
- Web, devices: `/pair` (`PairPage`) approves or denies a pairing code, typed into the
  segmented `PairCodeInput` (one real input under the cells) or prefilled from the
  device's QR (`?code=`); signed-out visitors go through `/login?next=` and back. Both
  profile variants end with `DevicesSection` (`ProfileContent` takes it as its owner-only
  `children`): the account's sessions from the core's `devices` view (`DevicesOpened`,
  `DeviceRevoked`; `features/auth/devices.ts`), revoke behind a confirm (this browser's own
  row is a plain logout, which also clears the cookie), and `ConnectDeviceModal`, which shows
  a one-time connect code as a QR (`ui/QrCode.svelte` over `uqr`), replaces it when it
  expires and closes itself once a new device shows up in the core's list, which it reads
  again every 3 s.

### catalog
The watchable catalog: movies and series with seasons/episodes and genres, the home
page (featured title, admin-curated rows, continue watching), browse with filters,
full-text search, per-user watch progress and My List (watchlist). Admin side: the
library table, title/season/episode CRUD and bulk actions.
- Endpoints: `/home`, `/titles`, `/titles/{slug}`, `/search`, `/genres`, `/progress`,
  `/me/continue-watching`, `/me/watchlist...`; admin `/admin/library`, `/admin/titles...`
  (incl. `DELETE /admin/titles/{id}/languages/{lang}` to drop a content language and
  `GET /admin/titles/{id}/storage` for the per-title disk-usage breakdown),
  `/admin/seasons/{id}...`, `/admin/episodes/{id}`. `adminUpdateTitle` announces a change of
  `allowRandomPlayback` through `Store.RandomPlaybackChanged` (Couch TV airs such a series'
  strips at random, see "couchtv").
- **Audio inventories**: the admin title read (`adminGetTitle`) carries `audioStreamsByFile`
  next to `subtitlesByFile`, keyed by media file id: each probed file's audio tracks in stream
  order (`media.AudioStream`: index, codec, the language tag as the file has it, name,
  channels, default, profile, layout, sample rate), read from `audio_streams` by SQL. A file the
  prober has not read yet has no entry, not an empty one: its languages are unknown, so an
  uploader replacing files (CouchPush) keeps it rather than taking it for one it covers.
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
- Web: the core's `playback` module plays (see "Shared core on the web"). `WatchPage` asks it
  to play what `/watch/{kind}/{id}` names and follows the URL when the core moves on (next
  up, a remote); `VideoPlayer` draws the core's `PlayerView` (qualities, audio and subtitle
  choices, the episode switcher, shuffle, the next-episode countdown, frame previews, a
  follower's linear player) with banner-accented chrome, bits-ui tooltips, shortcuts and the
  TV remote, and sends the viewer's choices as events. `ElementPlayer`
  (`playback/element-player.svelte.ts`) runs the core's player commands on the `<video>`: a
  file, HLS through hls.js (Safari's own where it has audio menus), a pinned quality, sidecar
  WebVTT drawn in the page's own overlay, audio renditions; it reports about once a second and
  on every change. The watch route's load only names the title, so preloading it cannot start
  a transcode. `lib/core/device-profile.ts` measures the browser once per launch.
- **The audio menu** (`playbackInfo.audio`, built by the pure `audioMenu` in
  `playback/audio.go`) spans the played file and its separate-language siblings (model B), each
  sibling decided for the same profile under its own grant. A file offers its embedded tracks
  (`embedded:<file>:<stream>`, labelled from their names or ISO 639-2 tags such as `eng`,
  `cze`) where the device can switch between them, else itself as one choice in the language
  it plays; `Decision.DefaultAudioOnly` says when only the default track reaches the device (a
  progressive file it cannot switch tracks in before its audio package is ready, instant play,
  pre-v2 MPEG-TS without the multi-audio remux), so those tracks are held back rather than
  offered as a choice that does nothing. A sibling's choices carry the sibling's stream or
  playlist and the played file's none, which play from its source at the chosen quality; only
  the played file's default is selected; a sibling that cannot play on the device is left out.
  A file's tracks are listed together in its player's order, which is how the core counts a
  rendition's place.
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
  hard-deletes the file plus its source, HLS/frame/fingerprint caches and subtitle files on
  disk), `GET/PUT /admin/media-files/{id}/markers` (`adminGetMediaMarkers`,
  `adminUpdateMediaMarkers`), `/admin/uploads...`.
- Job handlers: `probe`, `reprobe` (enqueued at every start; brings rows an older prober
  read up to `media.ProbeVersion`, from ffprobe or, once the source is gone, from the stored
  probe output, and queues the cheap HLS v2 package for them when auto-prepare is on) and
  `analyze_markers` (below).
- **Media markers** (`media_markers`, `library.MarkerAnalyzer`, docs/couch-tv-plan.md 3.1 and
  6.3): where a file's `intro` and `credits` are (a series episode, both ranges: the theme and
  the end credits) and, for a movie, where its `credits` start and the `credits_music` under
  them (a range), in ms. The analysis writes `auto` rows stamped with `MarkersVersion` (also
  on `media_files.markers_version`, so a file without markers is not analyzed again and a
  newer analyzer re-runs); an `admin` row of a kind overrides the auto one and is never
  touched by the analysis, and an admin row without a start (`none`) says the file has none
  of that kind. `PUT .../markers` replaces a file's admin rows (a kind left out falls back to
  the analysis). Couch TV airs programs to their effective `credits` start and picks its
  promo audio from `intro`/`credits_music`; VOD can use them later (skip intro). The title
  editor's Couch TV panel edits them (`features/couchtv` `MarkersEditor`).
- **`analyze_markers`** (one slot, priority -5 for new files and -10 for the backfill, every
  ffmpeg nice'd): queued per new video file by the prober; at every start a backfill job
  queues every analyzable file (primary, video, source on disk, on a movie or an episode)
  an older analyzer looked at. A movie file is analyzed on its own: its last 12 minutes are
  decoded at 2 fps (480x270, `media.Frames`) for the credits look (a card: mean luma < 45,
  saturation < 6, at least 6 pixels over 150; a crawl: the picture moved up 1-40 rows since
  the last sample with the error at that shift under half the unmoved one and under 6, at a
  steady speed) and measured with ebur128 for steady music (`media.Loudness.Steady`: 10 s
  windows of one-second means above -35 LUFS within 3.5 LU, the longest run of at least
  30 s is `credits_music`). Credits start at the first sample that begins 60 s with 85 %
  credits-like samples and 60 % of everything after it (a post-credits scene may follow);
  with the music too the later answer wins unless the picture shows credits all the way
  from the earlier; the music alone only when it lasts 90 s and ends within 45 s of the end.
  An episode file is fingerprinted (chromaprint, mono, raw: its first 12 and last 6 minutes,
  cached under `DATA_DIR/cache/fingerprints/<id>/`) and queues its season (payload `titleId`
  plus one season, 2 minutes later so an upload batch is matched once). The season job
  matches every file with its three nearest other episodes (the longest run of items within
  10 differing bits over every offset, single-item dropouts bridged, 0.1238 s per item); a
  file's intro (credits) is where at least two of its pairs agree within 2 s at either end,
  the span they share, at least 8 s; no consensus, no marker. Without the chromaprint
  muxer (Homebrew's ffmpeg) series are skipped and stay unanalyzed. A file ffmpeg cannot
  read is marked analyzed after its last attempt. Every change tells Couch TV
  (`MarkerAnalyzer.Analyzed`, wired in `internal/app`), which picks the title's promo again.
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
- Job handlers: `fetch_metadata`, `import_episodes`, `fetch_logos`.
- Apply also keeps the base language's **tagline** (`titles.tagline`; the other languages keep
  theirs in `translations`, and dropping the base language promotes the next one's) and a
  movie's TMDB **collection** (`belongs_to_collection` as `collection_tmdb_id`,
  `collection_name`, cleared when TMDB has none), for Couch TV's promos and Saga Sunday. No UI
  shows them yet.
- **Logos** (`metadata/logos.go`): one `GET /{movie|tv}/{id}/images` call with
  `include_image_language=<languages>,null`, the languages being the content languages and
  English (`logoLangs`: Couch TV's cutscenes are English, D13, and an English logo also serves
  English-display viewers); for each the best-voted PNG logo in that language (vote average,
  then vote count), else the best-voted language-neutral PNG, is stored as the title's logo in
  that language (SVGs are skipped; a legacy title without content languages asks for English
  and stores a null-language logo; languages sharing the neutral fallback share one download).
  Apply replaces them and drops TMDB logos it did not store (a re-link to another TMDB entry
  leaves none of the old one's behind) while uploaded logos are only ever overwritten; a
  failure fails the job like a poster's. The episode import only fills languages without a
  logo, best effort.
- **Logo backfill** (`metadata.LogoBackfill`, job `fetch_logos`, queued at every start,
  priority -10): titles linked to TMDB before logos existed (the production library) get the
  logos they lack, filled as the import fills them (never replacing a logo; a title with a
  logo not tied to a language keeps it as every language's), without an admin applying each
  title again. A title is marked once looked up (`titles.logos_version`), whether TMDB had
  logos for it or not (a 404, a link TMDB no longer knows, counts too), so it is not asked
  again at every start; raising `LogosVersion` (a better pick) looks every title up once more.
  Only with a TMDB key; titles go one at a time, and three failing in a row (TMDB down, the
  key refused) end the run, the job's retries and the next start carrying on. Tests:
  `TestLogoBackfill` (a fake TMDB).
- **Music facts** (`metadata/music.go`, for Couch TV's soundtrack finder): one
  `GET /{movie|tv}/{id}?append_to_response=credits,alternative_titles` answers the composers
  the crew names ("Original Music Composer", "Main Title Theme Composer", "Music",
  "Composer") and the US and British names ("Harry Potter and the Sorcerer's Stone").
  `metadata.MusicFacts` serves them with the server's key (nothing without one) through the
  `couchtv.MusicFacts` interface, wired in `internal/app`.

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
`Service.Within(art, width, height)` is a bounded copy for Couch TV's renders (no client asks
for one): the original when it fits, else made once into the same cache
(`<id>_<mtime>_<w>x<h>`), a logo as a PNG scaled like a stored one (below), anything else a
JPEG at `-q:v 2`.
- Endpoints: `/artwork/{id}`; admin `POST /admin/artwork` (multipart: `ownerKind`,
  `ownerId`, `kind`, optional `lang`, `file`), `DELETE /admin/artwork/{id}`.
- **Slots** are `(owner_kind, owner_id, kind, lang)` (`UNIQUE NULLS NOT DISTINCT`): `lang` is
  an ISO 639-1 code for language-bound art and null for the rest, so a title holds one poster
  and one backdrop but one **logo** per content language. Original files are named
  `<kind>.<ext>`, or `<kind>-<lang>.<ext>` for a language-bound slot.
- **Logos** (`kind='logo'`, titles only) are transparent PNG wordmarks for cinematic heroes:
  uploads must be `.png` (TMDB's SVG logos are never stored), and their resizes are cached as
  PNG instead of JPEG so the alpha channel survives. Every stored logo is cropped to its
  visible pixels: clients lay a logo out from the leading edge and by its aspect, so a
  transparent margin would read as an indent. It is also scaled down to fit within 2560 x 1280
  (`fitLogo`; TMDB's run to 12000 px wide, 184 MB decoded), each pixel the average of what it
  covers weighted by alpha, since a plain average mixes in the transparent pixels' colour
  (black, as often as not) and draws a dark fringe around light letters. `artwork.PickLogo`
  chooses which one a viewer sees (see catalog). Dropping a content language deletes that
  language's logo (`Service.DeleteForLang`).

### jobs
The Postgres-backed job queue (no Redis): enqueue/claim with `FOR UPDATE SKIP LOCKED`,
per-type concurrency slots, retries with backoff, progress reporting, admin
cancellation, and the in-process worker runner. Other features register handlers in
`internal/app` (`App.Start`); the hourly `cleanup` handler lives there too
(`internal/app/cleanup.go`). A second process may work the same queue (the Couch TV render
sidecar registers `couchtv_render` and `couchtv_assemble`, which the app never does): at
start a runner requeues only the running jobs of the types it registered, and
`Runner.Yield` keeps a type from being claimed while a job of other types runs anywhere
(claimed within the last hour, so a vanished process holds nothing up for long), backfill (a
negative priority, `library.PriorityBackfill`) aside unless asked: the render sidecar yields
to `transcode_hls`, and `analyze_markers` and `couchtv_analyze` yield to any render or
assembly, since Chromium and an analysis together push a Pi into swap. `Runner.Share` puts types
in one pool of slots: the app's heavy jobs (`transcode_hls`, downloads, `analyze_markers`,
`couchtv_analyze`), each of which keeps every core of a Pi busy, take turns within the
transcode budget, the higher priority first, while quick jobs run on the other workers.
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
dimension, so anonymous followers count), called best-effort by the couch hub. Couch TV
has its own bucket, `tv_watch_time_daily (day, user_id, title_id, seconds)` (unique with
nulls not distinct: a null user is a guest, a null title a break or a filler), upserted by
`RecordTvWatch` from the channel's room through its `TvWatchRecorder` interface.
- Endpoints (admin): `/admin/analytics/overview?days=N` - dense daily series
  (video/couch/TV seconds, active users), totals, top titles, top couch
  titles, top users (each with `avatarId` for the dashboard leaderboard).
- Web: charts + "Top viewers", "On Couch watch-time" and "Couch TV watch-time" cards on
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
- Endpoints: `POST /couch` (create, or hand the host's live session over to this device; host
  must be logged in; 409 `already_hosting` from a browser whose other tab plays for it),
  `POST /couch/{token}/join` (public, anon OK; the host's own account always joins as a
  remote, `?remote=true` only asserts it and gets 403 `not_host` for anyone else; a browser
  playing for the session as its host in another tab gets 409 `already_hosting`, since its
  tabs share one couch cookie), `POST /couch/{token}/leave`,
  `POST /couch/{token}/end` (any of the host's devices), `GET /couch/{token}/playback`
  (follower payload for the current media), `GET /couch/{token}/ws` (the sync socket;
  same-origin enforced for browsers).
- **Participant tokens**: create and join mint a token per device (a participant may hold
  several, so a host joining from a phone does not sign out their TV). Browsers get it as the
  couch cookie; native apps pass `?delivery=body`, receive `participantToken` and send it as
  `X-Couch-Token` wherever a browser sends the cookie (socket, playback, leave, end). A native
  guest without an account joins the same way, with no session at all.
- **Remote control**: a remote connection (the host's account joining by code, on any device
  but the playing one: never a second host, which would broadcast its own empty state over the
  player's and end the session by leaving) receives `host_state` like a follower but plays
  nothing; its `remote_command` frames (`play|pause|seek|next|previous`, `positionSeconds` for
  seek) go to the host's playing connection, the one that last sent `host_state`, which applies
  them and broadcasts the result. A remote leaving keeps the session; a follower or anyone
  sending it is refused.
- **Host handover**: the host's account starting the session on another device while one
  plays for it hands it over. That device gets a host token and the room's *seat*, which it
  takes with its first `host_state` (the core reports as soon as its socket opens): the
  host's other connections become remotes, told by a fresh `hello` with `role: remote`, and
  their tokens remotes' tokens, so those devices come back as remotes. Until then the old
  device keeps playing for everyone, so nobody waits. The new device plays its own title: the
  session's media moves with that first report (`media_changed`), and followers keep their
  seats and sockets. A remote's late `host_state` is dropped, not a protocol violation. Tabs
  of one browser share the couch cookie, so a tab cannot take over from another tab playing
  for the session (409 `already_hosting`), while a reloaded tab, its socket gone, can.
- WS protocol: host broadcasts authoritative `{media, playing, positionSeconds,
  serverTimestamp, seq}` (server-stamped) + emoji; followers extrapolate position
  from the last update + local elapsed and hard-seek past ~3s drift. Host identity
  is tied to the user (a handover between its devices, 60s reconnect grace).
- **Stream authorization (cross-feature):** followers (incl. anonymous ones) reach only
  the host's current media. `GET /couch/{token}/playback` builds their payload with
  `playback.BuildPlayback` and a `playback.Viewer` naming their participant, so every
  media grant in it is bound to that participant. `playback` must not import `couch`,
  so the check lives in the composition root: `internal/server`'s media-grant
  middleware asks the Hub's `AllowsMedia(participantID, mediaFileID)` on every request
  carrying such a grant, which revokes it the moment the follower leaves, the host
  switches media or the session ends. The couch payloads also carry an artwork grant
  (`artworkGrant`) so anonymous guests can load avatars and backdrops.
- Web (`features/couch`): the core's `couch` module runs the session (the socket with
  reconnect, the host's broadcasts from its player reports, the follower's drift correction
  through player commands, reactions, remote commands) and plays the host's media for a
  follower. The `couch` store reads its `CouchView` and sends the couch events; a public
  `/couch/[token]` route previews the session (`GET /couch/{token}/info`, the web's only
  couch call) and joins on the viewer's click, so the video may autoplay, for guests without
  an account too (the core rides on the couch cookie and the session's artwork grant). The
  host's own account joining there gets `CouchRemote` (the host's clock run on from its last
  report, play and pause, 10 s skips, the episodes either side, leaving), and so does a hosting
  tab once its account hosts on another device (the core closed its player). A failed join or
  start says why through `problemMessage`, and "Open in the app" links
  `couchverse://couch/<code>?server=<origin>` (not on a TV). The assembled accent-recoloured
  `Couch` (seated avatars + host remote), a management popover, couch buttons in the player
  control bar + TopNav and a bundled (no-CDN) emoji picker complete it. `VideoPlayer` is the
  same for hosts and followers; a follower's is linear.

### ranks
Player progression: XP, rank tiers, achievements, public profiles and the global
leaderboard. Gated by the admin `rankingsEnabled` flag (default on, mirrors
`couchEnabled`).
- **XP** defaults to `2/min` video + `1/min` Couch TV + `100` per finished movie + `20`
  per finished episode + `50`/`25` per couch session hosted/joined + the achievement
  rewards (bronze 50, silver 150, gold 400, platinum 1000). Couch pays per session because
  on-couch watch-time has no user dimension. A Couch TV minute (programs and breaks, not
  the standby card or the slate) is worth half a video minute: the channel needs no
  choosing and never stops, so a set left on earns all day, its minutes include the
  breaks, and its programs still pay the movie/episode bonus through the progress the
  room saves (rates are whole XP, so "a little less" than 2 is 1). XP counts all recorded
  activity **regardless of flags** - turning couch or Couch TV off must never demote
  anyone. Every term is one explainable line of the profile's XP breakdown (`tv` for the
  channel).
- **Every rate and threshold is admin-tunable** (`ranks.RankConfig` in the `ranks`
  settings key, defaults in `DefaultConfig`, edited on `/admin/ranks`). `RankConfig` is
  threaded through `ComputeXP`/`TierFor`/`ProgressFor` rather than read globally, so
  the pure functions stay testable. `Validate` rejects negative rates and a ladder
  that does not start at 0 or does not strictly ascend. Because XP is always derived,
  a saved change re-levels everyone on their next read - nothing is recomputed or
  migrated. The stored rates are decoded over the defaults (`decodeConfig`), so a config
  saved before a rate existed (`tvMinute`) keeps that rate's default instead of zero.
- **Tiers** (10, couch-themed, `ranks.defaultTiers`): rookie 0, remote 500, snack 1500,
  binger 3500, popcorn 7000, marathoner 13000, sage 23000, cinephile 40000,
  master 70000, legend 120000. Codes, levels and colours are fixed identity (persisted
  in payloads, translated by the clients); only the thresholds are configurable.
- **Achievements** (38) are a pure rules table in `achievements.go`: each is a
  `Target` plus a `Value(Snapshot)`, scored against one `Snapshot` that a single
  batch of SQL fills, so the whole catalogue is unit-testable without a database.
  `Evaluate` runs **two passes** - non-meta rules first, then the meta rules against
  the resulting count - so the badge that rewards ten badges unlocks in the same
  call as the tenth. Rules gated on `couch` or `couchtv` are **absent** (not locked)
  when their flag (`couchEnabled`, `couchTvEnabled`) is off, so no unreachable card is
  ever shown; already-earned hidden ones still count toward XP so totals cannot drift.
  A test asserts meta targets stay reachable on a flag-disabled install.
- **Couch TV** (plan D12, 6.9; category `tv`, all gated on `couchTvEnabled`): the room's
  20 s accrual writes `user_tv_viewing` (seconds per member, airing and hour of the
  channel's day, in the channel's time zone; programs and breaks only; `airing_id` has no
  foreign key, so XP outlives an airing that leaves with its title) and the counters
  `tv_reactions` (summed) and `tv_audience_max` (high-water mark). `tvFacts` (`tv_store.go`)
  reads it back in one statement, joining `couchtv_airings` for each airing's fill,
  series and time on air: a program is **seen** at half its time on air (at most 30
  minutes) and **watched whole** at 80% of it; a time-of-day window counts on a day with
  15 minutes in it; an airing's day is the one the member started it on. The rules:
  time of day - `tv_prime_time_5` (silver: the prime-time movie, fill `movie`/`premiere`/
  `saga`, seen on 5 days), `tv_night_owl` (bronze: a day with the channel from 02:00 to
  06:00), `tv_morning_5` (bronze: 5 days from 06:00 to 09:00); social - `tv_full_house`
  (silver: watched with 3 others, guests included), `tv_reactions_100` (bronze); loyalty -
  `tv_strip_5` (silver: one strip seen on 5 days in a row, gaps-and-islands per series),
  `tv_marathon` (gold: every marathon episode of a weekend watched whole; a weekend is
  the channel's marathon airings without a two-day gap, planned ones included, so it
  completes with its last episode whatever count the template gives it), `tv_premiere`
  (bronze: a premiere watched whole). Their names avoid the existing badges' (Night Owl,
  Full House, Marathon): Graveyard Shift, Standing Room Only, Weekend Marathoner. Every
  rule is unit-tested at and below its target; `TestRanksCouchTv` records a week of
  viewing as the room does and checks each fact, the XP line and the flag.
- **Evaluation is pull-based**: `POST /me/achievements/check` is the *only* writer of
  `user_achievements`, so **no SQL is added to the 10s progress beacon** and catalog
  needs no changes at all. It is throttled per user to
  one run per 30s (a mutex-guarded map on the module, bounded by the account count),
  and a throttled call returns `rank: null` so the client keeps the rank it has
  rather than painting a zero. `INSERT ... ON CONFLICT DO NOTHING RETURNING` yields
  exactly the rows actually inserted, so two tabs racing produce one celebration
  between them with no read-then-write window. The core's `ranks` module asks once the
  session's user and flags are in, on the player's progress saves (at most every 5 minutes
  unless forced), forced at the end of a title and after a profile edit (a first avatar is
  a badge), and asks once more when the server throttled a forced check; the web also asks
  (throttled) on every navigation.
- **Privacy**: the `publicProfile` boolean in the existing `users.preferences` jsonb
  (absent means public), written through `PUT /me/preferences` - **zero new write
  surface**. Reads filter with `preferences -> 'publicProfile' IS DISTINCT FROM
  'false'::jsonb`, not a `::boolean` cast, which would throw and take the whole
  leaderboard down if any client ever wrote a non-boolean there. An opted-out member
  is a **404, not a 403** (admins included), so "private" is indistinguishable from
  "no such member"; the owner always sees their own.
- Endpoints (all `RequireAuth` + `flags.RequireRankings`): `GET /me/stats` (your own
  profile, which also gives the nav ring its rank), `GET /users/{username}/profile`,
  `GET /leaderboard?period=all|week|month`,
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
  history. Couch TV's room does the same through `couchtv.TvStatsRecorder` (also
  `*ranks.Store`, wired in `internal/app`): its calls take plain values, so neither
  feature imports the other, and the room states the time on the channel's clock
  (`HubDeps.Settings`). `ranks` imports `catalog` only for the exported `Localize`/`GenreLabel`
  helpers, so profile title names and the favourite-genre label read in the visitor's
  language without restating the translations shape.
- Web (`features/ranks`): `/u/[username]` (hero accented from the member's banner
  via `accentVars`, falling back to the tier colour; markdown bio; 8 stat tiles; a
  53x7 activity heatmap scrolled to today; a 24-slice "when you watch" clock; the XP
  breakdown; most-watched; the achievement grid) and `/leaderboard` (2-1-3 podium in
  1-2-3 DOM order, table, sticky "you are #N"), both read from the core's `profile(username)`
  and `leaderboard({period, metric})` views (the core sorts a board by its metric and rates
  each heatmap day). `/profile` renders the same content with edit affordances off the
  same core screen. `/admin/ranks` holds the admin stats and the XP settings form, on the
  typed client. The nav avatar gains a rank ring + level chip (the core's `rank` view) that
  pulse on level-up (`levelUps`). Unlock celebrations use `svelte-sonner` off-player
  (`AchievementWatcher`) and a bespoke overlay **mounted inside the player wrapper** on
  `/watch` (`AchievementOverlay`) - the root Toaster is a fixed root-layout element and
  would vanish in fullscreen, the same trap `CouchBar` documents; the toasts stand down
  while an overlay is mounted. Whichever shows one takes the core's `celebration`
  (`rank.take()`, which sends `CelebrationDismissed`) and the core moves on to the next;
  the toast is created once the take settles, out of the effect flush, since creating one
  writes sonner's own reactive state and doing that mid-flush corrupts its height
  bookkeeping.
- Native: the Apple apps (`Ranks/`, see "Apple clients") and Android (`feature-ranks`) render
  the same core views and send the same events.

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

### couchtv
Couch TV, one auto-programmed, always-on channel (decisions D1-D32 and the roadmap in
`docs/couch-tv-plan.md`). Built so far: the config and weekly template, the pure schedule
generator, the plan job and the admin operations (Phase 1), the per-title analysis for
promos and the break composition and rendering (Phase 3), the live stream (Phase 4: the
prepare job, the stream under channel grants, tuning in and the guide), the viewer's side
(Phase 5: the room, the guest link, progress and watch time, the core's `couchtv` module and
the web's `/tv`, guide, home card and guest join), the admin tools (Phase 6: the schedule
editor, the template, render health and the title tools) and the bulletins (the weather and
the news headlines, fetched through the day and aired in slots of the breaks). Gated by the
admin `couchTvEnabled` flag, the one flag that is **off** while unset
(`flags.Load` leaves it out of the "missing means on" default; `flags.CouchTvOn` guards the
viewer routes and the stream; the core reads it as `SessionView.features.couchTv`, `false` from a
server that does not send it). The admin settings page toggles it.
- **Config** (`couchtv.CouchTvConfig`, settings key `couchtv`, `DefaultConfig`, `Validate`;
  `LoadConfig` falls back part by part, like `ranks.RankConfig`): `timeZone` (default
  `Europe/Prague`), `breakMinSeconds` (23), `rotation` (`primeRepeatMinDays` 5,
  `primeRepeatFactor` 0.8, `movieGapDays` 2, `rerunGapDays` 14, `premiereWindowDays` 28,
  `fillerMinMinutes` 5), `guestLink` (`enabled`, `createdAt`; the code's hash is stored beside
  the config and never served; `adminUpdateCouchTv` keeps the stored link whatever its body
  says, since the link's own operations change it), `bulletins` (`enabled`, default on, and
  `cities`: 1 to 20 named places in Czechia with `lat`/`lon`, by default the regional capitals
  without Pardubice, whose label would collide with Hradec Králové's on a map, Prague by its
  English name (the weather is English, D39) and the rest by their own; a config stored
  before it gets the default) and `template.slots`. A slot has a `key`, `days`, an optional `start` (HH:MM; an
  `anchor` starts exactly then, a flow slot uses it as the time the block before it fills up
  to; times before the day's first anchor are the night after it), a `fill` (`strip`,
  `marathon`, `reruns`, `movie`, `matinee`, `premiere`, `saga`, `encore`), a `pick` (genre
  preference: `any`, `comedy`, `animation`, `drama`, `family`, matched on English names and
  their Czech labels), a `count` (episodes per day) and pinned `titles`. A weekday's slots in
  template order are its **broadcast day**, from its first anchor to the next day's.
- **Default template**: plan 5.1 written as rules, so it fits any library: morning comedy
  reruns, daytime reruns, the 12:00 matinee (Sunday a family movie), the weekend marathon,
  afternoon reruns, strips at 17:00 and 18:00 (two sitcom strips), 20:15 prime-time movie /
  Saturday premiere / Saga Sunday, a drama strip (Mon-Thu) or another strip (Fri, Sun) or a
  late movie (Sat), late animation reruns, the encore near 01:00 and overnight reruns.
  Strips take the series with the most episodes of their genre, once per template (pinned
  first, then genre strips, any-genre strips, marathons); a marathon with no free series
  shares the biggest of its genre, cursor included. Pinning titles reproduces the owner's
  lineup.
- **The generator** (`couchtv.Plan`, pure, every rule table-tested in `plan_test.go` without a
  database, including a week over a library shaped like production's): anchors start exactly;
  flow starts on the next 5-minute mark after the previous program and the shortest break; a
  block that would overrun the next anchor drops what does not fit (a strip episode longer
  than its slot could ever hold is passed over); reruns fill up to the next slot's time
  (each at least half over by a flow target) taking turns through the pool's series, aired
  episodes outside `rerunGapDays` first; a gap of `fillerMinMinutes` or more before a fixed
  time gets `filler` reruns (or a movie) while one fits. Slack still left there, when it would
  make a break over 5 minutes (`breakTarget`), is spread over the breaks since the last
  program that cannot move (at its anchor, fixed, or opening the day), whole 5-minute steps
  to the shortest break first, so starts stay on the grid and breaks stay around 5 minutes
  (about 3 or 8 with 22-minute episodes). Prime time keeps a movie away for
  max(`primeRepeatMinDays`, `primeRepeatFactor` x movie count) days; matinees rerun movies that
  were in prime time recently, so they take nothing from the rotation; the premiere candidate
  and the next saga film are held back from other slots. Saturday premieres the newest movie
  added within `premiereWindowDays` that never aired (D24); Saga Sunday walks TMDB collections,
  else names sharing a prefix ("Iron Man", "Iron Man 2"), in release order, then the
  franchise shown longest ago (D30); a slot with neither runs the rotation (`fill: movie`).
  Each day draws from a seed of its date, so an unchanged day plans the same. Fixed rows
  (admin, locked, already started) are obstacles the flow fits around, and count toward the
  slot they carry (a locked strip episode leaves the strip's other episodes in place; a
  one-program slot with one does not run again). A program an admin removed (`state =
  replaced`, `PlanInput.Removed`) stays off its broadcast day: rotations and reruns pick
  something else and a strip passes the episode over. A program airs to its credits start (its file's effective `credits`
  marker, read by SQL join; `credits_at_ms` on the airing), else in full (`Media.onAir`); the
  rest of the credits plays in the break's squeeze. A marker that changes reaches the days
  the next plan run replans.
- **Strip cursors are derived**: the next episode is the one after the series' latest strip or
  marathon airing (seasons 1 and up in order, wrapping around), so replanning a day reads the
  state the days before it left; reruns and stand-ins do not move it. `couchtv_strips` holds
  an admin's restart (`next_episode_id` from `effective_from`) until the strip has aired past
  it.
- **Series at random**: a series whose episodes may play in any order (the title editor's
  random playback, `titles.allow_random_playback`, `Series.Random`) keeps its strips and
  marathons but has no cursor. Each pick takes an episode that fits and does not air that day
  already (placed, or fixed later that day): one that never aired, in the day's random order,
  else the one aired longest ago, outside `rerunGapDays` first; one a fixed program airs
  within the gap after it goes last. The day's random order is a hash of the day's seed and
  the episode (`dayState.draw`), so a pick changes only when the episodes it chooses from do,
  never because another choice of the day drew differently. Readiness, removals and fixed
  rows work as for any strip (within 6 h a prepared episode of the series first, else a
  prepared rerun of another series stands in). Its reruns draw from all its episodes, not
  only those that aired, and its strip promo has no foot (the count says "in order").
  Changing the flag in the title editor queues a plan run (`catalog.Store.RandomPlaybackChanged`,
  which `internal/app` points at `Scheduler.Start`), so the days not frozen yet follow it; a
  strip turned back to order goes on after its latest strip airing.
- **Readiness**: a file is ready when `couchtv_prepare` found something in its package the
  channel can air (a `couchtv_media` row). Beyond 48 h it does not matter; within 48 h a free
  choice prefers prepared media inside each rule's tier; within 6 h prepared media wins over
  every tier, an unprepared strip episode or premiere gives way to a prepared stand-in, and
  `couchtv.SwapUnready` swaps already planned rows (the break after a stand-in grows and loses
  its composition). A prepared file's `on_air_ms` (its video cut at the segment boundary nearest
  the credits it was measured for, else its end) is its exact time on air (`Media.Cut`), so the
  rows and the breaks composed for them match what airs.
- **The `couchtv_prepare` job** (`couchtv.Preparer`, one slot): queued at startup, after every
  plan run and every 10 minutes (its payload names its 10-minute slot, and each run queues the
  next one first). For the files of the programs airing within 48 h it queues what the package
  lacks through `library.Prepare` (the `package`: the copied source and every audio rendition;
  a rung of the source's class up to 1080p for anything but 8-bit SDR H.264 up to 1080p, or
  when the copied source has a segment over the 16 s target duration before its cut), at
  priority 10, ahead of the queue's other transcodes, with the trick-play rendition as
  backfill; it leaves queued, running and failed renditions alone, records each file that can
  air in `couchtv_media` (the video it airs and `on_air_ms`; a file airing from a VOD rung, say
  an HEVC source's 720p, is ready while its 1080p rung transcodes); then, under the plan lock, fits
  planned rows to their cut (`FitPrograms`: the break after moves and loses its composition),
  runs the readiness swaps and marks programs `ready` or `planned` and ended airings `aired`.
- **The live stream** (`couchtv.Channel`, served by `couchtv.Live`; plan 6.6): one live HLS
  presentation computed per request from the schedule by `hls.LiveSchedule`. An airing **pins**
  what it airs (`feed`: its parts, each a program from its package, a break's rendition or the
  slate, with its time on air in ms) and its numbering (`origin`: media sequence per rendition,
  discontinuity sequence) the moment it goes on air, chained from the airing before it
  (`LiveOrigin.After` over its items), and becomes `aired`; nothing pinned changes, so every
  request and every restart lists the same playlists. Airings are pinned in order by the first
  playlist request after they start; one whose predecessor never aired (nobody was watching)
  starts a fresh numbering from the clock (two media sequence numbers and one discontinuity per
  second since 2026-01-01, more than a chain ever grows).
  - **Programs** start on their scheduled time (a filler covers a hole before one) and air to
    the video boundary nearest their credits (`credits_at_ms`), else to their end, never past
    the next program's start. The video (`airable`) is the copied source when it is 8-bit SDR
    H.264 up to 1080p whose segments fit the target duration, else the tallest H.264 rung up to
    1080p; the audio per channel language (`cs`, `en`) is the file's own AAC rendition in it
    (the default among several), else the original (the default track, else the first); the
    subtitles per language a full track before a forced one, empty WebVTT where none.
    Separate-language files (model B) are not on the played file's timeline and never air.
  - **Breaks** air from where the program left the air to the next program: the break's
    assembled package (`Store.ReadyBreak`, read with `playback.ReadPackageDir`: the video in
    `source/`, the mix as AAC stereo, aired in both languages), else the evergreen break
    (`Store.Evergreen`) cut to the gap, else a filler. A break's own package shorter than the gap
    airs after a filler, so its up next still counts down into the program; the evergreen break
    has no up next, so a filler follows it.
  - **Bulletin slots** (plan 6.10, D36): a break with slots airs its sections (see the render
    sidecar), a part each, so every slot edge is a discontinuity with a program date; padded and
    cut as a break's package is (`sectionParts`). The parts after an airing's first are reached
    under `../airings/<id>/parts/<n>/` (`getCouchTvAiringPartFile`). As the break goes on air, a
    slot that airs whole binds: it airs the newest rendered bulletin of its kind still valid
    when the slot ends (`Store.ReadyBulletin`, `bulletinPart`: the clip's video and its rendition
    at the slot's loudness, in both languages, `PartBulletin`), else its fallback section. Pinned
    with the feed, a slot never changes, whatever renders later.
  - **Fillers** air where nothing else can (after the evergreen break, for a program that cannot
    air, in a hole in the schedule, and for up to 6 h after the last airing, then the playlists
    answer `off_air`): the standby card (`Store.Standby`, the sidecar's rendered loop), else the
    built-in **slate** (`couchtv.Slate`: a dark 1080p25 frame with "Couch TV", drawtext, plain
    where ffmpeg lacks it, and silent AAC, made once at startup into `cache/couchtv/slate` as one
    2 s segment per track, H.264 without B-frames). Both loop (`couchtv.loop`): copy n is segment
    n mod S of pass n / S retimed by whole passes (`mp4.Retime`), so a stretch of any length is
    one item of whole segments; the last copy is cut (`?keep=`).
  - **Playlists**: a constant `EXT-X-TARGETDURATION:16` (copied sources keep their encoder's
    keyframes, up to 15 s apart in the library, and a rung takes a Pi about as long to encode
    as it plays), a 2 min window, a discontinuity and an `EXT-X-PROGRAM-DATE-TIME` at every
    part, the audio segment that runs past a cut listed as `?keep=<seconds>` and served trimmed
    (`mp4.Trim`). The multivariant has one video variant
    whose `CODECS` (`hls.CoverCodecs`), `RESOLUTION`, `FRAME-RATE` and bandwidths cover what airs
    from the window to 6 h ahead, the fillers included; an audio group `cs`/`en` (`CHANNELS="2"`,
    the default from `?lang=`, else Czech) and a subtitle group `cs`/`en`.
- **Channel grants** (`grant.Channel`, resource `grant.CouchTv`, 6 h; the subject is the user,
  a guest's carries their id in the grant's `Couch` field, which `channelGrant` checks against
  `couchtv.Guests.Allows` on every request (kept for 30 s, cleared when the guests end), so a
  rotated or switched-off link ends a guest's stream at once: 403 `grant_revoked`): the
  `Routes.Channel` group under `/tv/{grant}` (`channelGrant` in `internal/server`), gated by
  the flag. Raw operations:
  `getCouchTvMaster` (`/live/master.m3u8?lang=`), `getCouchTvPlaylist`
  (`/live/{video,audio-<lang>,subs-<lang>}.m3u8`), `getCouchTvAiringFile`
  (`/airings/{airing}/{track}/{file}`: an init section, segment or `<n>.vtt` of what a pinned
  airing airs, served only within 10 minutes of its time on air and only from the renditions
  its feed names, so the grant reaches nothing else of the library), `getCouchTvAiringPartFile`
  (`/airings/{airing}/parts/{part}/{track}/{file}`: the same for the airing's media parts after
  its first, a slotted break's sections and bulletins), `getCouchTvStandbyFile`
  (`/standby/{clip}/{video,audio}/{init.mp4,<n>.m4s}`, a ready standby card only) and
  `getCouchTvSlateFile` (`/slate/{video,audio,subs}/{init.mp4,<n>.m4s,empty.vtt}`). An expired
  grant answers 403
  `grant_expired` (tune in again), any other 403 `invalid_grant`, nothing on air 404 `off_air`.
- Viewer endpoints (signed in, flag-gated, localized): `GET /couchtv/tune` (`tuneCouchTv`: a
  channel grant, `streamUrl`, `serverTime` in ms for the program-date hold, `targetDuration`,
  the stream's `languages`, `now` (a program or a break, absent while nothing is), the `next`
  program and up to four `later`) and `GET /couchtv/guide?from&to` (`getCouchTvGuide`: the
  programs overlapping, at most 8 days, with the channel's time zone and the broadcast `days`
  the stretch overlaps). Listings carry the localized title and episode, the overview, poster
  and backdrop, and the `fill`.
- **Guests** (D23, `couchtv.Guests`): the channel link's code is 24 random bytes whose hash
  is stored in the `couchtv` settings; `POST /admin/couchtv/link` (`adminRotateCouchTvLink`)
  makes a new one, turned on, and answers with its code and page (`/tv/join/<code>`) this
  once; `PUT /admin/couchtv/link` (`adminSetCouchTvLink`, 409 `no_channel_link` before there
  is one) turns it on or off. Rotating or turning it off ends every guest (their rows, their
  sockets, their grants). `POST /tv/join/{code}` (`joinCouchTv`, public, flag-gated, 10 tries a
  minute per address; 404 `no_channel_link`, 400 `invalid_name`) takes the name a guest types
  (1-32 characters) and stores a guest (`couchtv_guests`: a random token, hashed, and the link
  it came through), setting the httpOnly channel cookie `couchverse_tv`, scoped to
  `/api/v1/tv` for 30 days (or, with `delivery=body`, answering with the token for the
  `X-Couch-Tv-Token` header). `GET /tv/tune` (`tuneCouchTvGuest`, 401 `no_channel_guest`
  without a working cookie or token) is tuning in for a guest: the live channel only (no
  "Watch from the start", no guide, no library), under a grant naming them, with an
  `artworkGrant` and the guest. The hourly cleanup forgets guests not seen for 30 days.
- **The room** (`couchtv.Hub`, plan 6.9): an in-memory hub without a host on
  `GET /tv/socket` (`couchTvSocket`, flag-gated; a signed-in viewer by their session, a guest
  by the channel cookie or token; 401 `no_channel_guest` otherwise; cross-origin browser
  upgrades refused). Frames (`couchtv/protocol.go`, published by `couchverse couchtv-schema` as
  `contract/couchtv-protocol.schema.json`, one golden frame per type in
  `contract/fixtures/couchtv`, `go test ./internal/feature/couchtv -update` after an intended
  change): `hello` (`serverTime` in Unix ms, the viewer as everyone sees them, the presence),
  `presence` (the count, each viewer once however many devices, and the first 24 to tune in;
  ids `user-<id>` and `guest-<id>`, guests with the name they typed), `reaction` (relayed to
  everyone, three a second per viewer at most), `schedule_changed` (after a plan run, an
  admin's regenerate and a prepare run's swaps, through `Scheduler.Changed`/
  `Preparer.Changed`, gathered over 2 s: viewers tune in again for now and next) and the
  client's `reaction` and `report` (the program date on screen, every 10 s while playing).
- **Reports** become what was watched (D12): a socket's report counts the stretch since its
  previous one when the picture moved on about as much as the clock did (at most 30 s apart,
  never from the future or long gone); every 20 s the room merges each viewer's stretches
  (two devices showing the same picture count once), maps them onto what aired
  (`Channel.Lineup`, whose parts now carry the airing's title, episode and fill) and records,
  best effort: `tv_watch_time_daily` through `TvWatchRecorder` (`*analytics.Store`, wired in
  `internal/app`; guests and breaks included, with a null user or title), `watch_progress`
  through `ProgressRecorder` (`*catalog.Store`) once a signed-in viewer has watched two minutes
  of a program (finished when they watched to within 30 s of where it left the air), and the
  facts ranks scores through `TvStatsRecorder` (`*ranks.Store`: each signed-in viewer's
  stretches with the airing, the part's kind and the program date on the channel's clock,
  in the zone of the config the room reads through `HubDeps.Settings`; reactions; the most
  viewers watched with). See "ranks".
- **The `couchtv_plan` job** (`couchtv.Scheduler`, one slot): queued at startup, after every
  config save and daily at 04:00 in the channel's zone (its payload names the day, and each
  run queues the next one first, so a failing run cannot end the chain). It keeps the
  broadcast days up to the one a week from now planned; a day that starts within 24 h keeps
  its plan once it has one (frozen); the rest is replanned under an advisory lock and stored
  by diff, so unchanged rows keep their ids; then the swaps. It runs while the flag is off, so
  the schedule can be reviewed first.
- **Break composition** (Phase 3, `couchtv.Compose`, pure, rules in `compose_test.go`): a break
  is filled to the frame (25 fps) with the D17 mix: the credits squeeze (`ident` into the
  short ident, ending on its first frame; `expand` straight into up next when no ident fits),
  the short ident (the full one before a prime-time movie, `movie`/`premiere`/`saga`, in the
  broadcast day's design: the four take turns day by day, D35, and the full idents a long gap
  with nothing to promote repeats in the same break take the designs after it), promos while
  they fit, the lineup when the break ends on the hour and a promo still fits beside it, a slot
  for each bulletin it carries (`BreakInput.Bulletins`: the news, 20 s, then the weather, 15 s;
  the later goes first where the break would keep less than the squeeze, the short ident, a
  short promo and the shortest up next beside them), and up next. A slot holds a fallback of
  exactly its length (`slotFallback`): promos of titles the break does not promote yet (else of
  others), each title once, a promo leaving nothing or a stretch stings can close, closed with
  stings; its parts carry the slot's kind (`Part.Slot`, `Composition.Slots`). Up next
  (6-12 s), the lineup (10-16 s) and stings (any whole number of frames from 5 to 10 s, as few
  as possible, before the lineup and up next first, then between promos) take up the rest. A
  sting holds the whole sonic logo and lets its chord ring out, so it is never shorter than 5 s
  (`stingSpan`); its longest is twice its shortest, so k stings close every length from k times
  5 s on and the ranges overlap: **stings alone fill any gap of 5 s or more**, whatever the
  parts around them (a longer bulletin slot, a promo as long as a trailer), and a promo that
  would leave a sliver under 5 s where nothing flexible can take it up gives way to a smaller
  one (`promoFitting`). The shortest normal break is the squeeze, the short ident (5.2 s) and
  the shortest up next, 21.8 s; from 17.5 s the squeeze `expand`s into up next, from 6 s up
  next stands alone, from 5 s a sting alone; under 5 s nothing fits (the break fails over to
  the evergreen one, and the planner keeps breaks over 23 s). Promos are ranked by a
  fill's weight (premiere 6, movie and saga 2, strip and marathon 1.5, matinee 1, the rest
  0.4) over one plus the days until the title next airs, the next program left out and the
  last three breaks' titles last; a title gets its full promo (rendered body plus an end
  card with the airing phrase) when the body is rendered, a strip promo when it is a strip
  at a fixed time ("Every weekday 18:00", with "2 episodes, in order" under the days for a
  strip of several, left out for a series at random), else a short promo, and a gap longer
  than one round of the candidates starts another after a short ident. Airing phrases (`Today`,
  `Tonight` from 17:00, `Tomorrow`, `This week` + day, `Premiere`, `Saga` and `Marathon` +
  day) mirror the broadcast package's `airing.ts`. The break's sound is one break spec (an
  item per clip that starts a sound, a promo body's being its full promo; the sidecar mixes
  the items' sounds) at the loudness of the program before it (else after it), clamped to
  -23..-16 LUFS, -18 when unmeasured (`couchtv_titles.loudness_lufs`). `ComposeEvergreen` is
  the fallback: the full ident (always journey, so the evergreen break is not composed again
  every day), up to three rendered promo bodies with the evergreen end card, the short ident.
- **The `couchtv_compose` job** (`couchtv.Composer`, app): at startup, after every plan run and
  hourly, it composes every break of the next 24 hours that has no composition (a failed one
  again after 6 h), and keeps the standby card and an evergreen break of the current
  templates. With `bulletins.enabled` a break carries bulletins (`bulletinKinds`, D37): the
  news when it ends on the hour every two hours from 06:00 to midnight (06:00, 08:00 ...
  24:00; the owner chose every two hours over every hour), the weather in the first break ending
  within the hour after 07:00, 12:00 and 17:00 and in the break before a prime-time movie
  (ending 19:00-21:00), each only where a bulletin fetched on the grid at least
  `bulletinLead` (15 minutes) before the break goes on air is still valid when it ends
  (`coverable`, `lastBulletin`: the 06:00 news only in a break on air from 05:45, after the
  day's first fetch); the recipe records each slot (`slots`: kind, `at`, `frames` and the
  loudness its sound is mixed at, the break's in whole LU). It waits until a render sidecar
  has published its templates' versions (settings key `couchtv_render`), since a clip's
  cache key is a hash of its template's version and its spec. A break not on air yet that uses
  a clip rendered from another version of its template
  than the sidecar's is composed again (its own promotions forgotten first), so a changed
  template reaches the breaks composed before it; a clip still pending renders from the
  current template anyway. Specs are checked against `contract/broadcast-spec.schema.json` (embedded, copied by
  `make contract`) and use English names and taglines (D13). Media in specs are localhost URLs
  that say what they are cut from (`/media/v1/artwork/<id>?v=<version>[&size=...]`,
  `/media/v1/frames/<file>/<start ms>_<count>/<n>.jpg`, `/media/v1/audio/<file>/<start
  ms>_<length ms>.flac`, `/media/v1/soundtrack/<catalog>/<track id>/<start ms>_<length
  ms>.flac`), so a replaced image or file, or another soundtrack, changes the key. The squeeze takes the
  program's frames and sound from where it left the air (its backdrop when it airs in full);
  promos take `couchtv_titles` (footage shots `[{mediaFileId, startMs, durationMs, score}]`,
  else stills from the artwork; the bed (`promoBed`): an admin's excerpt of the title's own
  audio, else the soundtrack from the web once its preview is ready, else the analysis'
  excerpt, else none; the mood, else one from the genres). **A title in a spec** (D41) carries
  its `logo` in the language of its name (English, else, without an English name, its base
  language's; else one not tied to a language; none when it has neither or one never measured)
  at `size=1600x800` with the aspect of its stored size, and its `footage`, the analysis' shots
  as the promo body's frame URLs (so the sidecar extracts each once for every screen; none
  without analysis; a shot of a file that is gone or lost its source is left out, the promo
  body's too, as one failed extraction fails a render), on every screen that shows it in
  motion: up next, the squeeze's next card, the lineup's rows, the strip, short and full promos
  and their end cards, never the squeeze's ending, where the program itself plays. Those
  screens render for every break, so they draw the backdrop at `size=w1920`; the promo body,
  rendered once per title, keeps the original its stills crop into (`Candidate.BodyBackdrop`),
  as does the evergreen break. Clips are requested in `couchtv_clips` (`needed_at` the earliest
  air time, `last_used_at` touched) and their renders queued soonest-airing first, to run in
  the quiet hours (01:00-09:00 in the channel's zone) unless what they are for airs within
  3 h, and never later than 3 h before it airs (`renderAt`); missing promo bodies are
  requested for later breaks. Each run ends by timing every queued render again by the first
  break still to air that uses its clip (`jobs.Store.Reschedule`, sooner or later) and
  cancelling those no composition uses any more (a replan moved their break, or it was
  composed again), promo bodies and the standby card aside.
- **The render sidecar** (`couchverse render`, plan 6.7; `couchtv.Renderer`): a separate process
  on the same database and `DATA_DIR` that registers only `couchtv_render`,
  `couchtv_bulletin_render` and `couchtv_assemble` (the app never does), runs one job at a time
  and claims a clip's render only while no `transcode_hls` job runs (`jobs.Runner.Yield`;
  backfill such as the channel's trick-play renditions does not count; a bulletin's render,
  priority 1, ahead of every other job, never waits), at nice 10 and in the idle I/O class
  (the image's entrypoint wraps it in `nice` and `ionice`, so Chromium and every ffmpeg
  inherit them), its encoder on two threads (capture is the bottleneck); it closes Chromium
  whenever no render or assembly is due, so outside the
  quiet hours it holds no browser. It serves the embedded
  broadcast bundle (`backend/broadcast`, `make build`) and the media above on localhost to one
  headless Chromium (chromedp; GPU flags on Linux when `/dev/dri` exists; `--expose-gc`, so a
  full ident's render collects its canvas reads; restarted every 25 jobs) and publishes
  `versions()` at start. Artwork comes at the artwork feature's sizes or at the screens' own,
  made once on the first request (`artwork.Service.Within`): `size=w1920`, a backdrop at most
  1920 wide, and `size=1600x800`, a logo within that box, still transparent around its
  letters. A render extracts its media first (footage frames
  1080p, square pixels, SDR via `playback.SDRFilter`, the source's last frame repeated past
  its end, served `no-store`; stereo FLAC of the default track; a soundtrack's stretch of its
  downloaded preview as stereo FLAC too, since Chromium has no AAC decoder), then per frame `advance(t)`
  (no paint wait: the capture draws a fresh frame itself) and a JPEG q92 capture (a frame
  `isStatic` reuses the last one) piped to libx264 (veryfast, `-tune animation`, CRF 18,
  High@4.0, an IDR every 2 s and at the first frame, BT.709 limited range), then the page's
  `renderAudio()` posted back as a WAV and encoded AAC 192k, and `renderItemAudio()`, the
  clip's sound as a break item (`couchtv.itemSpec`: a promo body's is its full promo; the end
  cards and the standby card make none), kept as `couchtv/sounds/<key>.wav` (`soundKey`: the
  template's version and the spec, a promo's airing replaced by the body's, since a promo
  sounds the same whenever it airs). **A squeeze over the program's
  frames is composited by ffmpeg** (`composite.go`): the sidecar loads it keyed (`program:
  key`), reads `keyRect` for every frame, and the encoder decodes the program straight from
  its source (`-ss` at the frames' start, `fps=25`, cover-cropped to 16:9, SDR, its last frame
  held), scales it to each frame's box (the rectangle grown to even pixels; `scale` and
  `overlay` expressions on the frame's time; padded to the largest box, since the overlay
  keeps the size it started with) and lays it over the picture in 4:2:0 with that box's matte
  as its alpha, which the sidecar draws from the same rectangles (rounded, antialiased by pixel
  coverage, times the opacity) and streams on a second pipe; the picture's BT.601 JPEG becomes
  BT.709 before the blend, so the program's pixels pass through as they aired. Chromium never
  decodes a program frame and nothing is extracted for it. **An assembly mixes the break's
  sound** (`sound.go`): every item of the break spec has its kept sound, rendered in the
  page then and there when it has none (a clip rendered before items' sounds were kept, a
  sound pruned, a promo without its airing in the evergreen break: loaded for its sound
  alone, `load(spec, { sound: true })`, a squeeze keyed; its `soundDuration` checked against
  the item's slot), and `mixBreak` adds them up at their starts in 10 s chunks, the
  ring-out of each under the items after it, cuts the sum to the break's length to the
  sample and normalizes it as the page normalizes a render (a Go port of its BS.1770
  loudness and 4x true peak, `loudness.go`, which agrees with `loudness.ts` to 1e-9 LU on
  the same samples: to the spec's loudness, the true peak at or under -1 dBTP); Chromium
  opens only for a missing sound. It then joins the clips'
  video by stream copy (each clip's init and segments as one fMP4, the concat demuxer at
  exact lengths) and segments both. A break with bulletin slots is assembled in sections
  (`sectionsOf`: the clips around its slots and each slot's fallback, a package each in
  `couchtv/breaks/<id>/<n>`): its items' sounds are mixed per section (`mixSections`), what
  rings past a section's end cut and, at a slot's edge, faded out over 20 ms, the break around
  the slots normalized as one and each slot on its own at the slot's loudness, so a bulletin
  can take a slot's place with nothing sounding across its edges. A bulletin's clip (kinds
  `weather` and `news`) keeps no item sound (it never sounds in a mix) and carries, besides
  its own sound, a rendition at every whole LU a break is mixed at (`audio-l16-aac` to
  `audio-l23-aac`, `encodeSlotSounds`: the page's render measured in Go, turned once per
  level, faded in over 5 ms and out over 20 ms). Clips (`couchtv/clips/<key>`) and breaks
  (`couchtv/breaks/<id>`) are packages like a media file's (`source/` and `audio-1-aac/`
  with `rendition.json`, 6 s segments on the package timeline, plus a `master.m3u8`), read
  with `playback.ReadPackageDir`; the video's length is checked to the frame. The live stream
  reads `Store.ReadyBreak`, `Store.Evergreen` (cut to the gap) and `Store.Standby` (a
  seamless loop).
- **Pruning** (the hourly cleanup, `couchtv.Prune`): an aired break's package after an hour
  (its row after a week), clips nobody asked for in 14 days unless a waiting break uses them,
  a bulletin's clip, headlines and all, an hour after its bulletin stopped being valid (or
  once the bulletin is gone),
  evergreen breaks older than the newest assembled one, extracted media unused for 14 days,
  items' sounds no assembly mixed for 2 days (most are one break's; the shared ones are
  mixed daily), work directories after a day, soundtrack previews no title picked and nobody
  played in 14 days, and the raw sound (`sound.f32`, 32-bit float, about 384 KB a second)
  that older renders left in clip packages, a third of their size (a render now writes it
  beside its package and drops it once encoded).
- Endpoints (admin, not flag-gated, like `/admin/ranks`): `GET/PUT /admin/couchtv`
  (`adminGetCouchTv`, `adminUpdateCouchTv`: config plus `plannedUntil`; a save validates
  pinned titles against their slot and queues a replan; a config the planner cannot follow
  is a 400 with a stable code from `couchtv.ConfigError`: `invalid_time_zone`,
  `invalid_rotation`, `invalid_slot`, `anchor_needs_time`, `day_without_slots`,
  `day_needs_anchor`, `slot_out_of_order`, `reruns_without_end`, `invalid_pinned_title`,
  `invalid_bulletins`),
  `GET /admin/couchtv/airings?from&to` (`adminListAirings`, at most 15 days, with title and
  episode names, `fill`, `slot`, `ready` and a break's `render` state: `uncomposed`,
  `composed`, `rendering`, `ready`, `failed`), `POST /admin/couchtv/airings`
  (`adminInsertAiring`, 201 with the airing: a movie or an episode at a time of the admin's
  choosing, an admin row with `fill` `matinee` or `reruns`), `PATCH /admin/couchtv/airings/{id}`
  (`adminUpdateAiring`, `action`: `lock`/`unlock` keep a program through replanning or hand
  it back; `pin` airs another title (`titleId`, `episodeId`) in the row and/or from another
  `startsAt`; `swap` trades programs with `swapWith`, each airing at the other's start;
  `remove` takes it off: the planner's own pick stays as a `replaced` row that keeps its
  program off the day, an admin's is deleted; answers with the days it touched). Every change
  but a lock runs in the planning transaction and replans its broadcast day and the later
  ones (frozen days too: an admin's edit is the change they take), keeping the edited rows'
  ids; a pinned or swapped row keeps its slot and its fill while the title stays (another
  episode of a strip restarts the strip there), else takes the plain rerun of its kind (or
  the rotation's `movie` in a prime-time slot), so it never moves another strip or calls a
  rerun a premiere (`couchtv.adminFill`). A placed program must start after now, within the
  planned week, after what is on air (`on_air`, 409) and a shortest break clear of every
  program that stays (`overlap`, 409); its title must be published (`not_published`), on the
  channel (`excluded`) and playable (`no_media`). The breaks beside a changed program lose
  their compositions (`Store.resetBreaksAround`, also run by the diff for every new program,
  so an up next never announces what no longer airs).
  `POST /admin/couchtv/days/{date}/regenerate` (`adminRegenerateDay`: replans that
  day and the later ones, today to a week ahead, and answers with the day; an operator on
  the server runs the same as `couchverse couchtv-regenerate YYYY-MM-DD`),
  `GET/PUT /admin/couchtv/titles/{id}` (`adminGetCouchTvTitle`, `adminUpdateCouchTvTitle`:
  excluded, mood, promo audio, the soundtrack (`soundtrackId`, one of the title's candidates;
  400 beside `promoAudio`, since promos play one bed), shots; a part left out of the PUT goes
  back to the analysis, which runs again for it, or to the soundtrack finder; leaving or
  rejoining the channel queues a replan; the GET also answers `promoBed`, the `soundtrack`
  pick with its source and readiness, the `soundtracks` found, `soundtrackQuery`,
  `soundtrackSearchedAt` and `searchingSoundtrack`),
  `POST /admin/couchtv/titles/{id}/analyze` (`adminAnalyzeCouchTvTitle`, 202: the title's
  markers and its whole analysis again), `POST /admin/couchtv/titles/{id}/soundtracks/search`
  (`adminSearchCouchTvSoundtracks`, 202, body `{query?}`: the finder's search again, or the
  admin's own words; 409 `excluded`), `GET /admin/couchtv/soundtracks/{id}/preview`
  (`adminCouchTvSoundtrackPreview`, raw: a candidate's preview from the server's copy,
  downloaded on first play, `audio/mp4` or `audio/mpeg`; 502 `soundtrack_unavailable` when the
  catalog does not deliver it) and `GET
  /admin/couchtv/media-files/{id}/waveform?fromMs&toMs&points` (`adminCouchTvWaveform`: the
  promo audio picker's waveform, the loudest momentary loudness per step, at most 15
  minutes a request; `file_missing`/`source_deleted` 404 without the source),
  `GET /admin/couchtv/render` (`adminCouchTvRenderHealth`: when the sidecar last worked, the
  render queue in clips and seconds of video with the measured speed and the time it takes
  against when the first waiting clip airs, the next 24 hours of breaks by state with those
  at risk, the evergreen break and standby card, the latest failures, the disk renders take)
  and `GET /admin/couchtv/bulletins` (`adminGetCouchTvBulletins`, below).
- **Bulletins** (plan 6.10; on air in the breaks' slots, D36-D40): the `couchtv_bulletins` job
  (`couchtv.Bulletins`, app, one slot, priority -5, one attempt) keeps a weather and a news
  bulletin. Each kind is a chain whose run queues the next one first (payload `{kind, at}`):
  news every 20 minutes and weather every hour on a grid from 05:30 to 00:30 in the channel's
  zone (`nextBulletin`), and a run of each at startup that fetches only what went stale while
  the server was down. Every request names the app (`Couchverse/<version> (self-hosted media
  server; Couch TV bulletins)`), gives up after 10 s, is tried twice more after a timeout, a
  dropped connection, a 429 or a 5xx, and asks only for a change where the source sent an
  ETag or Last-Modified (a 304 reuses the copy; `bulletins_fetch.go`). A run that yields a
  bulletin good enough to air adds a `couchtv_bulletins` row (`kind`, `fetched_at`,
  `valid_until`, `data`: the API's JSON); a failing source or too few stories leave the last
  good one. The hourly cleanup keeps six of each kind, and those whose clips may still air,
  but none older than a day (`PruneBulletins`; iROZHLAS lets a reader keep its texts a day).
  With `bulletins.enabled` off the chains run without fetching; they fetch whatever the
  `couchTvEnabled` flag says.
  - **On air** (D38): every run, fetching or not, then asks the sidecar for a clip of the
    newest bulletin of its kind still valid when the next slot of that kind ends
    (`renderForSlot`), but only when it is the kind's last run before that slot's break goes
    on air less `bulletinLead` (15 minutes), so a bulletin is rendered only when it is the one
    that airs (the news, fetched every 20 minutes, once an hour), and only while the channel is
    on and a sidecar has published its templates. The clip's spec is the stored bulletin as
    the broadcast package reads it (`weatherSpec`, `newsSpec`: English chrome, the data's own
    words, D39), its kind `weather` or `news`, its length the slot's; it is requested once per
    bulletin (`couchtv_bulletins.clip_id`) and queued as `couchtv_bulletin_render` at priority
    1, at once. A failed run renders the newest bulletin it has instead.
  - **Weather** (`CouchTvWeather`, valid 6 h, by midnight at the latest since its days are
    named from the day it was fetched, `weatherUntil`): one Open-Meteo `/v1/forecast` request
    for every configured city (now: temperature, WMO code, `is_day`; hourly codes; five days
    of min, max, code and the highest chance of precipitation; the channel's zone, wind in
    m/s), positions
    kept as configured. A day's `code` is its daytime's, 09-18 (`daytimeCode`), since the
    source's daily code is its worst hour: two stormy hours make a stormy day, three wet ones a
    wet day (the most frequent kind), one or two a day of showers, fog in half the hours a foggy
    one, else the dry hours' average cloud cover. Credit "Weather data by Open-Meteo.com".
    `warnings` (absent until ČHMÚ was first read; the last state read stands in while it cannot
    be): the newest SIVS CAP message (`alert_cap_50_*.xml`) by the directory listing's date, its
    yellow, orange and red blocks in force now or later today, one warning per SIVS event and
    level with ČHMÚ's Czech and English names, the earliest onset and latest expiry, and its
    kraje by ISO 3166-2 code from the districts' CISORP codes (`partial` when not all of a
    kraj's districts are warned). Credit "Source: ČHMÚ". The map's outline is embedded as
    `czechia.geojson` (`CzechiaOutline`: geoBoundaries CZE ADM0 from ČÚZK, CC BY 4.0, 643 points;
    credit "Borders: ČÚZK / geoBoundaries (CC BY 4.0)", `OutlineAttribution`), which the
    broadcast package draws from a module generated from it (`npm run outline`, D40).
  - **News** (`CouchTvNews`, valid 3 h, 3 to 5 stories): the RSS of ČT24 and its editors'
    selection, iROZHLAS, Seznam Zprávy, Novinky and iDNES, which may be shown, and of CNN Prima
    and Deník, which only weigh stories, plus ČT24's unofficial trending JSON when it answers;
    ČTK and Economia are never read (their terms). Only RSS's own elements are read (iDNES's
    `media:description` is not the description), in the declared charset; entities are
    resolved inside CDATA too, tags dropped, spaces normalised, an em-dash made an en-dash, and
    links lose `utm_` parameters and fragments. A feed whose channel title lacks its outlet
    (a wrong address answered with a default feed) or that is empty has failed. Items of the
    last 12 hours that are news (not sport, culture, lifestyle, opinion or shows, by category,
    Seznam's sections or the address) are remembered across runs (in memory), since Novinky's
    and iDNES's feeds reach back two or three hours. Stories are average-link clusters (TF-IDF cosine
    over folded five-letter stems of title and perex, at least 0.25, never two items of one
    outlet), scored by their outlets, 1.5 more in ČT24's editors' selection and 1 more when
    trending, halving over a day of age. A story's headline is the first of ČT24, iROZHLAS,
    Seznam Zprávy, Novinky and iDNES to carry it, whole (Český rozhlas forbids shortening or
    rewording) with a no-break space after one-letter prepositions and conjunctions, and
    labelled with the outlet's name; a story only CNN Prima and Deník carry, or one already
    told (an outlet's second article on it), is passed over. `feeds` says how each source
    answered.
  - Tests: `bulletins_weather_test.go`, `bulletins_news_test.go` and `bulletins_test.go` on
    sources recorded on 13 Oct 2026 (`couchtv/testdata/bulletins`, trimmed; the ranking
    reproduces the research's day), `bulletin_spec_test.go` (the clips' specs from them,
    checked against the broadcast schema: English days and warnings, whole headlines, the
    credits), `bulletin_slots_test.go` (which breaks carry slots, their fallbacks, every gap
    filled with its slots whole), `live_slots_test.go` (a slotted break's sections on air, a
    bulletin binding to its slot or not), `TestCouchTvBulletins` (the job on a database over
    them, the answer checked against the spec, the cleanup) and `TestCouchTvBulletinRenders`
    (only the last run in time for a slot renders, once, and none while the channel is off).
- **The `couchtv_analyze` job** (`couchtv.Analyzer`, one slot, low priority, nice'd; plan 6.3):
  per title (published, not excluded), queued a few minutes after its markers change
  (`Analyzer.Queue`, the library hook), by the admin's analyze, and by a backfill at every
  start for titles an older `AnalyzeVersion` (or none) measured. Once per version it
  measures the **loudness** (`loudness_lufs`: a movie's full ebur128 pass, a series' four
  episodes spread over it averaged by energy) and picks **shots** (3-6, 1-3 s in whole 25 fps
  frames: keyframes over 5-50 % of the runtime at 160x90, the most contrasty usable one of
  each stretch, 12 for a movie and 4 in each of three episodes spread over a series, no
  near-black (95th luma percentile under 60), washed-out or flat frames; each decoded for
  3.2 s at 10 fps, the shot ending at the first cut, scored as scdet scores one (threshold
  10), or near-black frame; scored 0.7 motion + 0.3 contrast; the best kept, two at most per
  episode, in airing order). Every run picks the **promo audio** (D25): a series' intro from
  the season whose episodes most often share one (seasons with at least three first, then
  the bigger, then the longer), its median episode; a movie's `credits_music`, else its
  loudest steady stretch of 20 s or more; at most 40 s, the loudest 40 s of a longer one.
  Admin picks (`promo_audio_source`/`shots_source` `admin`) are never overwritten; ffmpeg
  failures leave a part empty and are logged. The excerpt is the fallback bed: a soundtrack
  from the web comes first (below).
- **The `couchtv_soundtrack` job** (`couchtv.Soundtracks`, one slot; plan 6.3 "Promo audio
  from the web"): finds a title's own music in two keyless catalogs with official 30-second
  previews, iTunes Search (`couchtv.ITunes`: AAC in MP4, one request every 4 s, well under its
  ~20 a minute) and Deezer (`couchtv.Deezer`: MP3, one a second; its preview URLs expire within
  the hour, so a download asks for the track again). Queued by a backfill at every start for
  titles an older `SoundtrackVersion` (or none) searched, a few minutes after a title's markers
  change (the library hook, so a new title is searched), by the admin's search, and to ready an
  admin's pick. Each catalog gets an album search "<name> soundtrack" and a track search of the
  same (a series also "<name> theme", a known composer "<composer> <name>"), then lists its two
  albums named most like the title in full. The matcher (`soundtrack_match.go`, pure, replayed
  on recorded answers for the production library in `couchtv/testdata/soundtracks`) compares
  names without release words ("Original Motion Picture Soundtrack", "Main Title", a season or
  volume, a year) by words in order, where a different number is a different work (Iron Man
  is not Iron Man 2) and a game, stage show or, for a series, its movie is another work;
  scores each track by the evidence that it is the title's music (its album named after the
  title, its own name with a theme word, what it says it is from), whether it is the original
  recording (TMDB's composer, the show as the artist, the title's own soundtrack album: a theme
  alone on a single or a compilation is anyone's), the album's year (a film's within a year, a
  series' from its first season), the marks of a cover (karaoke, tribute, piano or epic
  versions, remixes, a list of the usual cover acts) and how it suits a promo (a main title,
  theme or title track first, then an opening, suite or finale, never dialogue, at least
  40 s). Confidence is 0-1; up to 12 candidates are kept (5 per album), the same recording from
  both catalogs once (iTunes on a near tie). TMDB adds the composers and the US and British
  names in one request (`metadata.MusicFacts`, wired in `internal/app`; nothing without a TMDB
  key). The finder's own search stores the candidates in place of the last (a candidate found
  again keeps its id, the pick is always kept) and picks the best from 0.7 unless an admin
  chose; an admin's query only adds choices. The pick's preview is downloaded once into
  `couchtv/soundtracks/<catalog>-<track id>.<ext>` (from the catalog's own hosts only) and
  measured: the excerpt is the preview without its fade in and out (10 LU under its median),
  at most 30 s, at least 12 s; a preview that cannot play drops the candidate and the next
  confident one is picked. A catalog that fails (or a preview that will not download) leaves
  the title as it was, unless the other catalog still found a confident match, and the title
  tries again in 6 h, four times; it never fails the job.
- **Web viewer** (`features/couchtv`, plan 7.2; the core's `couchtv` module does the work and
  `channel.ts` names its screens, sends its events and holds the overlay and guide maths):
  **`/tv`** (`TvPage`, public so a guest watches signed out; a member with the flag off gets a
  404) sends `CouchTvTuneRequested` on mount and `CouchTvClosed` on leaving (or `pagehide`),
  and draws `VideoPlayer` in its live mode (`live`: no seek bar and no pausing, a thin
  progress line of what is on, the channel's audio and subtitle languages as the player's
  menus, a stopped picture resumes at the live edge; the `overlay`, `controls` and `actions`
  snippets) from the `CouchTv` view through `livePlayerView`. `LiveOverlay`: the corner bug
  (the broadcast's: `Lockup`, the package's row `lockupSvg`, a rule, a square steady LIVE
  marker and the clock of the picture's program date, run on from `programDateMs` at
  `programDateAtMs`, over the broadcast's night corner shade; placed where a break burns its own
  into the picture, on the stream's 16:9 frame as the full-viewport player contains it, in
  viewport units since TV browsers may lack container queries, so a cut never moves it) up all
  through a program, and with the controls the Now caption (title, episode, start-end) and the
  Next caption ("Next 22:30 Dexter") above the bar, as the view's cues say, in the brand's look
  (a red label, the title in 800, paper type, over a night shade in their corner; sharp and
  flat, D42: square marks, no shadows), the live player's subtitles lifted over them;
  `PresenceBar` (the reaction button, the main control a TV remote lands on, with five quick
  picks and couch's bundled emoji picker through `EmojiReactionButton`, and the viewers' faces
  opening the list, guests marked);
  `LiveReactions` (everyone's reactions rising up the left side with the sender's name);
  members also get Watch from the start (`/watch/...`, tap-preloaded) and the guide.
  Off air shows when it is back; a signed-out visitor without a working link is sent to sign
  in (`no_channel_guest`); other failures say why with a Retry. **`/tv/guide`** (`GuidePage`,
  `(app)`, flag-gated, core-backed: `CachedView` with `GuideSkeleton`, the core asks again on
  every visit): `GuideContent`, a row per broadcast day ("Today" is the broadcast day on air,
  which runs past midnight), programs placed by time (`guideRows`, 12 rem an hour; a sliver of
  a program drops its padding), a now-line, aired programs dimmed, the one on air
  `aria-current="time"` and the TV's autofocus, "Back to now"; a program opens
  `ProgramSheet` (overview, episode, day and times, a premiere/saga/marathon badge, Tune in
  while it is on, Watch from the start once it began, the title page). **Home**: `OnNowCard`
  leads the rows while the flag is on (`CouchTvOnNow`, through `useScreen`; nothing until the
  core answers, so home never waits): what is on with its progress, what is next, Tune in.
  **The join page** `/tv/join/[code]` (`TvJoinPage`, public): a name of up to 32 characters,
  `CouchTvJoinRequested`, then `/tv`; a member goes straight to `/tv`. `Lockup` (`size`, the
  word's px, and `layout`: a row, or the stack for `ChannelScreen`'s off-air states) imports the
  package's brand module alone, so home never carries the templates or the audio engine; its
  letters are outlines, so it needs no font. TV mode: every control is a real button
  or link, the reaction button and the guide's program on air are `data-tv-autofocus`, the
  quick picks show with focus as well as hover, and with live (as on a TV) an open popover
  keeps the player's controls up. The top nav's TV entry shows while the flag is on. E2E:
  `e2e/couchtv.spec.ts` (a member and a guest, Chromium and WebKit) on the schedule
  `e2e/setup.sql` airs (the series' first episode from its package every 32 s, the slate in the
  breaks), `/tv` in `e2e/a11y.spec.ts`.
- **Web admin** (`features/couchtv`, routes under `/admin/couch-tv`, one sidebar entry and a
  tab bar from `AdminCouchTvLayout`): **Schedule** (`AdminSchedulePage`, `ScheduleGrid`): a week
  of columns from the hour the earliest broadcast day opens, in the channel's time zone
  (`time.ts`, `schedule.ts`: wall-clock maths through `Intl`, clock changes included), programs
  as blocks (title, episode, rule, a lock, a dot for a prepared package, admin rows outlined),
  breaks in a thin lane coloured by render state, a now-line, readiness counts for the next
  48 h; it polls every minute. A program opens `AiringPanel` (details, the breaks on either
  side, lock, air something else, move, swap by clicking the other program, insert before or
  after with `ProgramPicker`, replan the day, remove); a day header replans its day.
  **Template** (`AdminTemplatePage` over `StreamedView`, `GuestLinkPanel`, `TemplateEditor`,
  `SlotEditor`): the guest link first (whether one exists, "Guests can join"
  through `adminSetCouchTvLink`, "Make a link" or, after a warning that it ends the old one
  and its guests, "New link" through `adminRotateCouchTvLink`, the new link shown once with a
  Copy button), then the time zone, shortest break, the rotation knobs and the slots (key, start, anchor, fill, genre,
  count, days, pinned titles through the picker; reorder within the day shown), with
  `FormState` and the server's checks repeated in `template.ts` (`problems`, the same codes,
  shown through `problemMessage`), so Save stays disabled while the template is invalid.
  **Render health** (`AdminRenderHealthPage`): `StatTile`s for the queue, speed, time to clear
  and breaks at risk, a `SegmentBar` of the next 24 h of breaks, the fallbacks, a `RankedList`
  of the disk and the failures; it polls every 10 s. **Preview**: the broadcast preview (the
  bulletins from the feeds recorded on 13 Oct 2026, `preview.ts`). The title editor ends with
  `TitleCouchTvPanel`: never on Couch TV, mood, the promo audio (what
  the promos play once saved and who picked it, "Back to automatic" for an admin's pick of
  either kind): the soundtrack from the web (`SoundtrackPicker`: the candidates with track,
  artist, album, catalog and match, each played and stopped from the server's copy through
  `adminCouchTvSoundtrackPreviewPath`, one chosen with Use, the one in the promos marked, the
  last search's time and query, and a search again with the admin's own words; the panel
  polls while `searchingSoundtrack`), then the excerpt of the title's own audio
  (`PromoAudioPicker` over `Waveform`: a file, five minutes of waveform at a time, a dragged or
  typed selection of up to 40 s within the file, played through the file's playback payload;
  choosing one or the other clears the other, since promos play one bed),
  the footage shots (`ShotsEditor`: stills from `getStreamFramePath` with the payload's grant,
  reorder, remove), the markers per file (`MarkersEditor`: the analysis' and the admin's,
  other times, "has none", back to the analysis; saved on their own) and the analysis re-run;
  a part an admin edits is sent as the admin's pick, "back to the analysis" leaves it out.
  `files.ts` finds the files a title airs as playback picks them; `media.ts` keeps one
  playback payload per movie or episode. E2E: `e2e/couchtv-admin.spec.ts`.
- Tables: `couchtv_airings` (programs and breaks, UTC; `fill`, `slot`, `source`, `locked`,
  `state` (`replaced`: an admin removed it), `credits_at_ms`, and once on air `feed` and
  `origin`), `couchtv_strips`,
  `couchtv_titles` (`excluded`, promo audio, `shots` with `shots_source`, `mood`,
  `loudness_lufs`, `analyzed_version`/`analyzed_at`, the soundtrack pick `soundtrack_id` with
  `soundtrack_source`, its downloaded `soundtrack_path` and excerpt
  `soundtrack_start_ms`/`soundtrack_duration_ms`, `soundtrack_query`, `soundtrack_version`,
  `soundtrack_searched_at`; the planner reads `excluded`, the composer the rest),
  `couchtv_soundtracks` (a title's candidates from the last search: catalog, track id, track,
  artist, album, year, the whole track's length, preview URL, confidence; unique per title,
  catalog and track), `couchtv_media` (what `couchtv_prepare` found airable), `couchtv_clips` (kind, key,
  spec, state, version, duration, path, size, `render_ms`, `needed_at`, `last_used_at`),
  `couchtv_breaks` (an airing's composition or an `evergreen` one: the recipe with its clips,
  frame offsets, sound spec and promoted titles; state, package path, which the live stream
  airs), `couchtv_guests` (the channel link's guests: token hash, name, the link they came
  through, `seen_at`), `couchtv_bulletins` (the weather and news bulletins: kind, when fetched,
  until when they air, the bulletin as JSON, `clip_id` its clip once one was asked for; clip
  kinds `weather` and `news`); `titles` gained `tagline`, `collection_tmdb_id`,
  `collection_name`.
- Tests: `couchtv/plan_test.go` (the rules, airing to the credits), `couchtv/analysis_test.go`
  (promo and shot picks, the waveform), `library/fingerprint_test.go` and `credits_test.go`
  (the detectors, on fingerprints and logs from the production library in
  `library/testdata`), `media/analyze_test.go`, `TestCouchTvPlanJob` (the job on the seeded
  database: a gapless week, the next daily run, a no-op replan, frozen days and a locked row
  kept, a swap), `TestMediaAnalysis` (a generated movie with a crawl, analyzed end to end; a
  season matched from cached production fingerprints; the admin's none),
  `TestCouchTvConfigStored`, `TestApplyMetadataTaglineAndCollection`, `TestCouchTvAiringEdits`
  (the schedule editor on the seeded database: a removal kept off its day, a lock surviving
  a replan, an insert flowed around, moved and removed, a stale composition dropped, gapless
  throughout) and `TestFixedRowsCountTowardTheirSlot`, `TestRemovedPrograms`, `TestAdminFill`
  in `plan_test.go`; series at random in `plan_test.go` (`TestRandomStrips`: a week beside a
  strip in order, never-aired first, no repeat within the gap, the same rows replanned, a draw
  that changes only with its own episodes; `TestRandomOwnerWeek`, `TestRandomStripRepeats`,
  `TestRandomStripReadiness`, `TestRandomStripRemovedAndFixed`, `TestRandomReruns`),
  `TestStripPromoFoot` and `TestCouchTvRandomPlayback` (the flag read by the library, a
  change in the title editor queueing a plan run);
  `couchtv/compose_test.go` (every gap up to half an hour filled exactly with valid specs, the
  mix, promo ranking, clocks, loudness, phrases, the full ident's designs taking turns by
  broadcast day and within a break, the evergreen break, a title's logo and scenes on every
  screen that shows it, the backdrops), `TestCouchTvComposeJob`
  (a week of breaks composed on the seeded database, idempotent, assembly queued as clips
  render), `TestCouchTvComposedTitles` (every composed spec's titles: the logo in the name's
  language, scenes where they move, the backdrops' sizes), `couchtv/mediaserver_test.go` (the
  screens' artwork sizes), `TestCouchTvPrune`, and `TestCouchTvRender` (`COUCHVERSE_RENDER_TEST=1`, Chromium
  from `CHROMIUM_PATH` or installed, the bundle from `make broadcast`): renders a squeeze over a
  generated program (composited, its panel at the hold compared with the program), an ident
  and a sting, each keeping its item's sound, removes one of them, assembles the break (which
  renders that one again) and validates every package and the break's loudness, assembles
  the same clips around a slot (a package per section, the slot's sound at its own loudness)
  and renders a weather and a news bulletin (every slot loudness measured);
  `couchtv/sound_test.go` (the loudness port against `loudness.ts`'s numbers, the mixer:
  offsets, ring-out under the next item, the cut at the break's end, its length to the sample,
  sections around a slot (nothing across its edges, faded there, normalized apart),
  loudness and the true-peak cap, a sound shorter than its slot; the sound keys);
  `couchtv/composite_test.go` (the
  keyed spec, the matte against every pixel's coverage, the graph run in ffmpeg). The
  live stream: `couchtv/live_test.go` (the stream over a fake schedule: the lineup,
  boundaries, cuts at credits, trimmed audio, missing languages and subtitles, origins
  chaining, a restart listing the same playlists, a listed segment never changing, fresh
  numbering, the off-air tail, a program capped by the next; `airable`, `pickAudio`),
  `couchtv/slate_test.go`, `TestCouchTvPrepareJob` (what gets queued, a file made ready, its
  row fitted and the break moved), `TestCouchTvChannel` (the real handlers: tuning in, the
  playlists, trimmed segments, subtitles, the slate, files outside the window or not on air,
  the grant's scope and expiry, the flag), and env-gated `TestCouchTvLive` (`make
  couchtv-live`: generated programs readied by the real jobs, assembled and evergreen breaks,
  the slate and a hole, a break in sections with a bulletin in its slot, looped around now,
  validated around the slot's edges and played by hls.js in Chromium (WebKit too with
  `VIEWERS`) with the program-date hold and by AVFoundation;
  `COUCHVERSE_SPIKE_HOLD=30m` keeps `/watch.html` up to watch it by hand or on a TV; results
  in `docs/couch-tv-plan.md`, Phase 4). The room: `couchtv/hub_test.go` (hello first, presence
  each viewer once, reactions and their limit, reports to watch time, progress and stats on a
  fake lineup, two devices counted once, the end of a program finished, viewers who left, the
  schedule change gathered, a guest's socket ended), `couchtv/protocol_test.go` (the golden
  frames), `TestCouchTvGuests` (the link rotated, a guest joining, tuning in, streaming and in
  the room beside a member, then ended with the link) and the conformance cases. The
  soundtrack finder: `couchtv/soundtrack_test.go` (the production titles' picks replayed from
  the catalogs' recorded answers, each film from its own album and a series' original before
  the covers; what must never be confident (Black Sabbath's "Iron Man", AC/DC's "Iron Man 2",
  cover acts, a game's or a movie's music for a series); the scoring rules on made-up tracks,
  the composer included; names, queries, a catalog down or failing with an error body, preview
  hosts, the excerpt, the bed's priority and a bed cut from a preview as the sidecar does;
  `COUCHVERSE_SOUNDTRACK_PRINT=1` prints every pick, `COUCHVERSE_SOUNDTRACK_LIVE=1` asks the
  catalogs and `COUCHVERSE_SOUNDTRACK_RECORD=1` records them again), `metadata/music_test.go`
  (TMDB's composers and names) and `TestCouchTvSoundtracks` (the job over a fake catalog on the
  seeded database: the backfill, the confident pick readied and measured, a preview served and
  one fetched on first play, the promos' beds, an admin's choice kept through searches and
  handed back, the admin's excerpt first, an outage changing nothing and retrying).

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
- Both surface as `playbackInfo.audio` (source `file`|`embedded`), and a title can mix them: a
  file in English and Czech with a German file beside it lists English, Czech and German (or
  the German file's own tracks), one menu (see playback). The core chooses by the language
  picked last on the device (`player.prefs`), independently of the display language, else
  the played file's default. A track of the source that plays switches inside it
  (`PlayerCommand::SelectAudio` with its place among its file's renditions); a sibling's
  swaps the source for that file's, and the played file's own again swaps back, where playback
  was, playing or paused as it was; a load names the rendition to start on by language and
  place (`PlayerLoad.audioLang`/`audioIndex`), so the second of two tracks in one language
  survives a reload.

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
  `player` (load a source, live or not, play, pause, seek, select audio or subtitles, play at
  a rate, stop; the shell reports back with `PlayerReported`, a live stream's with the
  program date on screen), `socket` (open resolves `socketOpened`, a
  `socketText` per frame and ends with `socketClosed`; send and close), `download` (a
  background transfer into the app's downloads directory by a stable name: `downloadProgress`
  now and then, ending with `downloadFinished` or `downloadFailed`; starting a name already
  transferring attaches to it; cancel and remove), and `render { surfaces }`, which names the
  view models to re-read. Writes, deletes, cancels and renders are fire-and-forget; a late
  answer for a cancelled or forgotten effect is ignored.
- **View models** are plain data per `Surface` (`app`, `servers`, `accounts`, `signIn`,
  `devices`, `pairingApproval`, `session`, `markdown(source)`, `home`, `browse(key)`,
  `title(slug)`, `genres`, `myList`, `search`, `notices`, `rank`, `profile(username)`,
  `leaderboard(key)`, `profileEditor`, `player`, `couch`, `downloads`, `couchTv`,
  `couchTvGuide`, `couchTvOnNow`), each with a `LoadStatus`
  (`idle|loading|loaded|stale|notFound|failed`: stale beats blank) and a `Problem { code }`
  that shells localize. `AppView.phase` (`starting|welcome|signIn|chooseAccount|ready`) picks
  the root screen; a TV always opens on "Who's watching?".
- **Modules** (`src/modules/`): `servers` (address -> `/server` identity, https then http),
  `accounts` (password, pairing with polling and expiry, connect and pair links, approvals,
  devices, tokens; each `AccountCard` carries the account's rank and banner accent as last
  seen on the device, persisted with it for "Who's watching?": the rank from the checks and
  the own profile, none while the server has rankings off, the accent from the own profile,
  the only place the server names it, read once per banner), `session` (user, features,
  display language, accent; a 401 anywhere signs the account out but keeps it), `catalog`
  (stale-while-revalidate home, listings, titles, genres and My List, fresh for 60 s, or 10 s
  for a visit to home, a title or My List, since the viewer's progress and list change on
  their other devices; search with debounce and supersede, warm-start home per account,
  deleted when that account signs out, image URLs per role with the artwork grant; confirmed My List
  changes are numbered and folded into answers requested before them, so a slow refetch never
  undoes one), `ranks` (rank badge and level-ups, throttled achievement
  checks, celebration queue, profiles with the heatmap, leaderboards), `profile` (edits,
  password, avatar/banner uploads), `playback` (the device profile and `resolvePlayback`,
  sources by tier (the server's media paths made whole URLs against the account's server for
  native players), resume, watched-time accounting and progress saves, JIT keepalive,
  preparing poll, qualities, tracks (audio across a title's files), next episode, shuffle; a
  downloaded title plays from the device with source `download` and in-file subtitles),
  `couch` (the socket protocol, reconnects, host broadcast, follower drift sync, remote
  control, reactions; a host reports
  as soon as its socket opens, which takes a session over from the account's other devices,
  and its own view follows what it plays; a `hello` naming a host a remote closes its player
  and makes it the new host's remote; a code that
  names another server than the active account's, or comes without an account, is joined as
  a guest there: `CouchJoinRequested`'s `server` tried as https then http when typed without a
  scheme, the participant token in `X-Couch-Token`, the follower's media and the couch's
  artwork made whole against that server, a follower's preparing poll through the couch; the
  guest's server is forgotten on leaving, at the end and when an account signs in, and the
  account's own playback leaves such a couch first), `downloads`
  (asks the server to prepare an MP4 for the device's profile, polls, fetches it and its
  artwork with the download effect, keeps the account's offline library in the store, plays
  it while `SessionView.offline`, and keeps progress that could not be saved, with the time it
  was watched, until the server answers again; signing out deletes the account's downloads),
  `couchtv` (Couch TV, plan 7.1: `CouchTvTuneRequested` tunes in, signed in or as the guest
  `CouchTvJoinRequested` joined as through the channel link (the channel cookie on the web,
  the guest token in `X-Couch-Tv-Token` natively), and loads the player with the live stream
  (`PlayerLoad.live`: three target durations of latency); the **hold** keeps the picture's
  program date that far behind the server's clock, learnt from the tune-in (the midpoint of
  asking and hearing) and the room's `hello`, against the shell's monotonic clock: a drift
  under 100 ms plays in real time, up to 2 s it plays `1 - drift / 5 s` faster or slower (at
  most 10 %, the S1 spike's hold) through the player's `rate`, beyond that it seeks, and the
  player settles 3 s after a load or a seek. `CouchTvView` says what is on by the picture's
  program date, not the wall clock (the tune-in's now, next and later; a gap between two
  programs is a break), with the overlay cues (the bug and the Now caption over programs, the
  Next caption too once a program follows, none over breaks; shells show the captions with
  their controls),
  presence, reactions, the channel's audio and subtitle languages (`CouchTvAudioChosen`,
  `CouchTvSubtitlesChosen`; kept across tune-ins) and stable problem codes; it renders when what
  is on screen or a cue changes, and the shell runs the clock on from `programDateMs` at
  `programDateAtMs`. The room's socket reconnects with backoff after a tune-in that says the
  channel (and a guest's link) still works, reports the program date every 10 s while the
  picture plays, and reads now and next again on `schedule_changed` or when the picture runs
  past what is known; a failing player is tuned in again and reloaded three times in a row
  before the view fails. `CouchTvGuideView` is today's broadcast day and the ones after it,
  `CouchTvOnNowView` the home card's now and next, asked again when what is on ends; a
  title's `PlayRequested` ("Watch from the start", signed-in viewers only) takes the player
  over from the channel), `notices` (transient toasts), plus pure helpers `theme`
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
  "z" and `wasm-opt -Oz`, and checked against a 440 KB gzip budget). In CI `core.yml` builds
  the wasm package, `apple.yml` the Apple one with the apps, `android.yml` the Android one with
  the Gradle project.

## Apple clients (cross-cutting)

The iPhone/iPad app and the Apple TV app (`clients/apple/`, build guide in `docs/apple.md`) are
SwiftUI shells over the shared core: they render its view models, perform its effects and own
navigation, nothing else.

- **Layout**: `Couchverse.xcodeproj` (folder-synchronized groups; settings in `Config/*.xcconfig`,
  per-builder team and bundle id in the gitignored `Config/Local.xcconfig`) holds two thin app
  targets, their UI test bundles and two app extensions (`CouchverseWidgets`,
  `CouchverseTopShelf`), which go into the apps only with `EXTENSIONS_ENABLED` (see Outside the
  app); the code lives in three local packages. `CouchverseCore`: the xcframework,
  `Generated/Messages.swift`, `CoreRuntime` and the executors (it alone also builds for the Mac,
  for `swift test`), plus `CouchverseShared`, a library without the core for what the apps share
  with their extensions (the shelf snapshot, the `couchverse://` title and play links, the App
  Group container, the Live Activity's content, the logo). `CouchverseDesign`: tokens, typography,
  the accent environment, components, generated strings. `CouchverseFeatures`: screens by feature
  folder (`Onboarding`, `Accounts`, `Settings`, `Home`, `Catalog`, `Player`, `Couch`, `Ranks`,
  `Downloads`) and `CouchverseRoot`, the view both apps show; `LiveRuntime.make()` returns the
  runtime and the `PlayerController` it drives, which the apps put into the environment, and on
  iPhone and iPad the background session downloads run in.
- **Runtime**: `CoreRuntime` (`@Observable`, main actor) is the only stateful service. It stamps
  `nowMs` from the continuous clock, runs one executor per effect (`HTTPExecutor` over an
  ephemeral URLSession, `TimerExecutor`, `SocketExecutor` over `URLSessionWebSocketTask`,
  `KeychainStore` with `AfterFirstUnlock` for tokens, `FileStore` in Application Support on iOS
  and `DefaultsStore` on tvOS, whose only guaranteed storage is user defaults), re-reads just the
  surfaces a `Render` names and publishes them as properties, decoded off the main actor (each
  surface keeps the generation it was published at, so a late batch never wins). Title pages,
  listings, profiles and leaderboards are published per slug, `BrowseKey`, username and
  `LeaderboardKey` once a screen opens them (`core.title(slug)`, `core.browse(key)`,
  `core.profile(username)`, `core.leaderboard(key)`, a loading view before). The runtime counts
  the screens holding each open (their `ScreenOpened`/`ScreenClosed`) and lets one go once none
  has held it for five minutes (`CoreRuntime.keptIdleMs`, checked as screens open and close; one
  the core renders without a screen, a board's other metrics, ages from when it arrives), so
  going back shows it at once and a long session keeps no more than it may go back to. `player`
  commands go to a `PlayerExecuting` (`PlayerController` in the apps, `SilentPlayer` in tests).
  `upload` effects are the `HTTPExecutor`'s: the picked file behind the handle goes as the one
  part of a multipart form, answered as an `http` effect is; a screen makes the handle with
  `UploadFiles` (a photo as a JPEG of at most the slot's size in the temporary directory, its
  file URL the handle; servers take JPEG, PNG and WebP, phones shoot HEIC). `download` effects go
  to a `DownloadExecuting` (`DownloadExecutor` on iPhone and iPad, `NoDownloads` on the TV and by
  default in tests). Screens read `core.<surface>` and `core.send(event)`; their own state is
  presentation only (focus, sheets, a field being typed). `CoreRuntime(fixture:)` shows fixed
  view models for previews and snapshots and records what it is sent. UI tests launch with
  `-uiTesting` (in-memory stores: a fresh install every launch).
- **Root**: `AppView.phase` picks the screen: `welcome`/`signIn` -> `OnboardingFlow` (welcome,
  add server, sign in), `chooseAccount` -> "Who's watching?", `ready` -> `MainTabs` (the TV's
  sidebar: Home, Movies, Series, Genres, My List, Couch, Profile and Leaderboard while rankings
  are on, Settings, Search, under a header with the profile and its rank; on iPhone a Liquid
  Glass tab bar with Home, Browse (movies, series and genres under a segmented control), My List,
  Settings and a search tab, the same tabs as an adaptable sidebar on iPad; each tab its own
  `NavigationStack`). The root also follows the session's display language (`L10n.language`,
  observable, so strings switch without rebuilding the app) and accent, sends `appBecameActive`,
  opens `couchverse://connect`, `couchverse://pair`, `couchverse://couch`,
  `couchverse://title/<slug>` and `couchverse://play/<movie|episode>/<id>` links (`DeepLink`
  decides what to present; the core parses sign-in links, a couch link fills in the join screen,
  title and play links, the same shapes as Android's, become an `OpenRequest`, see Outside the app),
  shows the core's notices as toasts (`NoticeDismissed` when one goes) and achievement unlocks
  (`CelebrationOverlay`), reports the device profile (`CapabilitiesReported`, again when the
  audio route changes) and covers everything with `AppCover`: the player while
  `PlayerView.target` is set (dismissing it sends `PlayerClosed`), or a couch screen standing in
  for it (see Couch).
- **Catalog** (`Catalog/`): Home (the featured hero, 8 s a slide unless a finger or the focus is
  on it, then the server's rows: Continue Watching as backdrop cards with progress that play at
  once, the newest titles and genre rows as posters), Movies, Series and a genre (`BrowseScreen`:
  a poster grid, sort and genre menus that make a new `BrowseKey`, `BrowseMoreRequested` as the
  last rows appear), Genres, My List, Search (`.searchable`, every keystroke a `SearchChanged`,
  the kept query restored) and a title (`TitleScreen`: backdrop, logo, facts, Play or Resume
  from the core's `PlayAction`, My List via `WatchlistChanged`, a random episode with shuffle
  switched on, seasons with each episode's progress as rows on touch devices and a shelf of
  stills on TV; tinted with the title's palette). A screen holds its surface open while it is up
  (`coreScreen`: `ScreenOpened`/`ScreenClosed` bracket the view's task), and home and a title
  also send `RefreshRequested` when the player over them closes, since they stayed open
  meanwhile. `CatalogStateView` shows content whenever there is some (stale beats blank, with a
  note when it could not refresh), else the screen's skeleton, else not found or failed with a
  retry; pull to refresh on touch devices. Cards use the TV's card button style (lift and
  parallax); hover-free focus works with the remote throughout. On iPhone and iPad a title zooms
  out of the poster or backdrop card it was opened from and back into it (`zoomSource` on the
  card, `zoomed(from:)` on the title: the system's zoom navigation transition in a namespace per
  tab, each card named by its shelf since a title can be on two); one opened from a link or a
  button is pushed as usual.
- **Player** (`Player/`): `PlayerController` executes `PlayerCommand`s on one `AVPlayer`: `load`
  (a URL, a file name in the downloads directory for `download`; the start position once the
  item is ready, `maxHeight` as `preferredMaximumResolution`, Now Playing metadata with the
  backdrop as `externalMetadata`), play, pause, seek, audio by place then language (a load's
  too) and subtitles: a sidecar WebVTT file for a progressive source is parsed (`WebVTT`) and
  drawn by the screen, a track inside the media (HLS renditions, a download's own subtitles) is
  selected as a legible option by language (`cze`/`ces`/`cs` alike). It reports
  `PlayerReported` every second while playing and on every change of state, buffering, the end
  and failures, but not while a new item settles at zero; a language picked in the system's own
  menu becomes `SubtitlesChosen` or `AudioChosen` (an audio rendition is named by its place
  among the played file's tracks, which the core lists together). `DeviceCapabilities` measures
  the profile (`VTIsHardwareDecodeSupported`, Main 10 by `isPlayableExtendedMIMEType`,
  `eligibleForHDRPlayback`, the TV's display size and frame rate, a 4K decode cap on phones and
  tablets, Atmos from spatial audio or a multichannel route) and `DeviceProfile.avPlayer`
  shapes it like `contract/fixtures/device-profiles/apple-tv-4k.json`. `PlayerScreen` wraps
  `AVPlayerViewController` (system transport, scrubbing, close, PiP started automatically from
  inline, AirPlay, Now Playing and remote commands; on TV display criteria matching). On TV the
  transport bar gets the core's Quality menu, Audio and Subtitles menus only when the system's
  cannot list them all (another language's file, a sidecar file), and Shuffle; the info panel
  lists the episodes; the next episode is a contextual action with its countdown. On iPhone and
  iPad a small options button between the system's top controls holds the same choices plus
  the episodes, and a glass card counts down to the next episode (Play now, Cancel). Waiting,
  preparing (with its progress), unsupported and failed states cover the player with the
  backdrop. The iOS app has the `audio` background mode for PiP.
- **Downloads** (`Downloads/`, iPhone and iPad, plan 10.8): `DownloadExecutor` runs the core's
  `download` effects as one transfer per file the core names, in a background `URLSession`
  (`BackgroundTransfers`: not discretionary, each task named by its file in `taskDescription`,
  which the system keeps, so a relaunch finds an earlier launch's tasks). A start attaches to a
  running transfer of its name or finishes at once when the file is there (a start without a URL
  only picks one up), progress goes out at most once a second, and a finished file is moved into
  the player's downloads directory (Application Support/Downloads), out of backups, before the
  delegate returns; anything but a 2xx answer is the server refusing. An interrupted transfer
  keeps the system's resume data on disk and continues from it, three times on its own and then
  on the core's next start (resume data the server refuses, an expired grant, starts over from
  the core's latest address); a full disk is reported as `noSpace`. The iOS app hands the
  session's events over in `.backgroundTask(.urlSession(...))` when the system wakes it for a
  transfer that ended while it was suspended or gone. The title page offers each movie and
  episode (`DownloadButton`: a quality menu, then the state from `DownloadsView` as a ring while
  waiting, preparing or coming down, a check when downloaded, a warning when failed, with retry
  and remove), unless `features.downloads` is off and the device keeps none of it.
  `DownloadsScreen` (Settings > Downloads) lists them with state, progress, size and the room
  they take; a finished one plays from the device (`DownloadPlayRequested`, its artwork kept
  beside it showing offline), a failed one is asked for again, any is removed by a swipe or the
  context menu. While `SessionView.offline` the downloads take the place of the tabs, with the
  account switcher. The core polls preparation on a timer, so a download the server finishes
  while the app is in the background starts coming down when the app is next opened. The TV
  keeps none (its storage is purgeable) and shows no download UI. Tested on the host (the
  executor over fake transfers, the downloads directory, a download through the real core) and
  in `DownloadsSnapshots`.
- **Couch** (`Couch/`, plan 10.6): every couch screen reads `core.couch` (`CouchView`). Hosting
  starts in the player: on TV a Couch menu in the transport bar (start, the panel with the code,
  end; a follower's members and leave) beside a reactions menu; on iPhone and iPad the options
  menu (start, the code, end) and, while a session is on, a sofa button (who is there, the panel)
  and a reactions button whose popover offers the recent reactions, then a curated set. The panel
  (`CouchPanel`) shows the host the join page's QR code (`CouchView.shareUrl`, the server's
  `/couch/<code>`) and the six-digit code, large on TV (a translucent cover over the playing
  video, Done focused first) and in a sheet on phones, then who is there (the host first, the
  host stepping away, members paused for themselves dimmed) and End (confirmed: on TV it may be
  the only button in reach) or Leave. Joining (`JoinCouchForm`): Settings' "Join a couch session"
  on every idiom and the TV's Couch tab (`CouchHubScreen`, the live panel while this TV is on a
  couch), a digit pad on TV and the number pad on phones, "Use as a remote" and the VisionKit
  scanner on iPhone; the host's own account pressing Join becomes the player's remote too (the
  server's rule). `couchverse://couch/<code>` links and a scanned join page fill in the code and
  the server they name (`CouchLink.invite`: a link's `?server=`, a join page's origin; the core
  gets a `CouchCode` with both, never the link). With an account the join goes through it on
  its own server and as a guest on any other; without one (the Welcome screen's "Join a couch
  session", or a link naming its server) the form also asks for the server's address and
  joins as a guest, while a link without a server waits for an account. A guest's couch shows
  the same controls as a member's (`CoreRuntime.couchOn`: a live couch counts as couch
  sessions being on). One full-screen cover (`AppCover`) shows the player, a follower waiting
  for the host (choosing, away, connecting), the phone as a remote (`CouchRemoteScreen`: play
  and pause, 10 s skips from the host's extrapolated position, the episodes either side; also
  a host's device once its account hosts on another one, the core having closed its player)
  and, for 3 s, why a session this device watched or steered ended; moving between them never
  presents a modal over one being dismissed.
  Closing a follower's player leaves the couch, or the host's next title would bring it back.
  Over the picture everyone sees reactions rise and fade (Reduce Motion fades them in place) and a
  follower what the host is doing (paused, away, resynced); a follower's own pause or play in the
  system controls (`requiresLinearPlayback` keeps them off the timeline) is told apart from the
  core's commands (`LocalPause`) and becomes `CouchLocalPauseChanged`, so the host's next update
  does not undo it. On TV the player's info panel gains "On the couch" (`CouchInfoPanel`, the
  members updated in place as they come and go) and a follower's has no episodes, whose choice
  is the host's; on iPhone and iPad the reactions popover ends with a "more" button that holds
  the system emoji keyboard up (`EmojiKeyboard`, a hidden field asking for the emoji input mode)
  and sends every emoji typed (`Reactions.emoji`). Logic tests cover links and their servers,
  codes, the status line, the cover's choice (a guest's join without an account), typed emoji,
  the local pause and the transport bar menu; `CouchSnapshots` every couch screen and state, the
  guest's join form among them. The Live Activity is under Outside the app.
- **Ranks** (`Ranks/`, plan Phase 8), absent rather than locked while the server has rankings
  off: a member's profile (`ProfileScreen`: the avatar in the `RankRing` over the banner or a
  glow in its colour, name, handle and joining date, XP towards the next level, the bio through
  `MarkdownView`, stat tiles, the last 26 weeks of the heatmap Monday first (`HeatmapLayout`),
  a 24-wedge watch clock (`WatchClockLayout`), most watched as posters leading to the titles,
  achievements by category with their medal, unlocked first, and the XP sources; your own adds
  Edit profile and, when private, a way back to public; it asks for an achievement check as it
  opens), leaderboards (`LeaderboardScreen`: metric and period switchers, XP always the
  all-time board, a 2-1-3 podium when the top three earned something, rows leading to profiles,
  the viewer's own place kept in view, the hidden notice leading to the editor) and the profile
  editor (`ProfileEditorScreen`: pictures through `PhotosPicker` into the upload effect on
  touch devices, a QR code of the account's web profile on TV, where the text is edited with the
  remote (plan 10.6); name and bio, visibility, password; each save's outcome in words, a
  failure through `Problem.message` when the code has words of its own). Touch devices reach
  them from Settings (your public profile with its rank beside it, the leaderboard, Edit
  profile, which stays with rankings off since a name, picture and password are not
  progression), a TV from its sidebar and Settings. `CelebrationOverlay` shows `RankView`'s
  celebration for 4.5 s (6.5 s and a fade instead of a spring under Reduce Motion), never
  taking the focus, then sends `CelebrationDismissed` for the next; the root shows it over the
  app and `AppCover` inside its cover, which a root overlay cannot reach. Codes become words in
  `RanksWords`.
- **Outside the app** (`Integration/`, plan 10.9). Anything that asks the app to open something
  (a link, a Spotlight result, an intent, a widget or Top Shelf item) leaves an `OpenRequest` in
  `OpenRequests` (a title, play a movie or an episode, the first of Continue Watching, My List, a
  couch code), which the signed-in tabs (`MainTabs`) take once they are up, so a cold launch waits
  for the account and a TV for "Who's watching?": a title opens over Home, play plays from where it
  stopped, Continue Watching waits for the home to settle and plays its first card, My List
  selects its tab, a couch code goes to the join screen. What the app leaves outside itself is the
  **shelf snapshot** (`ShelfSnapshot`, like Android's `widget/continue.json`): the active
  account's Continue Watching and My List with the words the extensions show, already in the
  display language. `Shelf` keeps it from the core's views (emptied once its profile is no longer
  signed in, started over for another one, each part replaced only by a loaded view, so a TV on
  "Who's watching?" keeps it) in the App Group container (`Library/Caches/shelf.json`) when the
  build has one, else in the app's caches; each change reloads the widget or Top Shelf,
  re-indexes Spotlight and updates the App Shortcuts' parameters. **Spotlight** (iPhone and iPad,
  `SpotlightIndex`): Continue Watching then My List, one CoreSpotlight item per title in the
  account's domain, its title link as the identifier, the poster when the image cache already
  holds it; each change
  replaces the account's items, signing out drops them all, and a result opens its title. My List
  is held open while an account is signed in, since the core loads it only while something shows
  it. **App Intents** (the iPhone app's `Intents/`, no capability needed): open a title, continue
  watching (the first title, or a chosen one in progress), open My List, join a couch by code; each
  opens the app and leaves a request through the `OpenRequests` the app registers as an intent
  dependency at launch. Their entities come from the snapshot, so a query needs neither the core
  nor the network; App Shortcuts phrases in English and Czech (`AppShortcuts.xcstrings`), titles
  and descriptions from `contract/i18n` (`intent_` keys, which codegen also writes into the app's
  own catalog: the system reads them from the app bundle). **Extensions**, only with
  `EXTENSIONS_ENABLED` (docs/apple.md): `CouchverseWidgets` holds a Continue Watching widget
  (small and medium, each title a tap from `couchverse://play`) and the couch **Live Activity**
  (the title, the members and the code on the Lock Screen and in the Dynamic Island), which
  `CouchActivities` starts when this device is on a couch, updates while the app runs, renews
  every minute against a 3-minute stale date (local updates only: pushes need a paid team) and
  ends with the session; `CouchverseTopShelf` puts Continue Watching on the Apple TV's Top Shelf
  (backdrops through their artwork grant URLs, play and display actions as links). Extensions
  read the snapshot and never run the core; their own strings (the widget gallery) are the
  `widget_` keys, generated into the extension.
- **Sign-in**: password, or "Sign in with another device" (the code large, a QR code of the
  pairing page and a countdown from `expiresAtMs` on the runtime's clock). A TV shows both side
  by side and starts pairing on its own; a phone shows one at a time. Approving another device
  (Settings, or a link) takes the code typed, from a pair link, or scanned from the TV's QR code
  (VisionKit on iPhone; the core opens a server's `/pair?code=` page as a pair link).
- **"Who's watching?"** (plan 12.3): glass profile tiles, each avatar in its rank ring (`RankRing`
  in the tier's colour, filled to the progress through it) once the device has seen the
  account's rank (`AccountCard.rank`), else in a ring of the profile's own colour. Focus lifts a
  tile, brightens and widens its ring and reveals the rank title above the server; the backdrop
  takes the focused profile's banner accent (`AccountCard.accent`, else its identicon's hue: the
  web's minidenticons, ported in `Identicon`) and cross-fades as the focus moves (with Reduce
  Motion nothing drifts or lifts and the tint fades). On TV, choosing one runs
  `ProfileChoreography`: the others blow outward and the backdrop goes dark, the chosen avatar
  flies from its ring into the sidebar header (its frame reported by `ProfileAvatar`) while home
  rises from black behind it; with Reduce Motion it is a cross-fade. Phones show the rank under
  each name and switch from a sheet (`AccountSwitcherSheet`, with a haptic tick and a cross-fade
  to the new account), whose rows, like Settings', put the ring around each avatar
  (`AccountAvatar`). At the accessibility text sizes the picker shows half as many tiles a row
  and lets the names wrap. A signed-out profile signs in again instead.
- **Accessibility** (plan 10.10, every screen audited in code): VoiceOver labels and values on
  every control (episodes say how far they were watched, skeletons that the page is loading, the
  heatmap and the watch clock are one summary each, couch codes are read digit by digit, rank
  rings and artwork are decoration, and rows that name someone do not repeat their picture),
  headers on section titles, and announcements for what appears away from the focus: problems,
  notices, a save's outcome, a pairing approval, the couch's status over the video and why a
  session ended, a QR code that is not Couchverse's. On touch devices the hero holds still under
  VoiceOver and Switch Control and its dots are an adjustable page control. Dynamic Type: text
  styles throughout, and layouts that wrap or reflow rather than clip at the accessibility sizes
  (Who's watching, the welcome screen, genre tiles, leaderboard rows with the podium stepping
  aside, the next-episode card, the remote's clock); the player's small glass buttons show their
  name in the Large Content Viewer. Reduce Motion: `motion(_:value:)` eases layout changes rather
  than springing them, and transitions fade rather than slide or scale. Reduce Transparency: the
  glass is the system's, which adapts; the TV's couch panel covers the video. Increase Contrast:
  secondary text and lines (`Tokens.Palette.mutedText`, `faintText`, `edgeLine`) brighten
  towards the text colour, and progress tracks and the heatmap's quiet levels stand further
  apart. Sidecar subtitles follow the viewer's caption style (`CaptionStyle`, MediaAccessibility).
  A TV reaches every control with the remote (the devices list's rows take the focus). What
  needs the app running is in docs/apple.md (items 38 to 44).
- **Design**: `primaryAction()` (glass prominent in the accent, the core's contrast-checked
  `onAccent` label; on TV the system's focused label), `secondaryAction()`, `FormField`,
  `AvatarView`, `GlowBackdrop` (radial gradients, no blur), `Skeleton`, `MarkdownView` (the
  core's safe tree), `QRCodeView`, `InsecureBadge`, `ProblemBanner` (every `Problem.code` in
  words, announced as it appears), `RankRing` (the tier's arc around an avatar with an optional
  level chip, flashing once on a level-up), `motion(_:value:)` (an animation that eases instead of
  springing under Reduce Motion) and the contrast-aware `mutedText`, `faintText` and `edgeLine`.
  `Ambience` (`live|still|flat`) quiets the decoration for screenshots and snapshots.
- **Logo and icons**: the logo is the web's `logo.svg`. `CouchverseLogo` (in `CouchverseShared`,
  since the widgets draw it too) is its path as a one-colour shape, fitted into its frame; the
  path is written by `scripts/gen-apple-logo.swift` and held against the web's file by
  `LogoTests`. `LogoMark` is the app mark the web and Android show: the logo in the on-accent
  colour across 80 percent of a rounded square of the accent, following the session's accent,
  on the welcome and launch screens. The widget's header and the Live Activity's Lock Screen and
  Dynamic Island glyph draw the logo in the session's accent; the sofa symbol stays where it
  stands for the couch itself (the Couch tab, menus, member counts). The same script draws the
  icons in the default red, whatever a server's accent: on iPhone and iPad the white logo across
  two thirds of `#e50914`, with dark and tinted versions on a clear background the system fills;
  on Apple TV image stacks (a gradient from the accent into the background behind, the logo in
  front for parallax) for the home screen and the App Store, and the Top Shelf images.
- **Tests**: Swift Testing throughout. `CouchverseCore`: the runtime over the real core with
  fake executors (ranks surfaces, an upload round trip, pages no screen holds let go after a
  while), the executors (the multipart form, uploads through a stubbed session, picked photos
  made JPEGs), and the shelf snapshot (its updates from the views, its file format, the links)
  and its Spotlight entries. `CouchverseDesign`: identicons against the web's output,
  localization and Czech plurals, colours, QR, markdown. `CouchverseFeatures`: deep links, open
  requests and the intents' parameters, the Live Activity's content, code input, the countdown,
  the choreography, Who's watching's tints and rank lines, catalog labels and episodes' watched
  state, WebVTT and stream languages, ranks words, the heatmap and clock layouts, achievement
  groups, the device profile against the contract fixture (in `CouchverseCore`), and
  `ScreenSnapshots` (swift-snapshot-testing) of every key screen and load state on iPhone, iPad
  and TV, English and Czech at the largest Dynamic Type (Who's watching and the switcher sheet
  also with rank rings and a banner's tint), on the pinned simulators `make apple-sims` creates
  (the suite refuses a simulator of another screen scale). UI tests: a smoke test per app, and
  `LiveFlowTests` that pair a TV from a phone and browse to a title and play it on both against a
  running server (skipped without `CV_LIVE_SERVER`). `make apple-test`, `make apple-uitest` and
  `make apple-lint` (swift-format) run them; `apple.yml` runs them all in CI.

## Android client (cross-cutting)

One app for phones and Google TV (`clients/android/`, docs/android.md), the TV UI picked at
launch from the UI mode, on the shared core.

- **Runtime** (`core/runtime/`): `CoreRuntime` owns the bridge on one serial thread for the
  process's life, stamps `SystemClock.elapsedRealtime` (and `wallMs` on messages), performs
  every effect (OkHttp for `http`/`upload`/`socket`, coroutine timers, files for `store`,
  Keystore-sealed files for `secureStore`, Media3 for `player`, WorkManager for `download` on
  phones) and publishes one `StateFlow` per surface,
  re-read only when a `render` names it. At launch it reports `CapabilitiesReported` from
  `MediaCodecList`, the display's HDR types and HDMI passthrough (`core/device/`).
- **Screens** read view models (`rememberSurface`, which also opens catalog surfaces) and send
  events; they own navigation and nothing else. Each screen has phone and TV layouts behind
  one entry point, takes plain view models, and is rendered from fixtures in Roborazzi
  screenshots (phone and TV, en and cs) and Compose UI tests. A TV screen names where focus
  starts (`focusOnStart`, or `screenFocus`, which also returns focus to the element that had it
  when the screen was left) and its text fields let the arrows out (`remoteLeavesField`).
- **Root navigation** follows `AppView.phase`; the signed-in screens are keyed by account. A
  TV starts on "Who's watching?" (each avatar in its rank ring, the tier's colour filled to the
  progress through it; focus lifts a tile, brightens the ring, reveals the rank title and
  tints the glow with the account's banner accent, or its identicon's hue without one; the
  chosen avatar flies into the sidebar as a shared element); phones open the last account and
  switch from the profile tab's sheet, which glows in the active account's colour, avatars in
  their rank rings. `couchverse://connect|pair|title` links and
  scanned QR codes (CameraX + ZXing, phones only) go to the core as `LinkOpened`;
  `couchverse://couch/<code>` joins a couch and `couchverse://play/<kind>/<id>` plays (Watch
  Next, the widget). A couch follower is taken to the couch player and a remote to the remote,
  whatever screen was showing; the remote replaces the player of a host whose account went on
  hosting on another device.
- **Playback** (`feature-playback`): `PlaybackEngine` carries out the core's `PlayerCommand`s
  on one ExoPlayer (whole URLs from the core, sidecar or in-stream subtitles, audio by index
  then language, a load's once its tracks are known, `maxHeight` as a track cap, no retries on
  a 4xx) and reports about once a second while playing and on every change, `playing` meaning
  what the viewer asked for. Phone and TV player screens draw `PlayerView`; a
  `MediaSessionService` gives the system controls, phones get picture-in-picture (shrinking
  from where the picture is).
- **Downloads** (phones, `feature-downloads`): unique WorkManager work per file, a foreground
  worker resuming with `Range` and reporting at most once a second (its notification in the
  display language the download was asked in), files in `files/downloads`; the Downloads
  screen replaces the tabs while offline.
- **Couch** (`feature-couch`): host panel with code and QR, joining by code, QR or link,
  follower and remote, reactions, and an ongoing notification with Leave/End on phones.
  Without an account (Welcome's "Join a couch session") the join form also asks for the
  server's address and joins as a guest, with no "Use as a remote"; `couchverse://couch/<code>
  ?server=` links and scanned join pages (their origin) fill in both and pass `server` in
  `CouchCode`, a link without one waiting for an account. The couch controls and the
  navigation to the player follow a live couch, account or not.
- **Ranks** (`feature-ranks`): profiles, leaderboards (with where the viewer stands pinned
  below the board while they are hidden from it or below its first three, as on the web),
  celebrations, the profile editor (photo picker into the `upload` effect), the rank beside
  the profile row in Settings, and the rank ring "Who's watching?" draws. The API's codes
  (tiers, XP sources, achievements) map to strings through explicit tables, which a test checks
  against `contract/`.
- **Outside the app**: Watch Next on Google TV and a Glance widget on phones (under the logo, in
  the session's accent), both from the home's Continue Watching (kept as is until the home has
  loaded, cleared without an account).
- **Releases**: `release.yml` attaches a signed APK to each `v*` release (keystore from
  repository secrets, a debug-signed APK without them); `android.yml` builds the release APK
  through R8 on every change.
- **Design** (`design/`): Material 3 and Compose for TV themes over the tokens, tinted by the
  session's (or a title's) accent palette; spring motion with a cut under "Remove animations";
  the account's display language applied to resources at runtime; Coil for artwork URLs the
  core signs. The logo is `couchverse_logo`, the web's `logo.svg` as a vector drawable:
  `LogoMark` draws the app mark as the web does (the logo in the on-accent colour across 80
  percent of an accent tile, following the session's accent) on the welcome and starting
  screens; the launcher icon (adaptive, with a monochrome layer), the splash and the TV banner
  show it in white on the default red (the banner fading into the background), and the couch
  notification's small icon (its Live Update chip too) is the logo, the downloads' an arrow.

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
- **Channel grants** (6 hours, `tuneCouchTv`) unlock the Couch TV channel under
  `/tv/{grant}/...` (`Routes.Channel`): its playlists, the slate and the files of what airs
  around now, reached through an airing (`airings/{id}/{track}/{file}`) and never by a file
  id, so a guest's grant never reaches the library; a guest's grant also needs the channel
  link they joined through to still work (see couchtv).

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
- **Checks**: `couchverse validate-hls <url>` (rules in the S3 spike; live playlists too),
  `TestPlaybackTiers` (every tier on generated media, validated; with `COUCHVERSE_AVPLAYER=1`
  played by `scripts/avplayer-probe.swift`), `make hls-check|hls-apple|hls-server-check|e2e-playback`.
- **Live** (Couch TV): `hls.LiveSchedule` writes the live playlists of a schedule of items
  from their packages (`playback.ReadPackage`, `playback.ReadRendition` for a break's); the
  channel airs them (couchtv, "The live stream"); `make couchtv-live` plays it in hls.js and
  AVFoundation, `make live-spike` the S1 spike (results in `docs/couch-tv-plan.md`, "Spike
  results: S1").

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
  so every visit catches up with progress and list changes made elsewhere) or
  `preload(screen)`/`preloadListing(key)` (genres, listings: `ScreenOpened`
  then `ScreenClosed`, loading only what the core does not hold as fresh, 60 s). The page
  holds the screen open while mounted (`useScreen` from `lib/core/screen.svelte.ts`:
  `ScreenOpened`/`ScreenClosed`, with `revalidate` it reloads unless its load just did) and
  reads the view model reactively. A load cannot own the open/close pair: hover, TV-focus
  and the player's preloads run loads for pages that never mount, and SvelteKit reuses a
  preload's result without running the load again.
- **Ranks and profiles are core-backed** the same way: a profile's and the leaderboard's
  loads `prefetch` their screen (the core holds both fresh for 60 s; one leaderboard payload
  per period serves every metric), and the `(app)` layout prefetches your own profile,
  which keeps the nav's rank ring current. The viewer-side pages keep no cache of their own.
- **Page pattern**: the route shell renders `XxxPage.svelte`, a thin wrapper around
  **`CachedView`** (`lib/components/`), which renders one of its snippets: `content` (the
  cached value paints instantly and updates silently when revalidation lands - stale beats
  blank), `skeleton` (cold visit), `notFound`, or `failed` (a cold visit that could not
  load, with a Retry: `LoadFailed`). Pages pass the view's content (`shown(view)`,
  `TitleView.detail`, `ProfileView.profile`) and its `LoadStatus`. The page body lives in
  `XxxContent.svelte`. Skeletons reuse
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
  player warms `/title/{slug}` on mount so "back" is instant). Playing starts a stream (a
  JIT transcode on the Pi), so the watch page asks the core to play only once it is on
  screen and its load just names the title; links to it still use
  `data-sveltekit-preload-data="tap"` (not the global `hover`), so hovering an episode row
  or continue-watching card does not even fetch its code. The core polls a transcode being
  prepared itself.

## Shared core on the web (cross-cutting)

The web runs the shared client core (see "Shared client core") as wasm for the session, bio
markdown, the viewer's catalog, the player, the couch, ranks and profiles, the profile editor
and the devices list. The admin pages, sign-in, pairing approval and the connect code stay on
the typed client.

- **Runtime** (`lib/core/`): `index.ts` creates the one `core` (`CoreRuntime`,
  `runtime.svelte.ts`) in cookie mode and starts downloading and compiling the wasm
  (`wasm.ts`) as soon as the root layout's module loads. The root `load` awaits
  `core.start()` (the wasm plus the session's first load, alongside the web's preferences);
  the guards (`(app)` and `admin` layouts, login, the rankings pages) `await parent()`, so they
  run with the session known. The runtime stamps `nowMs` from `performance.now()`, performs
  effects through `executor.ts` (`fetch` with the cookie, keepalive for everything but GETs
  so what the player saves as the tab closes arrives; `localStorage` under `cv.core.`;
  timers; WebSockets, the page's origin picking ws or wss; secure-store reads are empty and
  writes refused, the web keeps no secrets; `upload` sends a picked file as the one part of a
  multipart form, under its own name: the page holds the file by a handle
  (`lib/core/files.ts`, `holdFile`) and the event names the handle, so the core never holds
  bytes) and keeps `app` and `session` in `$state.raw`,
  re-reading only the surfaces a `render` names and reusing the unchanged parts of a view so
  effects reading them stay quiet. `send(event)` settles once every effect it led to has
  finished (timers aside). Every new core first hears `CapabilitiesReported` with the device
  profile (`device-profile.ts`, tested against `contract/fixtures/device-profiles`), so
  playback resolves by POST. Player commands go to the player a page attached
  (`attachPlayer`); those that came while none was, from the last load on, are replayed to
  it.
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
- **Player and couch**: see playback and couch above. The player's state lives in the core
  (`PlayerView`, `CouchView`); the page owns only the element and its chrome (volume,
  fullscreen, picture-in-picture, the subtitle style). In cookie mode the core plays and
  follows a couch without a signed-in session too, for guests on the couch cookie.
- **The live channel**: `ElementPlayer` plays a live load (`PlayerLoad.live`) in hls.js
  wherever it can, Safari included (`liveSyncDuration` the load's latency,
  `liveMaxLatencyDuration` two target durations more), with the channel's subtitle renditions
  as hidden text tracks the page draws like any other (`subtitleDisplay` off), reports
  `programDateMs` (hls.js's `playingDate`, else the native element's `getStartDate()` plus its
  position) and sets `playbackRate` for the core's `rate`. Guests watch on the channel cookie
  without a session, their artwork through the tune-in's grant.
- **Traps**: a Rust panic aborts the wasm instance. The wasm-bindgen glue keeps its instance
  in module scope, so each core evaluates its own copy of the glue; after a trap a new core
  sends `AppStarted` again while the old views stay up, and its sockets are closed. Markdown that trapped it renders as
  plain text from then on; a second trap while restarting gives up.
- **Session**: `features/auth/session.svelte.ts` reads `core.session` (user, flags via
  `features/settings/features.svelte.ts`, language, accent). The login form posts
  `/auth/login` itself and sends `SessionStarted`; logout sends `SignOutRequested`; an admin
  settings save sends `SessionChanged`, and so does a 401 from a web API call
  (the core confirms with its own `/auth/me`); a tab coming back into view sends
  `AppBecameActive`. The session store watches the core's user: when it goes away without a
  sign-out (that confirmation, or a 401 on one of the core's own requests such as the
  catalog's), the web, once the navigation under way has landed, sends session-only routes
  to `/login?next=`; the core forgets the last session's catalog, ranks, profile and devices
  itself. The root layout applies `session.accent` to `:root`
  (`lib/theme.ts` `applyPalette`); scoped accents (`accentVars`) are still derived in TS, the
  same way (a test checks the two agree).
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
  stay up while a player menu (a popover, `data-popover-content`) is open. HLS always goes
  through hls.js on TVs (their browsers claim native HLS, which would lose the quality and
  audio menus). Couch TV's live player has no seek bar: OK only resumes a stopped picture,
  left/right do nothing, and the controls come up on its reaction button
  (`data-tv-autofocus`); the guide's grid is buttons the arrows move between, scrolling the
  timeline.
- **Cards** mirror their hover look with `group-focus-visible:` (ring on the artwork, so
  the anchor drops its own outline); the hero carousel holds its slide while focus is
  inside it (advancing would re-create the focused button).
- Getting it onto a TV: Titan OS apps are hosted URLs registered in the Titan OS Partner
  Portal; DevView on the TV launches unpublished ones, and Chrome DevTools attaches to
  `<tv-ip>:9222` or `:7001` with Debug Mode on (docs.titanos.tv).

## CouchPush uploader (cross-cutting)

`tools/couchpush` is a Windows desktop uploader (Python, PySide6): it probes and encodes on the
PC with ffmpeg (NVENC or libx264) and uploads through the admin API, so the server has no
route of its own for it. Setup, builds and its tests are in its
[README](tools/couchpush/README.md).

- **Sign-in** is a device session, as the native apps have: `signInDevice` with platform
  `desktop`, named "CouchPush on <computer>", the token sent as `Authorization: Bearer` (over a
  plain-HTTP LAN address, the fast path for large uploads, as well as HTTPS) and listed among
  the account's devices, where it can be signed out. "Stay signed in" keeps the token, with
  the server it belongs to so it is never sent to another, and the password in the OS keyring;
  without it the session ends when the app closes.
- **Operations**: `adminListLibrary`, `adminGetTitle` (seasons, episodes, files and
  `audioStreamsByFile`), `adminCreateTitle`, `adminCreateSeason`, `adminCreateEpisode`, the
  chunked upload (`adminCreateUpload`, `adminAppendUpload` in 8 MiB chunks, resuming from a
  409's offset or `adminGetUpload` after a failure, `adminCompleteUpload`, `adminAbortUpload`),
  `adminUpdateMediaFile` (audio language and role), `adminDeleteMediaFile` and
  `adminUploadSubtitle` (embedded text subtitles extracted to WebVTT).
- The queue scans folders and takes dropped files, with filters, retries and an explicit movie
  target picker; title and year matching (Unicode, remakes) refuses ambiguous targets, and a new
  movie's draft title is created before encoding, its id kept across retries.
- Output defaults to 1080p, keeps the aspect ratio and never upscales; NVENC picks its own H.264
  level and falls back to the CPU; compatible HDR sources are tone-mapped to SDR.
- The default audio mode puts one matching track per requested language (`en, cs`) in one MP4,
  which the server's audio package makes switchable in the player. A combined upload becomes
  the primary file and earlier primaries alternates; a replacement deletes an existing file
  only when its whole known language inventory (`audioStreamsByFile`) is covered, so files
  with unknown or other languages stay.

## Broadcast package (cross-cutting)

`clients/broadcast` (`@couchverse/broadcast`) holds Couch TV's cutscenes (docs/couch-tv-plan.md
6.8): TypeScript with no framework and no runtime dependencies, the productionized planning
sample reel (`tools/couch-tv-samples`). The same code runs in two places: the web compiles its
source for the admin **Broadcast preview** (`/admin/couch-tv/preview`, `features/couchtv`,
through the `@couchverse/broadcast` alias in `svelte.config.js`), and the render sidecar loads
the built page `dist/render.html` (`make broadcast`) in headless Chromium. What the admin
approves is what airs.

- **Layout**: `src/core` (the Timeline, curves and springs, `moves.ts`'s whole-piece `motion`
  and `windows` with cuts on frame edges, the accent palette, DOM helpers, artwork framing, a
  seeded PRNG, `geo.ts`'s `czechiaMap`: Czechia's border, its kraje and the weather's cities
  on the stage by one projection), `src/audio` (the graph with per-piece music, sfx, voice and
  reverb buses into glue, limiter and trim, the reverb a generated hall; the synthesized kit;
  `voices.ts`, the house instruments every screen is scored with (see "Brand"); `pitch.ts`,
  note names in Hz; beds with ducking and tape stops; onset detection; BS.1770 loudness and
  true peak; WAV; the `synth:` placeholder beds), `src/brand`
  (everything that is the brand, see "Brand"; a brand redesign replaces this folder only),
  `src/templates` (one module per template, `index.ts` the registry), `src/styles` (the stage's
  stylesheet `broadcast.css`; `promo.css`, the promo's; `bulletins.css`, the bulletins'), `spec.ts`
  (the data contract), `piece.ts` (a loaded clip and the offline audio render), `stage.ts` (a 1920x1080
  stage in a shadow root), `player.ts` (live playback against the audio clock), `render.ts`
  (the sidecar's API). Design tokens come from `src/generated/tokens.ts`, written by `cargo
  xtask codegen`, and Czechia's border from `src/generated/czechia.ts` and its kraje from
  `src/generated/kraje.ts`, both written by `npm run outline` from the server's
  `couchtv/czechia.geojson` and `couchtv/kraje.geojson` (unit tests fail when they drift apart;
  `scripts/kraje.mjs` made the kraje from geoBoundaries' CZE ADM1, ČÚZK, CC BY 4.0,
  simplifying each border between two kraje once so neighbours share it); Plus Jakarta Sans, the brand's one face (OFL, latin
  and latin-ext), is bundled in `src/assets/fonts` and registered by `ensureFonts`.
- **Templates** (`template` in the spec): `ident` (`cut` full 10.52 s in one of four
  `design`s, see "Full idents", or short 5.2 s, their popcorn ending alone), `sting` (gap
  filler, any whole number of frames from 5 to 10 s, one design for every length), `upNext`
  (default 8 s, countdown), `promo` (moods `cinematic`, `grit`, `bright`; `premiere`; `part`
  `body`, `end` or `full`), `evergreenEnd` ("Coming up on Couch TV"), `stripPromo` (moods
  `bright`, `dark`), `shortPromo`, `lineup` (2 to 5 rows), `squeeze` (handover `expand`,
  11.52 s, or `ident`, 10.6 s: the program shrinks onto the couch in its place in the lockup
  and dissolves into it, and its last frame is the short ident's first, pixel for pixel;
  `cursor`, the old name, still renders as `ident`), `standby` ("Technical difficulties",
  loops seamlessly), and the bulletins, sharp screens that hold still between their moves
  (held frames render as one capture), the channel's lockup in the bug's corner (no clock: a
  bulletin binds to whichever slot it airs in, D38): `weather` (15 s by default, in scenes on
  the beat: Czechia's kraje on the night with every configured city's sky and temperature now,
  ČHMÚ's warnings filling the kraje they cover, hatched where they cover part, beside their
  card; tomorrow, as a red front crosses the country west to east and turns each city; the
  next days, a table a red bar sweeps in; under 13 s it skips tomorrow; WMO icons drawn in
  code, `weather-icons.ts`) and `news` (20 s: the top stories one at a time, each headline
  whole with its outlet's name, a red segment per story). A bulletin airs in a slot of its
  own, so its sound starts and ends inside the clip. Copy is English (D13, D39 for the
  bulletins: what the data brings, the city names and the Czech headlines, stays as written);
  every time and day arrives as a preformatted phrase (`Airing`, the bulletins' `asOf`, day
  labels and `BulletinTime`s), so templates do no clock maths.
- **Spec**: `src/spec.ts` is the single source; `npm run schema` writes
  `contract/broadcast-spec.schema.json` (CI fails on drift) for the server's composer. Beds are
  `{url, in, out}` excerpts (WAV, FLAC or Ogg Opus: open-source Chromium has no AAC); footage is
  a list of 25 fps JPEG frame URLs per shot; a title may carry its logo (`logo`: a trimmed PNG
  and its aspect, drawn in place of the name where the title shows big) and scenes of itself
  (`footage`, for the screens that show it in motion; without them they show its artwork);
  `loudness` is the render's target in LUFS
  (default -18). A `break` spec lists clip specs with their start times: the break's sound,
  each clip ringing out under the next over `crossfade`. The sidecar mixes it from items
  rendered one at a time (`renderItemAudio`, kept for every break that has them); the page
  still renders a break spec in one pass, the reference the mixed items are tested against.
- **Promo body and end card**: a promo is one timeline; its body is timeless (rendered once per
  title) and its end card (4 s, from a frame-aligned cut snapped to the bed) carries the airing.
  The body renders with a fixed airing whatever the spec says and the end card keeps a fixed
  place for the airing, so a body clip never depends on when the title airs; the end card's
  timeless layer may enter during the body's last moments. In a break the body's sound is its
  full promo, so the bed runs across the cut: a body's `parts.sound` reaches through its end
  card, and as the end card's words are seen, never heard, a body's item sound is its full
  promo's whatever the airing (without one, the evergreen end card's, it sounds different).
  `evergreenEnd` is the same end card without an airing.
- **Promo look** (2026-10-14, promo version 7 and `evergreenEnd` 6; the screens' design spec
  2.3-2.4 made sharp and flat: right angles only, no shadows or glows, `src/styles/promo.css`):
  the moods are the identity's three lights. Cinematic (dusk): long takes of the title's scenes pushed 6 %, dissolves, quiet
  type low in the frame, then the title over its hero scene (the least used one) with a red rule
  and the meta line. Grit (night): letterboxed to 2.39:1, so scope footage shows whole; hard cuts
  on the beat, night cards with a word (the genre, the year, or `PREMIERE`) over a red rule
  drawn on the beat, the title cut in whole. Bright (day): the brand's red as one field that
  slides across the frame between the taglines, a red block holding the genre over the montage,
  and the title beside a framed scene on the field its logo reads on, never on a box behind it:
  red first (the sheet stays under the logo and the last tagline sinks out of it; yellow and
  gold logos, Simpsons, Invincible, Pulp Fiction), else paper (the sheet leaves to the left and
  uncovers it; dark and outlined logos, Dexter, Iron Man, Futurama, Harry Potter), and a name is
  night type on paper; a logo that reads on neither goes over the title's scene under the
  night grade like the other moods' (`titleCard`). Which field is measured per logo
  (`readsOnField`, below), so a new title needs no setting. The premiere card is night, the
  dusk rising, `Premiere` over a red rule. Type rises line by line out of its own masks. Every
  picture is a take of a shot: footage frame by frame (a long take of a short shot goes on in
  the next one; a shot used again goes on where it was left, shots keyed by their first frame),
  a still's slow move, or, with no artwork, the channel's painted dusk. Where the title shows
  big it is its logo, sized by its aspect to one area within a box (never stretched) and drawn
  straight on the night the grade makes (a logo too dark for it has its colours lifted, "A
  title shown big"), else its name, fitted. The end card: a night column (to x 1008, its edge a
  red rule) and the title's scenes (its `footage`, else the body's, else the backdrop held
  still) close in from both sides as sharp panels; the title rises, the airing's eyebrow in a
  red block, the day and time, the channel tag (`channelTag`) landing in the corner on the
  sonic logo; the evergreen card lands the lockup in the airing's slot after "Coming up". Sound: the title's bed leads, the house only
  punctuates under it (unpitched: air, soft impacts), ducking 8 dB under the tag; grit's bed
  stops a beat before the tag. Without a bed each mood has its own house score in D at 75 bpm
  (cinematic D, B minor, G and the dominant landing on D with the title; grit a heartbeat over a
  low D minor drone, low piano octaves on the cards; bright glass mallets in eighths, felt piano
  on the red fields), each leaving the D to the tag.
- **Render API** (`window.couchtvRender` in `dist/render.html`, 1920x1080, 25 fps):
  `load(spec, { sound })` resolves with `{duration (whole frames), soundDuration (how long its
  sound lasts as a break item, where the next item starts), cues, fps, width, height, version
  (the template's, for the clip cache key), loop, keyed, missing}` once fonts, artwork and
  beds are in (`sound` builds a clip for its sound alone, never shown or painted); `seek(t)`
  resolves once that frame is painted (footage and every image decoded);
  `isStatic(t0, t1)` is true when nothing moves in between, so the renderer can reuse a frame:
  no dynamic value changes, no steady animation starts or stops applying and no animation is
  between two keyframes that differ (with a linear timing, as the whole-piece `motion` and
  `windows` have; any other timing counts as moving all through); `advance(t, from)` is the
  renderer's step: false when the frame at t is static since `from`, else it sets the frame up
  with every image decoded and resolves without waiting for a paint (a screenshot draws a
  fresh frame of its own; waiting drew each frame twice); `keyRect(t)` is the squeeze's
  program rectangle `{x, y, width, height, radius, opacity}` in stage pixels;
  `setLayer('matte' | 'picture')`; `renderAudio()` is a 48 kHz stereo 32-bit float WAV of the
  loaded clip or break, normalized to its loudness with the true peak at or under -1 dBTP
  (`renderAudioBase64()` for CDP, `audioReport()` for the measured loudness and gain);
  `renderItemAudio()` is the loaded clip's sound as an item of a break, as one render of the
  whole break played it: from its start, its buses faded out over 0.3 s from `soundDuration`,
  the hall ringing out after that (up to 4 s, cut once under -120 dBFS), at the engine's level
  (not normalized), placed 0.5 s into its own context like every item after a break's first;
  `versions()` names every template's version (the sidecar publishes it, so the app keys clips
  without a browser). Today's templates keep some motion in nearly every frame (a slow push,
  grain): only the brand's still stretches (the short ident's couch alone and its held lockup,
  about half of it; a sting's holds; the last 1.6 to 2.6 s of the full idents but cinematic,
  whose grain moves to the end) and a few lineup frames are static. The sidecar posts
  `renderAudio()`'s WAV back to its own server rather than moving base64 over CDP (a long
  break's sound is large).
- **Determinism rule**: every frame and every audio sample is a function of t and the spec.
  Animations are paused Web Animations whose start is folded into `delay`, so a frame is the
  browser's own evaluation at `currentTime = t`; values that change (counters, footage frames)
  are pure functions of t (`Timeline.dynamic`); noise and jitter come from seeded generators,
  and every sound cue reseeds by its index, so its noise is the same wherever playback starts;
  nothing reads the wall clock or keeps state between frames; templates build only after fonts,
  artwork and beds are loaded (layout and beat-synced cuts depend on them); images rasterize
  with their frame (`decoding = 'sync'`). Cuts built with `motion`/`windows` sit on frame edges
  (half a frame before the frame nearest their beat), so float error never moves one to the
  neighbouring frame and a 120 bpm cue lands within half a frame of its sound. Two audio renders
  differ only by float noise (under 1e-5). An offline render schedules each sound cue shortly
  before it plays (`renderSounds`, through the context's `suspend`): Chromium pulls every
  connected node every 128 frames, so a break's thousands of voices built up front rendered it
  slower than real time; the samples are the same. Where a sound sits in its context is not
  free, though: Chromium quantizes automation times to samples, so the same cue a few frames
  later can move a sample (a voice whose pitch moves comes out in another phase, -30 to -45 dB
  under the sound, as it always did from one break to the next), and the compressors open
  every render clamped down for a few hundred milliseconds, which is why an item renders
  0.5 s into its context.
- **Text fitting** (`src/core/fit.ts`): every piece of spec text (names, taglines, episode
  names, kinetic words) goes through `fitText(el, box)`: the design's size when it fits its box
  (its layout column, or the 96 px title-safe width at the largest scale its animations reach
  while it reads), else shrunk to the role's minimum on one line, else broken at word boundaries
  onto two (big display titles three) balanced lines, and shrunk further only when even that is
  not enough. `breakLines` is the pure part: the widest line as narrow as possible, a break after
  a colon or before "and"/"the" only when it costs little width, never a widow, never a
  one-letter word ending a line. It runs once fonts are in, on what `words`/`chars` build (the
  breaks are `<br>`s), so per-word and per-character animations keep their targets.
- **The screens around the programs** (`templates/onair.ts`: up next, the squeeze, the lineup
  and standby, in the new brand since 2026-10-14): the idents' night, paper type and the brand's
  red as sharp blocks and rules, square and flat (no rounded corners, shadows or glows). Type
  rises into place from behind its own edge and lifts away the same way; a label is a red
  block wiping open from its left edge. A title shown big is its own logo (`titleMark`: every
  logo gets the same area within its box, never stretched, drawn straight on the graded night
  with a dark logo's colours lifted as in "A title shown big"; its name, fitted, when it has
  none or it failed to load), and its picture is scenes from it (`scenes`: its footage shots in
  order, cycling, each at most 3.2 s and dissolving into the next over 0.8 s, under one camera
  pushed as a whole; its backdrop without footage, the channel's dusk plate without artwork).
  `reel` is that schedule as a pure function, so a screen continuing another's picture shows
  the same frame. **Up next**: the next title's scenes pushed from 1.04 to 1.12, its logo or
  name lower left under a red "Up next" block, the episode and meta under it, and the horizon
  line filling red, its red head reaching the start time as the program starts; the title's
  bed with the sonic logo on glass on the three beats before the last, the D ringing out under
  the type lifting away so the sound is over before the program starts (without a bed the
  house's D add 9 pad, let go at the logo, and felt piano). **The squeeze**: the program pulls
  back from the frame into a sharp panel in the dark room (a dolly, 2.0-3.6 s), a hairline rim round
  it; the next program
  arrives beside it as a card playing its scenes under its logo or name, a red "Up next" block
  and the start time; while it holds only the card's scenes move. `expand` pushes into the
  card until it is up next's first frame (its scenes framed and timed as up next frames them,
  `UPNEXT.continued`; a test holds both frames to one golden) while the panel slides out left;
  `ident` clears the room and the brand's collapse shrinks the panel onto the couch. The
  program's music carries on, ducked 6 dB as the card arrives, with air under the pull back,
  a glass E5 as the card settles and a rising air into the push that settles before the last
  frame. **The lineup**: "Later on
  couchtv" over rows on hairlines (names as
  type, the live row's red time and progress, the next row's badge); a red marker walks the
  rows on half beats and each lit row's scenes dissolve and slide in on the right, nothing
  else moving between steps; the house's pad, felt piano eighths and a glass note per step,
  then an open fifth that rings out under the rows lifting away. **Standby**: the couch alone on a
  flat dusk horizon under twinkling square stars, the dusk
  breathing once a loop; its pad's partials make whole cycles per loop under a low-pass that
  opens and closes with the dusk, and the sonic logo opens each loop on the felt piano, so
  picture and sound loop without a seam (tests check both).
- **A title shown big** (`src/templates/reel.ts`, the strip and short promos): its scenes are
  its footage shots cut to the beat (`planCuts`: each cut as near the wanted length as the bed's
  beats, else the house tempo's, allow and never longer than its shot has frames, the shots in
  order and cycled, a shot used again picking up where it stopped), each cover-cropped toward
  its `focus` in a box of its own shown only while it plays (so only the frame on screen
  decodes) and slowly pushed; without footage, its backdrop pushed over the same window. Its
  logo is sized by its aspect to the same area whatever its shape, inside a box (`logoSize`), and
  never sits on a box of any colour: where it cannot read on its field the composition changes,
  not the logo. A logo reads on a field when 55 % of its opaque pixels, or 60 % of its outline,
  are at 3:1 against it at display size (`core/logo.ts`, pure over the pixels; a yellow logo
  outlined in black reads on night by its fill and on paper by its outline). `readsOnField`
  returns the first field of a screen's preference list the logo reads on (null without a logo,
  -1 on none), and the screen sets its whole field to it: the bright promo's title red, then
  paper, else over the picture; the bright strip's panel paper, else night. On a field that is
  night already (the end card's column, the other moods' title over the graded picture, the
  dark strip, the short promo, up next and the squeeze's card), `logoFor` draws the logo as it
  is when it reads; else, when it has colour, once on a canvas with its coloured pixels
  (saturation 0.2 and up) walked in HSL lightness to 4.5:1 and at most 0.78, hue and saturation
  kept (Dexter's dark red lifts to a red that reads on night; greys, blacks and an outline stay
  as drawn, so a logo made for paper is never inverted); else as it is while a quarter of it
  still shows (Harry Potter's metal, whose dark parts melt into the night); else the name in
  type. A logo replaces the name; one that did not load falls back to the name in type.
- **Strip and short promos** (sharp and flat, D41's logos and scenes): the strip (12-20 s) opens
  on the series' scenes for a beat; on the bed's strongest onset near 1 s a flat panel with a
  12 px red bar on its edge slides in over the left half while the scenes slide half as far
  aside (a parallax), and the column types in on it in
  masked rises: the eyebrow in a red block, the logo (else the name), the time at 184 px figure
  by figure on a strong onset, the week (seven sharp 8 px bars, square ends) whose aired days
  fill red on the bed's beats, half a beat apart, each with a glass note climbing D major's
  pentatonic (dark: the felt piano an octave down), and the foot line with a square marker under
  a season's start day. 1.3 s before the channel tag the panel sweeps on across the scenes, its
  red bar crossing them, and the tag lands on the clean field (on paper the lockup's `couch` is
  inked night). Bright is day: a paper panel, night type, the scenes as they are, hard cuts
  (with a logo that does not read on paper, Pulp Fiction's yellow, the panel is night by day,
  its type the night tier, the scenes still undimmed, no dusk: never a plate behind the logo);
  dark is night: a night panel, the scenes at half strength (their own opacity, no layer over
  them), the dusk rising at the panel's foot, dissolves, slower moves. The short promo (7-12 s)
  rushes into the scenes from night (one scene, or two quick cuts, the second on the bed's
  strongest beat before the hit), and on the bed's first hit a 240 px red bar crosses the frame
  in 0.36 s with the title card wiped in behind it: the same scene carrying on under the night,
  the logo (else the name) whole in the middle, settling as the swipe passes and then drifting
  closer, the airing rising into a red block above it half a second later, the tag 3.6 s after
  the hit (no earlier than 4.2 s and no later than 3.3 s before the end). Under a bed the house
  adds only punctuation (air under the moves, a soft landing, the notes, a reversed swell into
  the hit); without one the house plays the music itself (pads, sub, the felt piano on the
  beat; the short's hit an impact with D add 9 blooming and a felted pulse under the card).
- **Squeeze compositing**: the spec's `program` fills the panel with the program's last seconds as
  frames (`frames`: the page draws them; the composer writes this, and the sidecar renders it keyed
  and composites the frames itself, see "couchtv"), a stand-in (`standIn`: the ending title's
  backdrop under a credits roll, the preview), or nothing (`key`): the panel is then transparent
  (the picture shows the background behind it), `keyRect(t)` gives its rectangle per frame (driven
  by the same data as the animation), and the `matte` layer renders white where the program shows
  (its opacity included; square since D42, `radius` 0) on black. An ffmpeg composite places the
  program at `keyRect`, takes its alpha from a matte (the sidecar draws it from `keyRect` rather
  than capturing this layer) and overlays it on the picture; with the `ident` handover the short
  ident's couch is drawn in the picture under the panel, and the panel's fading opacity (the
  matte's) dissolves the program into it, so a standard overlay comes out right. What the page draws
  over the panel's area is covered by the program (the hairline rim round it at the hold sits
  outside). A colour key is avoided on purpose (JPEG capture and 4:2:0 chroma fringe it).
- **Brand** (`src/brand`, D33, D34; design notes in docs/couch-tv-plan.md 6.8; the screens' design
  spec in `clients/broadcast/concepts/variants/screens/DESIGN.md`, local): the Couchverse logo and
  the wordmark `couchtv`, Plus Jakarta Sans only, the token red (#e50914), night (#050505), paper
  (#f4ede4, all type on air) and white (#ffffff, the lockup's `couch`, as the approved idents end);
  no taglines. `glyph.ts` (+ `couchverse-glyph.json`): the logo taken apart into the couch (`COUCH`,
  its backrest dome completed) and the popcorn (`POPCORN`, tilting about `PIVOT`); drawn with the
  popcorn knocked out of the couch along its outer contour (`SILHOUETTE`, `LOGO_GAP` 28.5 logo
  units) they are `LOGO`, the web's `static/logo.svg`, exactly (a golden test rasterizes both); the
  full idents knock it out with a halo round its whole path (`GAP`, 30), as their concepts did.
  `wordmark.ts`, the web's light import: `BRAND` (the colours), `PARTS` (couch, t and v as outlines
  of the bundled face at weight 800, in font units, with the face's advances and the -0.035 em
  tracking as browsers lay the word out; a golden test draws them over the face's own text), `WORD`
  (the advance, ink and baseline in em), `geometry(size, layout)` and
  `lockupSvg({ size, layout, ink, brand, label })`: `stack` (the logo 2.93 sizes wide centred over
  the word, its top 2.33 sizes above the word's line box: the approved ending's 540 px logo at y 222
  over a 184 px word at 650) or `row` (the corner bug's: the logo 36/34 sizes tall, the word's 1 em
  line box sharing its top, the c 0.32 sizes from it; 1.06 tall and 5.63 wide), paths only, so it
  needs no font. `sonic.ts`: the sonic logo, A4 E5 D5 on "couch", "t", "v" (the beat after the
  landing, then half a beat apart at 75 bpm, `BEAT` 0.8 s), the last note held while D add9 (A2 D3
  E4 F#4) settles; `sonicLogo` (the full statement: felt piano, the glass halo an octave up and a
  bowed swell on every note), `sonicSting`, `sonicTag` (compact, over a ducked bed) and `sonicChime`
  (struck glass, the standby card); `popcornBuild`, `popcornFall` and `popcornLanding` (the moment
  before it). A statement that ends its clip is given `until`, the context time its sound has to be
  over (the clip's end less `END`, 0.12 s): its voices' decays and releases shorten to the room
  they have (`ringRoom`, a note's natural ring being 2.8 s), and `sonicRingOut` ends the mix from
  just after the last note, falling at a steady rate (`ringOut`, depth by the room: about 9 dB
  over two seconds, 14 over one and a half; the instruments fall about 10 dB a second on their
  own) and closing to silence over the last 0.2 s, hall included, so the chord dies away instead
  of being faded; all on the house instruments (`src/audio/voices.ts`: one `house(kit, bus)` mix per
  piece, a dry chain and a 3.6 s generated hall (Chromium's convolver gets far slower past that), a
  pads' bus with a sweepable low-pass and a chorus; `piano`, `pad`, `padSweep`, `glass`, `bowed`,
  `sub`, `impact`, `pulse`, `air`, `inhale`, `riser`, `shimmer`, `ringOut`, each at an absolute
  context time with a seed). `ending.ts`: the popcorn ending (the approved motion: the popcorn from
  rest 760 px up over 0.72 s, a 26 px rebound, the frame's 7 px give, the couch pressed 7 px; couch
  rises 40 px on its note, t and v 34 px from 1.2 times their size) on one CPU canvas drawn from t,
  each moving frame averaging up to 32 shutter samples in linear light where something moved, a
  still stretch drawn once (its frame key is its state). `short-ident.ts` (`buildShortIdent`): the
  ending alone, the landing at 1.2 s, couch t v at 2.0, 2.4, 2.8, held to 5.2 (the last note has
  2.4 s to ring out; the clip was 4.52 s, whose chord could only be faded). `sting.ts`: one design
  for 5-10 s (`STING`), the ending placed by `stingTimes(D)` (the couch fades in and the popcorn
  drops, the landing later the longer the sting), the hold taking the rest, the chord ringing out
  inside it (the last note has 1.9 s at 5 s, 2.5 s at 6, 4.7 s at 10). `lockup.ts`: the static
  `lockup`, the corner `bug` (its right edge on x
  1824, its top on y 64, 40 px tall: the row lockup at 34, a rule, a steady square red marker with
  LIVE and the clock in paper; nothing in it moves, so held frames stay still; `scrim` shades the
  corner for a screen whose own grade does not) and `channelTag` (the promo end card's sign-off in
  the tag slot, the row lockup 80 px tall ending on x 1824 and y 1008: the couch fades in, the
  popcorn drops onto it with gravity, its knockout a mask, and lands at `at` on a soft round impact,
  then couch, t and v type in half a beat apart on the compact sonic logo; `scrim` as the bug's).
  `ident.ts` routes the short cut to `buildShortIdent` and the full cut to `buildFullIdent` (below).
  The old C()UCH TV brand (D27: Saira, the bracket wordmark, the brass theme and its kit sounds) is
  deleted. Sharp and flat (D42): no border radius anywhere but the logo's curves and the type, no
  shadows, glows or bevels; corner shades (night radial gradients) keep type legible.
  **For clients** (D14 overlays), the package exports `lockupSvg` (above) and `BRAND`; the
  broadcast's corner bug is drawn with it.
- **Full idents** (`src/brand/idents`, D35): the `ident` template's full cut in four designs
  (`design`, journey by default; `buildFullIdent`, `IDENT_DESIGNS` in the order the channel
  rotates them), each in its own folder with its picture and score, ported from the planning
  harness as approved on 2026-10-13: `journey` (one camera move through a dream panorama: dusk
  dunes, the sun setting into the sea behind the couch, a city lighting up, the stars going out
  like a cinema's lights), `cinematic` (a short film: the couch on a hilltop at dusk, a whip to
  a lit window in town, a push into the room as it goes dark), `morph` (four flat posters,
  night, sunset, space and a stage, each shape becoming the next) and `assemble` (the couch
  built from its geometry: two circles orbiting into the arms, the dome, the slab, the feet).
  Every one ends the same way: the couch alone on black, a popcorn bucket drops onto it
  (gravity, a small rebound, the frame giving) and they are the Couchverse logo (`glyph.ts`: the
  logo split into couch and popcorn, from `couchverse-glyph.json`), then "couch", "t", "v" rise
  in, Plus Jakarta Sans ExtraBold with `tv` in red, on the sonic logo's three notes, A4 E5 D5
  (journey's motif, D5 A5 F#5, and cinematic's, A D F# with its B minor echo at dusk, were
  re-scored to it wherever they state it, with their own instruments and timing). The pictures
  are drawn from t on a 1920x1080 canvas (assemble: sixteen stacked SVG copies); a render (the
  dynamic gets `wait`) averages sub-frames over a half-frame shutter in linear light and
  dithers, so motion blurs like film, and live playback draws one sample; cinematic takes as
  many sub-frames as its fastest drawn point needs (its dusk's layers count only while drawn).
  Each score is generated in Web Audio and scheduled as one cue built before an offline render
  starts, as the approved renders built it: their voices start slow oscillators up to a period
  early, which a cue scheduled shortly before it plays would start late. Journey's ping-pong
  echo is entered through one delay only: Chromium pulls a feedback loop from wherever the graph
  first reaches it and lags the edge into that node by a quantum, so a loop reached from two
  sides lagged its repeats differently from render to render. Modules build nothing for the DOM
  as they load (the web's unit tests import the package in Node). A render reads its canvas back
  once per sub-frame, up to 64 full frames a frame, which V8 left to pile up (1 to 2 GB of array
  buffers within a few frames, past the sidecar's 1 GB): every 64 MB of reads it calls `gc()`
  where the page has one (the sidecar and the golden tests start Chromium with `--expose-gc`),
  which keeps the page's heap and buffers under 140 MB (the renderer under 400 MB, a promo's
  280). Each design renders once per template version: an estimated 4 to 16 minutes on the Pi,
  cinematic 65 to 116 (plan 6.7).
- **Adding a template**: add its spec interface to `spec.ts` and to `ClipSpec`; write
  `src/templates/<name>.ts` exporting a `Template` (`version` and `build(ctx, spec)` returning a
  `Timeline`; only `transform`, `opacity` and `visibility` animate; text from the spec goes in
  as text and is fitted with `fitText`); register it in `templates/index.ts`; add styles to
  `src/styles/broadcast.css`; run `npm run schema`; add a fixture to `test/fixtures/specs.ts`
  and its golden frames to `test/golden/frames.spec.ts`; offer it in the web's
  `features/couchtv/preview.ts`. Bump a template's `version` whenever its picture or sound
  changes.
- **Tests** (`make broadcast-test`, CI `broadcast.yml`): vitest for curves, springs, the
  palette, the timeline, line breaking, loudness, WAV and the schema; Playwright in Chromium
  against the built render page for golden frames (480x270, `test/golden/__goldens__`, refresh
  with `npx playwright test --update-snapshots`; the `-long` fixtures carry the longest names
  and lines), and invariants: text that reads stays inside the title-safe width and on the
  frame, a frame is a function of t, `advance` shows what `seek` shows and skips held frames
  (each full ident too, mid-motion and from where its lockup holds), the package loads without a
  DOM, a promo body does not depend on its airing, the end card continues the body on the next
  frame, the key rectangle follows the panel, the `ident` squeeze ends on the short ident's
  first frame (pixel-equal) and the `expand` one on up next's, the lockup's letters are the
  bundled face's outlines, the couch and the popcorn with its knockout are the logo, the corner
  bug (a full-size golden), two audio renders agree (each full ident's too) and hit their
  loudness, the standby loop has no seam (its last frame is its first, its sound wraps), an
  item's sound is the clip's faded into its ring-out, a promo sounds the same whatever its
  airing, a bulletin's sound starts silent and is over before its last frame (so is every short
  clip's: the stings at 5, 6.2 and 10 s, the short ident, up next, the lineup, the squeeze, the end
  cards, the strip and short promos end in silence, the last 50 ms under -60 dBFS, nothing rings on
  past the clip, and where the house plays alone its level only falls through the last second),
  and items rendered
  alone and mixed sound like the break rendered in one pass (within 0.1 LU item by item, 0.5 LU
  in every 400 ms after the first second, 0.2 dB of true peak); a unit test pins the loudness
  numbers the server's Go port reproduces. Stand-in media (gradient art, title logos, two footage
  shots, a bed) are committed in `test/media` with the script that made them. `npm run dev`
  (port 5190) plays every fixture.

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
couchtv -> {auth, catalog, library, artwork, playback}  (nothing imports couchtv; reads media,
            variant and marker tables by SQL, imports library to queue its marker analysis
            and program preparation; playback's exported package helpers write and read its
            renders and read the programs' packages; reaches analytics, catalog's progress,
            ranks and metadata's TMDB music facts through the TvWatchRecorder,
            ProgressRecorder, TvStatsRecorder and MusicFacts interfaces it declares, wired
            in internal/app)
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
  `CouchStatsRecorder` interfaces it declares (wired in `internal/app`), and nothing
  imports it. The anonymous-stream guard is inverted into `internal/server` so `playback`
  never depends on `couch`.
- `ranks` reads a dozen other features' tables but imports only `auth` (for
  `UserFrom`) and `catalog` (for the exported `Localize`/`GenreLabel`); everything
  else is a SQL join. **Nothing imports `ranks`**, which is what makes the couch
  counters safe: `couch` declares the `CouchStatsRecorder` interface itself and
  `internal/app` hands it `*ranks.Store`; `couchtv`'s `TvStatsRecorder` likewise, with
  plain-valued calls so `ranks` needs no `couchtv` type either. For the same reason the nav rank badge reads
  `GET /me/stats` and is deliberately **not** added to `/auth/me` - that would need
  `auth -> ranks`, and `ranks -> auth` already exists.
- Media markers are library's, but Couch TV picks its promos from them: `couchtv` imports
  `library` (to queue a title's marker analysis) and the other way round is a callback,
  `library.MarkerAnalyzer.Analyzed`, which `internal/app` points at
  `couchtv.Analyzer.Queue` and `couchtv.Soundtracks.Queue`.
- Whether a series may play at random is catalog's, but Couch TV's strips follow it: the
  title editor announces a change through a callback, `catalog.Store.RandomPlaybackChanged`,
  which `internal/app` points at `couchtv.Scheduler.Start` (a plan run).

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
