#!/usr/bin/env bash
# Generates copyright-free sample media covering every playback pipeline tier:
#   direct play (h264/aac mp4), copy-remux (h264+ac3 mkv), full transcode (hevc mkv).
# Usage: ./scripts/gen-sample-media.sh [output-dir]
set -euo pipefail

OUT="${1:-data/samples}"
mkdir -p "$OUT/movies"

if command -v ffmpeg >/dev/null; then
  FF=(ffmpeg)
else
  echo "ffmpeg not found locally, using docker (linuxserver/ffmpeg)"
  ABS_OUT="$(cd "$OUT" && pwd)"
  FF=(docker run --rm -v "$ABS_OUT":"$ABS_OUT" -w "$ABS_OUT" linuxserver/ffmpeg)
  OUT="$ABS_OUT"
fi

gen_video() { # name container vcodec acodec extra...
  local name=$1 vcodec=$2 acodec=$3 out=$4
  "${FF[@]}" -y -hide_banner -loglevel error \
    -f lavfi -i "testsrc2=size=1280x720:rate=25:duration=30" \
    -f lavfi -i "sine=frequency=440:duration=30" \
    -c:v "$vcodec" -preset ultrafast -c:a "$acodec" -shortest \
    -metadata title="$name" "$out"
  echo "wrote $out"
}

mkdir -p "$OUT/movies/Test Pattern (2026)" "$OUT/movies/Glass Harbor (2025)" "$OUT/movies/Static Bloom (2024)"
gen_video "Test Pattern"  libx264 aac "$OUT/movies/Test Pattern (2026)/Test Pattern (2026).mp4"
gen_video "Glass Harbor"  libx264 ac3 "$OUT/movies/Glass Harbor (2025)/Glass Harbor (2025).mkv"
gen_video "Static Bloom"  libx265 aac "$OUT/movies/Static Bloom (2024)/Static Bloom (2024).mkv"

echo "sample media in $OUT"
