# Spike S3: Apple-grade HLS from ffmpeg

Status: done (2026-10-03). Throwaway experiments lived in `data/spike` (gitignored); the recipes
below are what `backend/internal/feature/playback` now does. Plan references: 8.5 (Playback v2),
13 (quality), 16 (S3). Dolby Vision is in `s5-dolby-vision.md`.

## Verdict

ffmpeg's HLS muxer produces fMP4 presentations that AVFoundation and hls.js both play, for every
tier: HEVC Main 10 (PQ and HLG) copied and tagged `hvc1`, E-AC-3 and AC-3 passed through (Atmos
announced as `CHANNELS="16/JOC"`), AAC stereo next to them, WebVTT subtitle renditions with
`X-TIMESTAMP-MAP`, an I-frame playlist that turns on AVPlayer's trick play
(`canPlayFastForward`), and tone-mapped H.264 rungs. Two things ffmpeg does not do for us: write a
correct multivariant playlist (it omits `CODECS` for HEVC and marks every rendition default), and
keep separately muxed renditions on one timeline (see gotcha 1). Both are handled in Go.

Verified on this Mac (M1 Pro, macOS 26.7): our validator reports no errors for any tier, and
`scripts/avplayer-probe.swift` loads, decodes and plays every presentation, switches every audio
and subtitle option and gets trick play. The same holds for the Docker image's ffmpeg
(`debian:bookworm-slim`, 5.1.9) and for trixie's 7.1.5. Apple's `mediastreamvalidator` was not
run (it needs an Apple ID download): `make hls-apple` runs it when it is installed. Real devices
are on the checklist below.

## Versions

ffmpeg 8.1.1 (Homebrew; no zscale), 7.1.5 (Debian trixie), 6.1.1 (Ubuntu 24.04, GitHub runners),
5.1.9 (Debian bookworm); hls.js 1.6.16; Playwright 1.60 (Chromium headless shell 148), Google
Chrome 154; macOS 26.7, Swift 6.4; dovi_tool 2.3.4, mkvmerge 102.0 for the samples.

## Recipes

One timeline for renditions muxed separately (video copy, audio copies and transcodes, ladder
rungs, trick play, JIT runs):

```sh
ffmpeg -copyts -start_at_zero [-ss <start>] -i <source> \
  <map and codec options> -output_ts_offset 1.4 \
  -f hls -hls_time 6 -hls_playlist_type vod -hls_segment_type fmp4 \
  -hls_segment_options movflags=+frag_discont+negative_cts_offsets \
  -hls_fmp4_init_filename init.mp4 [-hls_flags independent_segments] \
  -hls_segment_filename <dir>/seg_%05d.m4s <dir>/index.m3u8
```

- The copied source (one read, together with every audio rendition): `-map 0:v:0 -c:v copy
  -tag:v hvc1` (HEVC). Its segments end at source keyframes, so it is its own multivariant
  playlist ("Original", `?video=original`) and never an ABR rung.
- Audio per source track: AAC stereo (`-c:a aac -ac 2 -ar 48000 -b:a 160k`, or a copy when it
  already is), plus for multichannel tracks a surround rendition: AC-3/E-AC-3 copied (Atmos
  included), anything else (DTS, TrueHD, PCM, FLAC, Opus, AAC 5.1) as `-c:a eac3 -ac 6 -b:a 640k`.
- Ladder rung: `-vf scale=-2:<h>,<tone map>,format=yuv420p`, libx264 `-profile:v high -crf 23
  -maxrate/-bufsize` or a hardware encoder at the same rate, `-g <2 s of frames>
  -force_key_frames expr:gte(t,<start>+n_forced*2)`, BT.709 tags. IDRs on a two-second grid of
  source time make every rung cut at the same instants, which ABR needs.
- Tone mapping (HDR to SDR): `zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,
  tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p`.
- Trick play: `-skip_frame nokey` on the input (only keyframes are decoded, cheap even for 4K
  HEVC on a Pi), `-vf fps=1/2,scale=-2:180`, libx264 `-g 1`, `-hls_time 2`. Every segment then
  holds one intra frame, so `iframes.m3u8` is the same list with `EXT-X-I-FRAMES-ONLY` and no
  byte ranges.
- WebVTT renditions are generated per request from the sidecar files: a media playlist on the
  6-second grid, and per segment every cue overlapping it (a cue across a boundary repeats, as
  the spec asks) under `X-TIMESTAMP-MAP=MPEGTS:126000,LOCAL:00:00:00.000` (1.4 s at 90 kHz).
- Multivariant playlist (written in Go from each rendition's `rendition.json`, measured after
  the job: codec strings from the init section, peak and average bit rates by Apple's
  definition): every video rendition is listed once per audio group ("surround" first, then
  "stereo"), `CODECS` names the video and every codec of the group, `BANDWIDTH` and
  `AVERAGE-BANDWIDTH` add the group's largest audio rendition, plus `RESOLUTION`, `FRAME-RATE`,
  `VIDEO-RANGE`, `SUPPLEMENTAL-CODECS` for Dolby Vision 8, `SUBTITLES` and `CLOSED-CAPTIONS=NONE`.
  The ladder starts with the rung nearest 720p because AVPlayer begins with the first variant.

