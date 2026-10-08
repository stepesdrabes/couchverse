# CouchPush

Desktop tool to transcode movies and TV episodes to **H.264 MP4**
on a Windows PC (RTX 3060 / NVENC) and upload them to a [Couchverse](../../README.md)
instance. Video encoding happens on the PC. One AAC audio track allows direct play;
multiple tracks use CouchVerse's selectable-audio HLS preparation, which copies the
H.264 video and prepares the audio on the server.

## What it does

1. Log in to your Couchverse instance with an admin account under **Connection and encoding**.
2. In **Upload queue**, choose **Library: TV series or Movies**.
3. Scan a local folder, use **Add files**, or drop video files/a folder onto the window.
4. **Series:** parse season/episode with a regex (the default handles both `S01E02` and `01x02`),
   with a live preview. **Movies:** each file is matched to an existing movie title by name + year,
   or a new draft title is created with the chosen name and year. Double-click a movie or
   click **Choose target** to search the catalog or correct a new movie's title/year.
5. Pick **Audio mode**, languages (e.g. `en, cs`), output resolution, quality + encoder,
   check the items to process, then **Upload**. **All listed languages** includes both
   matching English and Czech tracks in one MP4. **One preferred language** includes
   only the first available preferred language.

For each selected item it transcodes to a temp MP4, chunk-uploads it, attaches it to the right
episode/movie (creating it if it does not exist yet), tags the audio language, extracts and
uploads embedded text subtitles as sidecars, then deletes the temp files. **One encode and one
upload run at a time**, with the encoder allowed to run a few files ahead of the uploader so GPU
and network work overlap.

Movie matching preserves Unicode names and release years, including remakes. Ambiguous matches
require a target choice. Existing titles with media default to **skip**; checking them replaces
only files whose known audio languages are covered by the new output, preserving other languages.
Combined uploads retain files whose full stream inventory is unavailable.
Unmatched files default to **create + upload**. New titles are explicitly created through the API
so the server cannot silently attach them to another same-name title.
Movie targets are created before encoding or uploading; rejected title requests fail
immediately. The server assigns draft status. Created target IDs are kept for retries,
including when a batch is cancelled before the movie uploads.

Search and action filters make large batches easier to review. **Select visible** and **Clear
selection** affect only visible rows; checked files hidden by a filter still upload. Duplicate
selected targets are blocked before starting. Successful rows are unchecked, and **Retry failed**
selects only failed rows. Batch controls are locked while workers run; closing the app cancels
the batch and waits for cleanup. Saved source folder and library mode are restored next launch.

## Requirements

