# Couchverse

A self-hosted, Netflix-style streaming server for your movies, TV series and music —
with a Spotify-like music player and a full management panel. One Go binary, one
Postgres database, runs on anything from a Raspberry Pi with a USB disk to a beefy
home server.

![stack](https://img.shields.io/badge/stack-Go%20·%20SvelteKit%20·%20Postgres%20·%20ffmpeg-8b7cf0)

## Features

**Watching**
- Immersive dark UI with a featured-title marquee, continue-watching and genre rows
- Movies and series with seasons/episodes, resume positions, auto-next-episode
- My List, watch history, full-text search across video and music
- Custom video player: subtitles (side-car VTT), quality menu, keyboard shortcuts
- Subtitle support: upload `.srt`/`.vtt` or automatic extraction of embedded text subs

**Music**
- Spotify-style persistent bottom player that survives navigation
- Albums, artists, playlists (create/reorder), queue with shuffle & repeat
- Cover art from file tags, lock-screen/media-key controls (Media Session API)

**Library management**
- Admin panel: library table with quality badges and bulk actions, user management,
  job queue, storage meters, home-page row editor
- Two ways in: resumable chunked uploads (pause/resume survives disconnects) or
  drop files onto the disk and hit *Scan* (Jellyfin-style)
- Filename parsing (`Show/Season 01/Show S01E01.mkv`, `Movie (2024).mkv`) and
  music tags (ID3/FLAC/MP4) build the catalog automatically
- TMDB integration: search & apply metadata + artwork with one click

**Playback pipeline** (scales with your hardware)
1. **Direct play** — browser-compatible files stream straight from disk (zero CPU)
2. **Copy-remux** — h264 in MKV / AC3 audio is repackaged to HLS automatically (fast everywhere)
3. **Background transcode** — admin-queued quality ladder (1080p/720p/480p)
4. **Instant play (JIT)** — unprepared files transcode live while you watch, with
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

By default media + app data live in a **Docker named volume** — zero setup, no
permission issues, everything goes in through browser uploads. If you want your
media on a real disk folder (so you can also drop files over SMB/SFTP and
re-scan), set in `.env`:

```sh
MEDIA_ROOT=/mnt/usbdisk/couchverse   # the folder on your disk
PUID=1000                            # owner of that folder: `id -u`
PGID=1000                            # group of that folder: `id -g`
```

The container runs as `PUID:PGID`, so they must match the folder's owner —
otherwise writes (uploads, transcodes, artwork) fail with permission denied.
Layout inside `MEDIA_ROOT`:

```
media/movies/   media/series/   media/music/   ← drop files here, then Scan
artwork/  subtitles/  cache/                   ← managed by the app
```

Add users under **Admin → Users** (no public signup). Set a TMDB API key under
**Admin → Settings** for one-click metadata.

### Raspberry Pi notes

- Put `MEDIA_ROOT` **and** the Postgres volume on the external SSD/USB disk — never
  the SD card (write wear, fsync latency). Bind `pgdata` to a disk path in
  `docker-compose.yml` if your root filesystem is an SD card.
- Share `data/media` over SMB/SFTP and use **Jobs & Storage → Scan** after dropping
  files — uploads through the browser work too.
- Keep the transcode ladder at 720p and 1 concurrent job (the defaults). On a Pi 4
  the `h264_v4l2m2m` hardware encoder is detected automatically; a Pi 5 has no
  video encoder, so prefer direct-play-friendly files (h264 mp4/mkv).
- Images are multi-arch: `docker buildx build --platform linux/arm64 .`

### Stronger hardware

With any hardware encoder (Intel QSV/VA-API via `/dev/dri`, NVIDIA NVENC, Apple
VideoToolbox) **instant play** turns on automatically — everything plays immediately,
even 4K HEVC remuxes. For VA-API/QSV inside Docker, pass the device through:

```yaml
services:
  app:
    devices:
      - /dev/dri:/dev/dri
```

Tune ladder, preset, concurrency and instant play under **Admin → Settings**.

## Development

Prereqs: Go 1.24+, Node 22+, Docker, ffmpeg on PATH.

```sh
docker compose up db -d      # postgres on localhost:5432 (DB_PASSWORD=couchverse in dev)
make run-backend             # Go API on :8080 (bootstraps admin/admin)
make run-frontend            # Vite dev server on :5173, proxies /api
make sample-media            # generates test clips covering every pipeline tier
make lint check test         # golangci-lint/vet · svelte-check/eslint/prettier · go test
make build                   # SPA → embed → single binary at backend/bin/couchverse
```

Architecture notes live in `CLAUDE.md`. The frontend is a static SPA embedded into
the Go binary; in production only two containers run: the app and Postgres.
