#!/usr/bin/env bash
# Asks a running server how every movie plays for each device profile in
# contract/fixtures/device-profiles, validates every HLS presentation it hands out
# with `couchverse validate-hls`, and on macOS plays the Apple TV's in AVFoundation
# (scripts/avplayer-probe.swift). Apple's mediastreamvalidator runs too when installed.
# usage: SERVER=http://localhost:8080 CV_USER=admin CV_PASSWORD=admin scripts/hls-check.sh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SERVER=${SERVER:-http://localhost:8080}
API=$SERVER/api/v1
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
jar=$tmp/jar
go -C "$ROOT/backend" build -o "$tmp/couchverse" ./cmd/couchverse
validator=("$tmp/couchverse" validate-hls -q)
failed=0

curl -fsS -c "$jar" -X POST "$API/auth/login" -H 'content-type: application/json' \
  -d "{\"username\":\"${CV_USER:-admin}\",\"password\":\"${CV_PASSWORD:-admin}\"}" >/dev/null

movies=$(curl -fsS -b "$jar" "$API/admin/library?type=movie&limit=200" |
  python3 -c 'import json,sys; [print(t["id"], t["name"]) for t in json.load(sys.stdin)["items"]]')

while read -r id name; do
  for profile in "$ROOT"/contract/fixtures/device-profiles/*.json; do
    device=$(basename "$profile" .json)
    info=$(curl -fsS -b "$jar" -X POST "$API/playback/movie/$id" -H 'content-type: application/json' -d @"$profile")
    read -r tier mode url ladder < <(echo "$info" | python3 -c '
import json, sys
i = json.load(sys.stdin)
print(i.get("tier") or "-", i["mode"], i.get("streamUrl") or "-", i.get("hlsUrl") or "-")')
    printf '%-24s %-16s %-10s %s\n' "$name" "$device" "$tier" "$mode"
    urls=("$url")
    [ "$ladder" != "$url" ] && urls+=("$ladder")
    for u in "${urls[@]}"; do
      [[ "$u" == *.m3u8* ]] || continue
      if ! "${validator[@]}" "$SERVER$u"; then
        failed=1
      fi
      if [[ "$device" == apple-* && "$(uname)" == Darwin ]] && ! swift "$ROOT/scripts/avplayer-probe.swift" "$SERVER$u" --play 3 >"$tmp/avplayer" 2>&1; then
        cat "$tmp/avplayer"
        failed=1
      fi
      if command -v mediastreamvalidator >/dev/null && [[ "$device" == apple-* ]]; then
        mediastreamvalidator "$SERVER$u" || failed=1
      fi
    done
  done
done <<<"$movies"
exit $failed
