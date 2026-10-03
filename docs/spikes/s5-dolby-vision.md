# Spike S5: Dolby Vision profiles 5, 7 and 8.1

Status: done (2026-10-03) on synthetic samples; real Dolby Vision releases and a Dolby Vision TV
are on the checklist. Plan references: 8.5 (HDR handling), 16 (S5). HLS basics are in
`s3-apple-hls.md`.

## Verdict

- **8.1** (HDR10-compatible base layer) is copied into the Original as `hvc1` with its `dvvC`
  box and `SUPPLEMENTAL-CODECS="dvh1.08.LL/db1p"`, so Dolby Vision devices play it as Dolby
  Vision and HDR10 devices as HDR10, from one rendition. Where ffmpeg cannot write `dvvC`
  (Debian bookworm's 5.1) the same copy is plain HDR10.
- **5** (no compatible base layer, IPT-PQ-c2) is copied as `dvh1` with `dvcC` and offered only
  to clients that list `dolbyVision5`. Everyone else gets the ladder, tone-mapped from the base
  layer, whose colours are wrong (see gotcha 4). Without `dvh1` support (ffmpeg 5) there is no
  copy at all.
- **7** (dual layer, Blu-ray) is offered as is only to clients listing `dolbyVision7` (Android TV
  boxes that decode it direct-play the MKV); the Original is the HDR10 base layer with the
  enhancement layer and RPUs removed (`filter_units=remove_types=62|63`), which AVPlayer plays as
  HDR10. No Apple device decodes profile 7.
- ffmpeg 6.0 or newer is needed to signal Dolby Vision in fMP4 (`dvh1`, `dvcC`/`dvvC` with
  `-strict unofficial`); the server detects the version (`Features.DolbyVision`) and decides
  accordingly. The Docker image moves to Debian trixie (ffmpeg 7.1.5) for this.

## How it was verified

No real Dolby Vision sample can be redistributed, so the samples are built (the recipe is in
`scripts/gen-sample-media.sh`, which adds them when `dovi_tool` and `mkvmerge` are installed):

1. Encode an HDR10 base layer with x265 (Main 10, PQ, mastering display) as Annex B.
2. `dovi_tool generate` writes profile 8.1 and 5 RPUs from a JSON config (it cannot generate
   profile 7); `dovi_tool -m 1 extract-rpu` turns the 8.1 RPUs into profile 7 MEL ones.
3. `dovi_tool inject-rpu` adds the RPUs to the base layer (8.1, 5) or to a 960x540 enhancement
   layer, which `dovi_tool mux` interleaves with the base layer (7).
4. `mkvmerge` reads the RPUs and writes the Dolby Vision configuration record (ffprobe:
   `dv_profile` 8/5/7, `dv_bl_signal_compatibility_id` 1/0/6, `el_present_flag` 1 for 7).

The profile 5 sample carries a PQ base layer under profile 5 RPUs, so its colours are not real
IPT; it proves signalling and plumbing, not colour.

Then, with ffmpeg 8.1.1 (Homebrew), 7.1.5 (trixie), 6.1.1 (noble) and 5.1.9 (bookworm):

| Package | 8.1 / 7.1 / 6.1 | 5.1 (bookworm) |
|---|---|---|
| 8.1 `-tag:v hvc1` | hvc1, RPUs kept, no `dvvC` | same |
| 8.1 `-tag:v hvc1 -strict unofficial` | hvc1 + `dvvC` | hvc1, no `dvvC` |
| 5 `-tag:v dvh1 -strict unofficial` | dvh1 + `dvcC` | silently `hev1`, no `dvcC` |
| 7 `-bsf:v filter_units=remove_types=62\|63` | hvc1, no EL, no RPU | same |
| 7 `-bsf:v dovi_rpu=strip=1` | RPU removed, EL kept | filter missing |

RPU presence was checked with `dovi_tool extract-rpu` on the packaged streams. Every package
plays in AVFoundation on macOS 26.7 (`scripts/avplayer-probe.swift`), including profile 7
copied with its enhancement layer (the decoder skips NAL types 62 and 63); the server still strips
it because real Apple TVs are less forgiving and the layers double the bit rate.

Through the server (`make ingest-samples hls-server-check`): Apple TV profile (`dolbyVision5`,
`dolbyVision8`) gets 8.1 and 5 remuxed as Dolby Vision and 7 remuxed as HDR10; Android TV
(`dolbyVision7`, `dolbyVision8`) direct-plays 8.1 and 7 and gets the ladder for 5; Chrome gets
the tone-mapped ladder for all three. On bookworm's ffmpeg the Apple TV gets 5 as the ladder.

## Tone mapping

The Docker images (bookworm and trixie) ship zscale and tonemap, and libplacebo. The ladder uses
zscale + tonemap (hable). Homebrew's ffmpeg lacks zscale; there the HDR picture is only
converted to 8-bit and relabelled BT.709.

## Gotchas

1. ffmpeg's libx265 wrapper can only produce Dolby Vision from frames that already carry RPU
   metadata, and it picks the profile from the colour tags (it labelled every re-encode 8.1).
   Use dovi_tool and mkvmerge for test media.
2. mkvmerge options apply to the next file on the command line (`-D -S hdr10.mkv`, not after it).
3. `dvcC` is written for profiles up to 7 and `dvvC` above, by mp4 muxers that know them.
4. Profile 5 colours need the RPU's reshaping; zscale on the base layer gives green/purple
   pictures. libplacebo can apply it (`apply_dolbyvision`, ffmpeg 6+) but needs a Vulkan device,
   which a headless Pi 4 (v3dv) may or may not provide. Left as an open issue; profile 5 is
   mostly streaming-service output and rare in a home library.
5. HDR10+ is detected from the first frame's SMPTE 2094-40 side data and delivered as HDR10 to
   clients without `hdr10plus` (the dynamic metadata is ignored, the picture is the same).

## Real-device checklist

- [ ] Apple TV 4K on a Dolby Vision TV: Prism Profile 8 and 5 switch the TV to Dolby Vision;
      Prism Profile 7 plays HDR10.
- [ ] Apple TV 4K on an HDR10-only TV: Prism Profile 8 plays HDR10 (the `hvc1` copy); profile 5
      falls back to the ladder (the profile must not list `dolbyVision5`).
- [ ] A real profile 7 FEL Blu-ray remux and a real profile 8.1 streaming release: same checks,
      and `couchverse validate-hls` on their playlists.
- [ ] A real profile 5 release on Chrome: note how wrong the tone-mapped colours are.
- [ ] Android TV / Google TV device with Dolby Vision (Phase 11): direct play of 7 and 8.1.
