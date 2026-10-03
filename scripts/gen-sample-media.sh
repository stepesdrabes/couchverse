#!/usr/bin/env bash
# Generates copyright-free sample media covering every playback tier for every client:
#   Test Pattern     h264/aac mp4                        direct everywhere
#   Glass Harbor     h264 + AC-3 5.1 mkv, 2 subtitles    remux (AAC for browsers, AC-3 for Apple)
#   Static Bloom     HEVC Main 10 HDR10 mkv, E-AC-3 5.1 + Czech AAC, subtitles
#                                                        remux as HDR for Apple, tone-mapped ladder for browsers
#   Quiet Hours      HEVC 8-bit SDR mkv                  remux to browsers that decode HEVC, ladder otherwise
#   Northern Lights  HEVC Main 10 HLG mp4                HLG remux / ladder
#   Deep Current     h264 + TrueHD and DTS mkv           remux with E-AC-3 made from lossless/DTS audio
#   Neon Drift       VP9/Opus webm                       direct in browsers, never on Apple
# With dovi_tool and mkvmerge installed it adds Dolby Vision 8.1, 5 and 7 files
# (docs/spikes/s5-dolby-vision.md).
# Usage: ./scripts/gen-sample-media.sh [output-dir] [seconds]
set -euo pipefail

OUT="${1:-data/samples}"
SECONDS_LONG="${2:-30}"
mkdir -p "$OUT/movies"
OUT="$(cd "$OUT" && pwd)"

if command -v ffmpeg >/dev/null; then
  FF=(ffmpeg)
else
  echo "ffmpeg not found locally, using docker (linuxserver/ffmpeg)"
  FF=(docker run --rm -v "$OUT":"$OUT" -w "$OUT" linuxserver/ffmpeg)
fi
TMP="$OUT/.tmp"
mkdir -p "$TMP"
trap 'rm -rf "$TMP"' EXIT

D=$SECONDS_LONG
VIDEO=(-f lavfi -i "testsrc2=size=1280x720:rate=24:duration=$D")
STEREO=(-f lavfi -i "sine=frequency=440:duration=$D:sample_rate=48000")
SURROUND=(-f lavfi -i "sine=frequency=330:duration=$D:sample_rate=48000,pan=5.1|FL=c0|FR=c0|FC=c0|LFE=c0|BL=c0|BR=c0")
SURROUND71=(-f lavfi -i "sine=frequency=550:duration=$D:sample_rate=48000,pan=7.1|FL=c0|FR=c0|FC=c0|LFE=c0|BL=c0|BR=c0|SL=c0|SR=c0")
HDR10=(-vf "format=yuv420p10le,setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc"
  -c:v libx265 -preset ultrafast -tag:v hvc1
  -x265-params "keyint=48:min-keyint=48:hdr10=1:master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(10000000,1):max-cll=1000,400:log-level=error")

subtitle() { # lang text -> path of an SRT with a cue across a segment boundary
  local path="$TMP/$1.srt"
  printf '1\n00:00:01,000 --> 00:00:04,000\n%s\n\n2\n00:00:05,500 --> 00:00:07,500\n%s (across a segment boundary)\n\n3\n00:00:20,000 --> 00:00:23,000\n%s, later\n' \
    "$2" "$2" "$2" > "$path"
  echo "$path"
}

gen() { # "Title (Year)" extension ffmpeg-args...
  local dir="$OUT/movies/$1"
  mkdir -p "$dir"
  "${FF[@]}" -y -hide_banner -loglevel error "${@:3}" "$dir/$1.$2"
  echo "wrote $dir/$1.$2"
}

gen "Test Pattern (2026)" mp4 "${VIDEO[@]}" "${STEREO[@]}" \
  -map 0:v -map 1:a -c:v libx264 -preset ultrafast -c:a aac

gen "Glass Harbor (2025)" mkv "${VIDEO[@]}" "${SURROUND[@]}" \
  -i "$(subtitle en "Hello from Glass Harbor")" -i "$(subtitle cs "Ahoj z Glass Harbor")" \
  -map 0:v -map 1:a -map 2 -map 3 -c:v libx264 -preset ultrafast -g 48 -c:a ac3 -b:a 384k -c:s srt \
  -metadata:s:a:0 language=eng -metadata:s:s:0 language=eng -metadata:s:s:1 language=ces -disposition:s:1 forced