```
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="surround",NAME="English 5.1",LANGUAGE="en",DEFAULT=YES,AUTOSELECT=YES,CHANNELS="6",URI="audio-1-eac3/index.m3u8"
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="surround",NAME="Čeština",LANGUAGE="cs",DEFAULT=NO,AUTOSELECT=YES,CHANNELS="2",URI="audio-2-aac/index.m3u8"
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="stereo",NAME="English 5.1",LANGUAGE="en",DEFAULT=YES,AUTOSELECT=YES,CHANNELS="2",URI="audio-1-aac/index.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="English",LANGUAGE="en",DEFAULT=NO,AUTOSELECT=YES,FORCED=NO,URI="subtitles/<id>/index.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=2051099,AVERAGE-BANDWIDTH=2036399,CODECS="hvc1.2.4.L120.90,ec-3,mp4a.40.2",RESOLUTION=1280x720,FRAME-RATE=24.000,VIDEO-RANGE=PQ,AUDIO="surround",SUBTITLES="subs",CLOSED-CAPTIONS=NONE
source/index.m3u8
#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=60000,CODECS="avc1.64000d",RESOLUTION=320x180,VIDEO-RANGE=SDR,URI="trickplay/iframes.m3u8"
```

Instant play (JIT) runs the rung recipe from the requested segment with `-start_number`, its
audio muxed in, `-hls_flags temp_file`, one init per run (`init_<run>.mp4`; the first one serves
them all) and a virtual VOD playlist on the same 6-second grid, behind a one-variant
multivariant playlist that adds the subtitle renditions.

## Gotchas

1. Separate outputs are shifted separately. ffmpeg moves each output so its first decode
   timestamp is zero: the copied HEVC (two B-frames of delay) lands 83 ms later than its audio,
   which a player reading only `tfdt` plays out of sync. By default the shift hides in an edit
   list that AVFoundation honours but hls.js ignores; `use_editlist=0` instead zeroes every
   track. `-copyts -start_at_zero` plus the same `-output_ts_offset` on every output keeps all
   timestamps positive (nothing gets shifted), `frag_discont` writes the real decode time into
   `tfdt`, and `negative_cts_offsets` puts the first frame's presentation time on it.
   Debian's 5.1 lands 21 ms later than 8.1 (a different `-start_at_zero` base); harmless.
2. ffmpeg's own `-master_pl_name` playlist has no `CODECS` for HEVC and `DEFAULT=YES` on every
   rendition; we never use it.
3. Audio-only playlists from ffmpeg lack `EXT-X-INDEPENDENT-SEGMENTS`; the multivariant
   playlist declares it for everything.
4. `-hls_time` with a copied stream cuts at the first keyframe after each boundary, so the
   Original's segments are irregular (6.006 s for 2.002 s GOPs, up to the GOP length for long
   GOPs). Copied open-GOP sources start segments with a CRA whose leading pictures need the
   previous segment; AVPlayer skips them.
5. The muxer creates `init.mp4` before it writes the `moov`; a JIT session waits until the file
   parses.
6. `-shortest` stalls ffmpeg forever once SRT inputs run out (sample generation).
7. Chrome now answers `canPlayType('application/vnd.apple.mpegurl')` but has no
   `video.audioTracks`, so the web player keeps native HLS for Safari only.
8. hls.js would add text tracks for the subtitle renditions next to the player's own sidecar
   tracks; it runs with `renderTextTracksNatively: false`.
9. ffprobe reports ISO 639-2 languages (`eng`, `cze`); playlists need BCP 47 (`en`, `cs`).
10. Homebrew's ffmpeg has no zscale. Without it HDR is converted to 8-bit and relabelled
    BT.709 (washed out but playable and honestly tagged); the Docker image tone-maps properly.
11. fMP4 media segments are served as `video/mp4` (`audio/mp4` for audio renditions), init
    sections the same, playlists as `application/vnd.apple.mpegurl`.
12. The validator warns that HDR-only `?video=original` playlists offer no SDR variant. That is
    by design: SDR clients are given the ladder playlist instead.

## Validator rules (`couchverse validate-hls`, `internal/hls`)

