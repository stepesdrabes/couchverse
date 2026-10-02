#!/usr/bin/env bash
# Plays the native-client flow against a running server: identify it, pair a
# "TV" approved by a signed-in member, fetch home with the device token, then
# play a movie's grant URL in ffprobe and (on macOS) AVFoundation.
# usage: SERVER=http://localhost:8080 CV_USER=nora CV_PASSWORD=admin TITLE_ID=<movie id> scripts/device-smoke.sh
set -euo pipefail

SERVER=${SERVER:?set SERVER}
CV_USER=${CV_USER:?set CV_USER}
CV_PASSWORD=${CV_PASSWORD:?set CV_PASSWORD}
TITLE_ID=${TITLE_ID:?set TITLE_ID}
API=$SERVER/api/v1
json() { python3 -c "import json,sys; print(json.load(sys.stdin)$1)"; }
jar=$(mktemp)
trap 'rm -f "$jar"' EXIT

echo "server:   $(curl -fsS "$API/server" | json '["name"]') (api level $(curl -fsS "$API/server" | json '["apiLevel"]'))"

pairing=$(curl -fsS -X POST "$API/auth/pairings" -H 'content-type: application/json' \
  -d '{"deviceName":"Smoke test TV","platform":"tvos"}')
device_code=$(echo "$pairing" | json '["deviceCode"]')
user_code=$(echo "$pairing" | json '["userCode"]')
echo "pairing:  show $user_code at $SERVER$(echo "$pairing" | json '["verifyPath"]')"

curl -fsS -c "$jar" -X POST "$API/auth/login" -H 'content-type: application/json' \
  -d "{\"username\":\"$CV_USER\",\"password\":\"$CV_PASSWORD\"}" >/dev/null
curl -fsS -b "$jar" -X POST "$API/me/pairings/$user_code/approve" >/dev/null
token=$(curl -fsS -X POST "$API/auth/pairings/poll" -H 'content-type: application/json' \
  -d "{\"deviceCode\":\"$device_code\"}" | json '["device"]["token"]')
auth=(-H "Authorization: Bearer $token")
echo "signed in as $(curl -fsS "${auth[@]}" "$API/auth/me" | json '["username"]')"

echo "home:     $(curl -fsS "${auth[@]}" "$API/home" | json '["rows"].__len__()') rows"

info=$(curl -fsS "${auth[@]}" "$API/playback/movie/$TITLE_ID")
url=$SERVER$(echo "$info" | json '["streamUrl"]')
echo "playback: $(echo "$info" | json '["mode"]') via a grant URL"
echo "ffprobe:  $(ffprobe -v error -show_entries format=format_name,duration -of csv=p=0 "$url")"
if [[ "$(uname)" == Darwin ]]; then
  echo "avplayer: $(swift "$(dirname "$0")/avplayer-probe.swift" "$url")"
fi
echo "ok"
