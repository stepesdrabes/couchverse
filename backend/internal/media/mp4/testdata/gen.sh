#!/usr/bin/env bash
# Regenerates the init sections and segments the mp4 tests read. The Dolby Vision
# inits need streams with RPUs (see docs/spikes/s5-dolby-vision.md); pass their
# directory as $1 to refresh those too.
set -euo pipefail
cd "$(dirname "$0")"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

pkg() { # name input-args... -- output-args...
  local name=$1; shift
  local in=() out=()
  while [ "$1" != "--" ]; do in+=("$1"); shift; done
  shift
  out=("$@")
  mkdir -p "$tmp/$name"
  ffmpeg -y -hide_banner -loglevel error "${in[@]}" "${out[@]}" \
    -f hls -hls_time 1 -hls_playlist_type vod -hls_segment_type fmp4 \
    -hls_fmp4_init_filename init.mp4 -hls_segment_filename "$tmp/$name/seg_%d.m4s" "$tmp/$name/index.m3u8"
  cp "$tmp/$name/init.mp4" "$name.init.mp4"
  cp "$tmp/$name/seg_0.m4s" "$name.seg.m4s"
}

video=(-f lavfi -i "testsrc2=size=128x72:rate=24:duration=2")
audio=(-f lavfi -i "sine=frequency=440:duration=2:sample_rate=48000")
pkg avc "${video[@]}" -- -c:v libx264 -profile:v high -preset ultrafast -g 24
pkg hevc10 "${video[@]}" -- -vf format=yuv420p10le -c:v libx265 -preset ultrafast -tag:v hvc1 \
  -x265-params "keyint=24:min-keyint=24:scenecut=0:open-gop=0:log-level=error"
pkg av1 "${video[@]}" -- -c:v libsvtav1 -preset 12 -g 24
pkg aac "${audio[@]}" -- -c:a aac -ac 2
pkg ac3 "${audio[@]}" -- -af "pan=5.1|FL=c0|FR=c0|FC=c0|LFE=c0|BL=c0|BR=c0" -c:a ac3 -b:a 192k
pkg eac3 "${audio[@]}" -- -af "pan=5.1|FL=c0|FR=c0|FC=c0|LFE=c0|BL=c0|BR=c0" -c:a eac3 -b:a 192k

if [ -n "${1:-}" ]; then
  pkg dv81 -i "$1/dv81mm.mkv" -t 2 -- -map 0:v:0 -c copy -tag:v hvc1 -strict unofficial
  pkg dv5 -i "$1/dv5mm.mkv" -t 2 -- -map 0:v:0 -c copy -tag:v dvh1 -strict unofficial
  # their segments are megabytes; the tests only read the inits
  rm -f dv81.seg.m4s dv5.seg.m4s
fi
ls -la
