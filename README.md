# Couchverse

A self-hosted, Netflix-style streaming server for your movies and TV series, with a
full management panel, a web app and native apps for iPhone, iPad, Apple TV, Android and
Google TV. One Go binary, one Postgres database, runs on anything from a Raspberry Pi with a
USB disk to a beefy home server.

![stack](https://img.shields.io/badge/stack-Go%20·%20SvelteKit%20·%20Rust%20core%20·%20SwiftUI%20·%20Compose%20·%20Postgres%20·%20ffmpeg-8b7cf0)

## Features

**Watching**
- Immersive dark UI with a featured-title carousel, continue-watching and genre rows;
  title pages and the player pick up an accent colour from each banner
- Movies and series with seasons/episodes (episode thumbnails), resume positions,
  auto-next-episode
- My List, watch history, full-text search
- **Native apps** for iPhone/iPad and Apple TV (SwiftUI) and for Android phones and Google TV
  (Compose), built from source (see [Native apps](#native-apps)). A TV signs in by showing a
  code and QR that you approve on your phone or the web; "Who's watching?" switches profiles
- **Downloads** on iPhone, iPad and Android: the server prepares an MP4 for the device,
  which plays it offline; progress watched offline syncs back once the server is reachable
- Custom video player: subtitles (side-car VTT), quality menu, picture-in-picture,
  keyboard shortcuts, hover tooltips, and a seek bar with time + frame preview
- Subtitle support: upload `.srt`/`.vtt` or automatic extraction of embedded text subs
- **Couch sessions** - synced watch parties: share a link and friends watch in sync
  (even without an account), with a playful couch of avatars and emoji reactions; the
  host drives playback and everyone follows
- **Ranks & achievements** - watching earns XP and rank tiers (Couch
  Rookie to Couch Legend), 30 achievements pop as you unlock them, and every member
  gets a public profile with their own banner, a markdown bio, a year-long activity
  heatmap, a "when you watch" clock and their most-watched titles. Global leaderboards
  for XP, watch time and achievements, with a one-switch opt-out per
  person; admins get a Ranks page with level distribution, achievement rarity and
  editable XP rates

**Library management**
- Admin panel: library table with quality badges and bulk actions, user management,
  job queue, storage meters and a per-title disk-usage chart, a filterable season/episode
  editor, home-page row editor, and a Ranks page (level distribution, achievement
  rarity, per-member standing, editable XP rates and rank thresholds)
- Media comes in through resumable chunked uploads (pause/resume survives disconnects)
- Filename parsing (`Show/Season 01/Show S01E01.mkv`, `Movie (2024).mkv`) builds the
  catalog automatically
- TMDB integration: search & apply metadata + artwork with one click
- English & Czech interface (flag switcher) and per-title content languages: translate
  title/episode metadata per language, add languages from a searchable picker, or remove a
  language to purge its translations and files

**Playback pipeline** (scales with your hardware; every client reports what it can play,
measured on the device, and gets the cheapest stream that fits)
1. **Direct play** - a file the device plays as is streams straight from disk (zero CPU)
2. **Remux** - the source video copied into fMP4 HLS with its audio as renditions (copied,
   or AAC/E-AC-3 when the device needs it) and subtitles as WebVTT renditions; cheap even on a
   Pi, and how HDR10, HLG, Dolby Vision and Atmos reach an Apple TV
3. **Background transcode** - an H.264 SDR quality ladder (1080p/720p/480p, HDR tone-mapped),
   with an I-frame playlist for scrubbing previews
4. **Instant play (JIT)** - unprepared files transcode live while you watch, with
   seek-anywhere; enabled automatically when a hardware encoder is detected
   (VideoToolbox, NVENC, QSV, VA-API, Raspberry Pi 4's V4L2)

## Quick start (Docker)

```sh
git clone <this repo> couchverse && cd couchverse
cp .env.example .env        # set DB_PASSWORD, ADMIN_USERNAME, ADMIN_PASSWORD, MEDIA_ROOT
docker compose up -d --build
```

Open `http://<host>:8080`, sign in with the admin credentials from `.env`.

### Where media lives

By default media + app data live in a **Docker named volume** - zero setup, no
permission issues, everything goes in through browser uploads. If you want your
media + app data on a real disk folder, set in `.env`:

```sh
MEDIA_ROOT=/mnt/usbdisk/couchverse   # the folder on your disk
PUID=1000                            # owner of that folder: `id -u`
PGID=1000                            # group of that folder: `id -g`
```

The container runs as `PUID:PGID`, so they must match the folder's owner -
otherwise writes (uploads, transcodes, artwork) fail with permission denied.
Layout inside `MEDIA_ROOT`:

```
media/movies/   media/series/   ← uploads are stored here
artwork/  subtitles/  cache/                   ← managed by the app
```

Behind a reverse proxy (Caddy, nginx, Traefik), set `TRUSTED_PROXIES` in `.env` to the
proxy's address or subnet (comma-separated, e.g. `172.18.0.0/16`) so login rate limiting
sees real client addresses. Forwarding headers from anyone else are ignored.

Upgrading from a release with music: music was removed. The upgrade drops the music
catalog from the database, but leaves files on disk - delete `media/music/` inside
`MEDIA_ROOT` (and album art under `artwork/`) by hand if you no longer need them.

Add users under **Admin → Users** (no public signup). Set a TMDB API key under
**Admin → Settings** for one-click metadata. Couch sessions, ranks and downloads can each be
switched off under **Admin → Settings → Features**.

### Native apps

The apps are open source and built from source; nothing is published to an app store.

- **iPhone, iPad and Apple TV**: Xcode 27 and a free Apple ID are enough
  ([docs/apple.md](docs/apple.md)). Free provisioning lasts 7 days, so the apps are rebuilt
  weekly from Xcode.
- **Android and Google TV**: one APK for phones and TVs ([docs/android.md](docs/android.md)),
  built with Gradle or downloaded from a release.

In an app, add your server's address (the app checks it is a Couchverse server and warns
when the connection is not encrypted), then sign in with a password, or on a TV show a code
and approve it from a signed-in phone (scan its QR) or at `http://<host>:8080/pair`. Each
signed-in device appears under your profile's **Devices**, where it can be signed out.

### Raspberry Pi notes

- Put `MEDIA_ROOT` **and** the Postgres volume on the external SSD/USB disk - never
  the SD card (write wear, fsync latency). Bind `pgdata` to a disk path in
  `docker-compose.yml` if your root filesystem is an SD card.
- Media is added through the browser (resumable chunked uploads), so there is no need
  to expose the media folder over SMB/SFTP.
- The remux tier (copying the source video into HLS) is cheap on a Pi and covers most MKV
  files for the native apps; transcodes are what cost CPU.
- Downloads are MP4s the server prepares: "Original" quality copies the video (cheap),
  lower qualities transcode. Turn downloads off under **Admin → Settings → Features** if the
  Pi should never do that work.
- Keep the transcode ladder at 720p and 1 concurrent job (the defaults). On a Pi 4
  start the stack with the hardware-encoder overlay so transcodes use the
  `h264_v4l2m2m` video encoder (~3x realtime) instead of software libx264, which is
  unusably slow on a Pi:

  ```sh
  docker compose -f docker-compose.yml -f docker-compose.pi.yml up -d --build
  ```

  The overlay passes `/dev/video11` (the Pi's H.264 encoder) into the container and
  adds the `video` group. Once the encoder is detected, instant play (JIT) also
  turns on automatically. A Pi 5 has no video encoder, so prefer direct-play-friendly
  files (h264 mp4/mkv) there.
- Images are multi-arch: `docker buildx build --platform linux/arm64 .`

### Stronger hardware

With any hardware encoder (Intel QSV/VA-API via `/dev/dri`, NVIDIA NVENC, Apple
VideoToolbox) **instant play** turns on automatically - everything plays immediately,
even 4K HEVC remuxes. For VA-API/QSV inside Docker, pass the device through:

```yaml
services:
  app:
    devices:
      - /dev/dri:/dev/dri
```

Tune ladder, preset, concurrency and instant play under **Admin → Settings**.

## Development

Prereqs: Go 1.25+, Node 22+, Rust (rustup, with the `wasm32-unknown-unknown` target and
`binaryen` for `wasm-opt`), Docker, ffmpeg on PATH. Xcode 27 and the Android SDK only for the
native apps.

```sh
docker compose up db -d      # postgres on localhost:5432 (DB_PASSWORD=couchverse in dev)
make run-backend             # Go API on :8080 (bootstraps admin/admin)
make run-web                 # Vite dev server on :5173, proxies /api
make sample-media            # generates test clips covering every pipeline tier
make lint check test         # golangci-lint/vet · svelte-check/eslint/prettier/vitest · go test
make core-test               # the shared Rust core: fmt, clippy, scenario tests
make contract                # regenerate the API spec, the typed clients, strings and tokens
make e2e                     # Playwright smoke suite (and the axe accessibility audit)
make apple-test android-test # the native apps' package, snapshot and JVM tests
make build                   # wasm core → SPA → embed → single binary at backend/bin/couchverse
```

### End-to-end smoke suite

`make e2e` builds the app, then `scripts/e2e-server.sh` serves the binary on a throwaway database
it creates on the dev Postgres (the API conformance seed plus `clients/web/e2e/setup.sql`, a
generated one-minute clip and placeholder artwork), and Playwright drives the web client in
Chromium, then WebKit (CI runs Chromium only): sign-in, browsing, My List, playback, a couch
session with an anonymous guest, the admin pages, the language switch and TV mode. The database
and data directory are dropped when the run ends. It needs the compose `db` up and ffmpeg; `psql`
is optional (the script falls back to the one in the `couchverse-db-1` container,
`E2E_PG_CONTAINER` to override).

```sh
cd clients/web
E2E_SKIP_BUILD=1 npx playwright test e2e/couch.spec.ts --project=chromium   # reuse the last build
npx playwright test --ui                                                     # watch it run
npx playwright show-report                                                   # traces of failures
```

Set `E2E_SERVER_LOG=/tmp/e2e-server.log` to keep the server log; CI uploads it with the report.

Architecture notes live in `CLAUDE.md` and `FEATURES.md`; the native clients' design in
`docs/native-clients-plan.md`. Every client runs the same Rust core (`core/`) for its logic -
the native apps through UniFFI, the web as WebAssembly - so sign-in, browsing, playback
decisions, couch sync, ranks and downloads behave the same everywhere. The web client
(`clients/web`) is a static SPA embedded into the Go binary; in production only two
containers run: the app and Postgres.