- Windows with **ffmpeg + ffprobe** on PATH (a "full" build, e.g. from
  https://www.gyan.dev/ffmpeg/builds/). NVENC needs an NVIDIA GPU.
- Python 3.10+.

## Setup & run

```powershell
cd tools\couchpush
python -m venv .venv
.\.venv\Scripts\Activate.ps1
pip install -r requirements.txt
python -m couchpush
```

## Build a standalone .exe

To run without a Python install (a single portable file at `dist\Couchpush.exe`):

```powershell
cd tools\couchpush
.\.venv\Scripts\Activate.ps1
pip install -r requirements.txt pyinstaller
.\build_exe.ps1
# Build separately while an older executable is running:
.\build_exe.ps1 -OutputDirectory 'dist\multi-audio'
```

(First launch of a `--onefile` exe is a few seconds slower while it unpacks; that is normal.)
The checked-in PyInstaller spec is part of the build: it prevents unrelated Poppler/Conda
ICU DLLs on PATH from shadowing Windows' ICU API and breaking the Qt import at startup.

## Output / quality

- Always **MP4 / H.264 / AAC**. A single audio track direct-plays. Multiple tracks are
  switchable after the server finishes HLS preparation; this copies video rather than
  encoding it again.
- **Resolution** in the queue defaults to **1080p**. Choose 720p, 480p, 2160p (4K), or
  Original size. Outputs fit the selected 16:9 bounding box while preserving aspect ratio
  and even dimensions; smaller sources are never enlarged. A 4K H.264 source is re-encoded
  when downscaling instead of being remuxed. The chosen resolution is remembered.
- 10-bit HEVC (Main 10) is converted to 8-bit automatically. HDR10/HLG and Dolby Vision sources
  with a compatible HDR10/HLG base are tone-mapped to SDR BT.709 for browser playback. Dolby
  Vision profile 5 has no compatible base and produces a clear per-file error; use an HDR10
  or SDR source instead. HDR conversion needs a full ffmpeg build with `zscale` and `tonemap`.
- Files already in **SDR h264 8-bit 4:2:0 are remuxed** (no re-encode, no quality loss); AAC stereo
  audio is copied, other audio is re-encoded to AAC (downmixed to stereo unless you tick
  "keep original channels").
- Quality presets (NVENC `cq`): Max (p6, cq 19), **High - default (p4, cq 23)**, Small
  (p4, cq 26), Smallest (p4, cq 29). Each +3 cq is roughly -27% file size. The common tiers
  use NVENC preset p4, which is ~1.5x faster than p5 on an RTX 3060 with near-identical size;
  for cartoons, Small/Smallest are usually fine.
- Decode uses `-hwaccel cuda`; the 8-bit conversion is done on the CPU via `-pix_fmt
  yuv420p`. It deliberately avoids the `scale_cuda` filter, which fails on some ffmpeg builds
  and on HDR/BT.2020 sources (and measured no faster than the CPU-convert path anyway).
- NVENC chooses the H.264 level automatically, including for UHD sources. The old fixed level
  4.2 caused `InitializeEncoder failed ... Invalid Level` on 4K movies. If NVENC fails, the
  same file is retried once with libx264 and software decoding; both errors are retained if
  that retry fails. Individual probe, encode and network errors do not strand the batch.
- The **Time** column shows per-episode transcode and upload seconds; the **Subs** column
  shows which embedded text-subtitle languages were extracted and uploaded.

## Audio language & alternates

**All listed languages** is the default. Enter `en, cs` to include one matching English
track and one matching Czech track, in that order. The first included track is the
default. Duplicate requests are ignored. Missing languages are reported in the log;
available requested tracks are still included. If no requested language exists, the
file fails clearly instead of uploading the wrong language. Container tags such as
`eng`, `cze` and `ces` are normalized, and output MP4 tracks receive ISO language tags.

Video is encoded once, with each audio track independently copied or converted to AAC.
The **Audio** column shows the actual included languages after probing. The server's
file-level `audioLang` contains the first language; the embedded streams hold all
languages. CouchVerse must finish probing and HLS preparation before the player can
switch tracks.

Combined files become the primary source; older primaries become alternates without
changing their language tags. Replacement deletes an existing file only when its
complete known language set is covered by the uploaded tracks. Untagged or unprobed
files and files containing another language are preserved. Old files are deleted
only after completion, tagging and successful primary promotion.

This change also includes a CouchVerse player fix to show embedded tracks alongside
separate language files and to switch back between their sources while preserving
position. Deploy the updated **backend and frontend** for that mixed-file behavior
and the admin `audioStreamsByFile` inventory. Older servers already support embedded
tracks on a title with only one file, but can hide them when other language files exist.

In **One preferred language**, `en, cs` means English first, Czech as fallback. If none
match, the first source track is used with a warning. An untagged source remains
untagged. If an episode already has a file:

- **same language present** -> the row is unchecked by default; check it to **Replace**
  (the new file is uploaded, then the old same-language file is deleted).
- **different language** -> uploaded as an **alternate audio** track (`audio_alt`).

Replacements preserve the old track's primary/alternate role. Untagged legacy files are kept
when adding known-language audio, and old files are deleted only after successful completion
and audio tagging of the new file.

## Subtitles

Every embedded **text** subtitle track is extracted to WebVTT with ffmpeg and uploaded as a
sidecar. Bitmap subs (PGS/VOBSUB) are skipped (they would need OCR).

## Server URL (HTTP vs HTTPS)

Both work. The session cookie is marked `Secure` by the server; the tool drops that flag
client-side so the cookie is also sent over a plain-HTTP LAN URL (e.g.
`http://192.168.0.69:8080`), which is the faster path for large uploads. The public HTTPS
URL works too.

## Credentials

Passwords are never written to the JSON config. With "Stay signed in" ticked, the session
cookie **and** the password are stored in the Windows Credential Manager (via `keyring`):
the cookie auto-reconnects next launch, and the password field is pre-filled so re-login is
one click. Untick it to store nothing and clear what was saved. Non-secret settings (server
URL, username, regex, paths, quality) live in `%APPDATA%\couchpush\config.json`.

## Troubleshooting

- **NVENC fails to start** (driver/session limits, no NVIDIA GPU): CouchPush retries with CPU
  automatically. You can choose `libx264 (CPU)` in settings for future batches.
- **"low disk space"**: set a Temp folder with room - up to `lookahead + 2` transcoded MP4s
  can exist at once.
- **A combined-language movie is not direct-play**: expected for multiple audio tracks.
  Wait for CouchVerse's HLS preparation job, then use the player's audio menu.
- **Only one language appears beside older files**: deploy the included CouchVerse
  backend/frontend player fix so embedded and separate-file languages share the menu.
- **400 invalid request body after uploading a new movie**: use the corrected build.
  Older CouchPush builds sent an unsupported `status` field while creating the title.
  Errors now include the failed HTTP method and endpoint. Movie creation happens
  before encoding, preventing an entire file transfer before this check.

## Layout

```
couchpush/
  config.py            settings + keyring-backed session cookie
  api/                 client.py (HTTP), models.py (typed responses)
  media/               ffprobe.py, plan.py (smart-copy + audio pick), ffmpeg.py, subs.py
  core/                parser.py, matcher.py, uploader.py, pipeline.py, eta.py
  workers/             pipeline_worker.py (Qt signal bridge)
  ui/                  main_window.py, movie_picker.py, theme.py
  __main__.py          entry point
```

## Verification

```powershell
cd tools\couchpush
.\.venv\Scripts\python.exe -m unittest discover -s tests -v
# Also verify real 4K encoding on an NVIDIA PC:
$env:COUCHPUSH_NVENC_TESTS = "1"
.\.venv\Scripts\python.exe -m unittest discover -s tests -v
# Render sample queue, compact window, settings and movie picker without server access:
.\.venv\Scripts\python.exe -m tests.preview_ui
```

The tests use the standard library plus the app dependencies. ffmpeg/ffprobe enable a short
real HDR conversion test; the opt-in NVENC test reproduces the former level error and verifies
the corrected UHD output. UI checks run with Qt offscreen and do not access saved credentials.
The CPU media test also verifies an English/Czech MP4, AAC copy plus AC3 conversion,
language order, and exactly one default track. For the accompanying player fix, run
`npm run test:playback-audio` in `frontend`; it exercises the actual player handlers
through embedded tracks and separate files without a server or browser decoder.