gen "Static Bloom (2024)" mkv "${VIDEO[@]}" "${SURROUND[@]}" "${STEREO[@]}" -i "$(subtitle en "Static Bloom subtitle")" \
  -map 0:v -map 1:a -map 2:a -map 3 "${HDR10[@]}" \
  -c:a:0 eac3 -b:a:0 448k -c:a:1 aac -ac:a:1 2 -c:s srt \
  -metadata:s:a:0 language=eng -metadata:s:a:0 title="English 5.1" -metadata:s:a:1 language=ces -metadata:s:s:0 language=eng

gen "Quiet Hours (2019)" mkv "${VIDEO[@]}" "${STEREO[@]}" -map 0:v -map 1:a \
  -c:v libx265 -preset ultrafast -tag:v hvc1 -x265-params "keyint=48:min-keyint=48:log-level=error" -c:a aac

gen "Northern Lights (2022)" mp4 "${VIDEO[@]}" "${STEREO[@]}" -map 0:v -map 1:a \
  -vf "format=yuv420p10le,setparams=color_primaries=bt2020:color_trc=arib-std-b67:colorspace=bt2020nc" \
  -c:v libx265 -preset ultrafast -tag:v hvc1 -x265-params "keyint=48:min-keyint=48:log-level=error" -c:a aac

gen "Deep Current (2021)" mkv "${VIDEO[@]}" "${SURROUND71[@]}" "${SURROUND[@]}" -map 0:v -map 1:a -map 2:a \
  -c:v libx264 -preset ultrafast -g 48 -c:a:0 truehd -c:a:1 dca -b:a:1 768k -strict -2 \
  -metadata:s:a:0 language=eng -metadata:s:a:1 language=eng -metadata:s:a:1 title="DTS"

gen "Neon Drift (2023)" webm "${VIDEO[@]}" "${STEREO[@]}" -map 0:v -map 1:a \
  -c:v libvpx-vp9 -deadline realtime -cpu-used 8 -b:v 1M -c:a libopus

# Dolby Vision: RPUs from dovi_tool injected into an HDR10 encode; mkvmerge signals
# the configuration record (profile 7 adds an enhancement layer)
if command -v dovi_tool >/dev/null && command -v mkvmerge >/dev/null; then
  frames=$((D * 24))
  "${FF[@]}" -y -hide_banner -loglevel error "${VIDEO[@]}" "${HDR10[@]}" -bsf:v hevc_mp4toannexb -f hevc "$TMP/bl.hevc"
  for p in 8.1 5; do
    printf '{"cm_version":"V40","profile":"%s","length":%d,"source_min_pq":7,"source_max_pq":3079,"level6":{"max_display_mastering_luminance":1000,"min_display_mastering_luminance":1,"max_content_light_level":1000,"max_frame_average_light_level":400}}' \
      "$p" "$frames" > "$TMP/dv$p.json"
    dovi_tool generate -j "$TMP/dv$p.json" -o "$TMP/dv$p.rpu" >/dev/null
    dovi_tool inject-rpu -i "$TMP/bl.hevc" --rpu-in "$TMP/dv$p.rpu" -o "$TMP/dv$p.hevc" >/dev/null
  done
  dovi_tool -m 1 extract-rpu -i "$TMP/dv8.1.hevc" -o "$TMP/mel.rpu" >/dev/null
  "${FF[@]}" -y -hide_banner -loglevel error -f lavfi -i "testsrc2=size=640x360:rate=24:duration=$D" \
    -vf format=yuv420p10le -c:v libx265 -preset ultrafast -x265-params "keyint=48:min-keyint=48:log-level=error" \
    -bsf:v hevc_mp4toannexb -f hevc "$TMP/el.hevc"
  dovi_tool inject-rpu -i "$TMP/el.hevc" --rpu-in "$TMP/mel.rpu" -o "$TMP/el-rpu.hevc" >/dev/null
  dovi_tool mux --bl "$TMP/bl.hevc" --el "$TMP/el-rpu.hevc" -o "$TMP/dv7.hevc" >/dev/null
  "${FF[@]}" -y -hide_banner -loglevel error "${SURROUND[@]}" -c:a eac3 -b:a 448k "$TMP/audio.mka"
  for p in 8.1 5 7; do
    name="Prism Profile ${p%%.*} (2020)"
    mkdir -p "$OUT/movies/$name"
    mkvmerge -q -o "$OUT/movies/$name/$name.mkv" --default-duration 0:24p "$TMP/dv$p.hevc" "$TMP/audio.mka"
    echo "wrote $OUT/movies/$name/$name.mkv"
  done
fi

echo "sample media in $OUT"