Errors: missing `EXT-X-VERSION` or one below what the tags need (6 for `EXT-X-MAP`, 4 for byte
ranges and I-frames); multivariant without variants; `EXT-X-MEDIA` without `GROUP-ID`/`NAME`,
unknown `TYPE`, a `URI` on closed captions or none on subtitles, a `LANGUAGE` that is not BCP 47,
`DEFAULT=YES` without `AUTOSELECT=YES`, `FORCED` outside subtitles, audio without `CHANNELS`, two
renditions with one name or two defaults in a group; variants without `BANDWIDTH`,
`AVERAGE-BANDWIDTH`, `CODECS` or `FRAME-RATE` (video), referencing a missing group, declaring
`CODECS` that miss a codec their media uses, `RESOLUTION` or `VIDEO-RANGE` that disagree with the
init section, `BANDWIDTH` more than 10% under the measured peak (Apple's definition: any run of
segments lasting 0.5 to 1.5 target durations, video plus the group's largest audio); audio
`CHANNELS` that disagree with `dac3`/`dec3`/the sample entry (16 for JOC); I-frame variants
without attributes, with wrong `CODECS`/`RESOLUTION`, or whose playlist lacks
`EXT-X-I-FRAMES-ONLY`; durations of variants and renditions more than 2 s apart; media playlists
without `EXT-X-TARGETDURATION`, with an `EXTINF` that rounds above it, VOD without
`EXT-X-ENDLIST`; fMP4 without `EXT-X-MAP`, an init without `moov`, a segment that carries a
`moov`, has no `moof`, names a track the init lacks, does not start with a sync sample, or starts
more than 0.25 s off the playlist timeline; MPEG-TS without sync bytes; WebVTT without the
`WEBVTT` header or `X-TIMESTAMP-MAP`, cues ending before they start; byte ranges past the end;
empty or unfetchable resources; wrong HTTP content types.

Warnings: no `EXT-X-INDEPENDENT-SEGMENTS`, no I-frame playlist, HDR without SDR variants,
`CLOSED-CAPTIONS` not `NONE`, rendition without `LANGUAGE`, forced subtitles without
`AUTOSELECT`, declared codecs nothing uses, `FRAME-RATE` more than 0.5 off the measured one,
`AVERAGE-BANDWIDTH` more than 10% off, Dolby Vision without `SUPPLEMENTAL-CODECS`, target
durations above 10 s, no `EXT-X-PLAYLIST-TYPE`, segments whose media duration is far from
`EXTINF`, cues outside their segment window.

## AVFoundation harness (`scripts/avplayer-probe.swift`)

Loads the URL as an `AVURLAsset` (no headers or cookies), lists the variants
(`AVAssetVariant`: peak and average bit rate, codec, size, range, frame rate, audio formats),
loads every referenced media playlist on its own (variants and audio must be playable and as long
as the presentation), lists the audible and legible media selection groups (language, name,
forced, default), plays a few seconds muted through an `AVPlayerItemVideoOutput` (frames must
decode), selects every audio and subtitle option halfway, and reports trick play. `--audio n`,
`--subtitles n` and `--iframes` turn expectations into failures.

## Running the checks

- `make hls-check`: `TestPlaybackTiers` (also in `go test ./...` when ffmpeg is installed) builds
  the real app on a test database, generates samples, runs the real probe, subtitle and
  transcode jobs, asks for playback as Chrome and as an Apple TV and validates every
  presentation; on macOS it plays each one with the harness. CI: `backend.yml` (Linux, validator)
  and `hls.yml` (macOS, validator and AVFoundation).
- `make hls-apple`: the same plus `mediastreamvalidator` when installed.
- Against a running server: `make sample-media ingest-samples hls-server-check` (every movie for
  every profile in `contract/fixtures/device-profiles`) and `make e2e-playback` (the web player
  in Chromium, `CHANNEL=chrome` for Google Chrome).

## Real-device checklist

Apple TV 4K (2nd or 3rd gen), tvOS 27, connected to an HDR10/Dolby Vision TV and an Atmos
receiver or soundbar:
- [ ] `make sample-media ingest-samples`, then play every sample from the Apple TV app (Phase 6)
      or by pointing a test build's `AVPlayer` at the payload's `streamUrl`.
- [ ] Static Bloom: TV switches to HDR10; audio menu lists English 5.1 and Czech; the receiver
      shows Dolby Digital Plus 5.1; subtitles render natively; forced Czech subtitles on Glass
      Harbor appear without being picked.
- [ ] Deep Current: E-AC-3 made from TrueHD and DTS plays 5.1.
- [ ] Northern Lights: HLG mode.
- [ ] Scrubbing on the Siri Remote shows trick-play thumbnails; fast forward works.
- [ ] Quality: Original and the ladder both start within two seconds over Wi-Fi.
- [ ] An E-AC-3 JOC (Atmos) source (from a real release) shows Dolby Atmos on the receiver.

Raspberry Pi 4 (64-bit, Docker image from this branch, media on USB 3 storage):
- [ ] `make sample-media` (or copy the samples), upload them, note how long the package (copy),
      trick-play and 720p jobs take; the copy should run near disk speed.
- [ ] `PickEncoder` chooses `h264_v4l2m2m`; a 1080p HDR10 HEVC source transcodes to the
      tone-mapped 720p rung at or above real time, or document the factor.
- [ ] `-force_key_frames` is honoured by `h264_v4l2m2m` (the rung's segments are 6 s:
      `couchverse validate-hls` reports no `fmp4.duration` warnings).
- [ ] Instant play: first segment of a JIT session within 5 s; a far seek within 8 s.
- [ ] Trick play over a 4K HEVC source (keyframes only) finishes in reasonable time.
