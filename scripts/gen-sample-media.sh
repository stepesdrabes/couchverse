#!/usr/bin/env bash
# Generates copyright-free sample media covering every playback pipeline tier:
#   direct play (h264/aac mp4), copy-remux (h264+ac3 mkv), full transcode (hevc mkv),
#   music (mp3 + flac with tags).
# Usage: ./scripts/gen-sample-media.sh [output-dir]
set -euo pipefail

OUT="${1:-data/samples}"
mkdir -p "$OUT/movies" "$OUT/music"

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

gen_audio() { # title artist album track out extra...
  local title=$1 artist=$2 album=$3 track=$4 out=$5; shift 5
  "${FF[@]}" -y -hide_banner -loglevel error \
    -f lavfi -i "sine=frequency=$((200 + track * 60)):duration=20" \
    -metadata title="$title" -metadata artist="$artist" \
    -metadata album="$album" -metadata track="$track" \
    "$@" "$out"
  echo "wrote $out"
}

mkdir -p "$OUT/movies/Test Pattern (2026)" "$OUT/movies/Glass Harbor (2025)" "$OUT/movies/Static Bloom (2024)"
gen_video "Test Pattern"  libx264 aac "$OUT/movies/Test Pattern (2026)/Test Pattern (2026).mp4"
gen_video "Glass Harbor"  libx264 ac3 "$OUT/movies/Glass Harbor (2025)/Glass Harbor (2025).mkv"
gen_video "Static Bloom"  libx265 aac "$OUT/movies/Static Bloom (2024)/Static Bloom (2024).mkv"

ALBUM_DIR="$OUT/music/Sine Language/Pure Tones"
mkdir -p "$ALBUM_DIR"
gen_audio "Square One"    "Sine Language" "Pure Tones" 1 "$ALBUM_DIR/01 Square One.mp3" -b:a 192k
gen_audio "Second Wave"   "Sine Language" "Pure Tones" 2 "$ALBUM_DIR/02 Second Wave.mp3" -b:a 192k
gen_audio "Third Harmonic" "Sine Language" "Pure Tones" 3 "$ALBUM_DIR/03 Third Harmonic.flac"

echo "sample media in $OUT"
