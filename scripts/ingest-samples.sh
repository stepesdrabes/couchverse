#!/usr/bin/env bash
# Uploads the sample media (scripts/gen-sample-media.sh) into a running server as
# movies, the way the web's upload queue does, and waits for its jobs to finish.
# usage: SERVER=http://localhost:8080 CV_USER=admin CV_PASSWORD=admin scripts/ingest-samples.sh [samples-dir]
set -euo pipefail

SERVER=${SERVER:-http://localhost:8080}
API=$SERVER/api/v1
DIR=${1:-data/samples}
jar=$(mktemp)
trap 'rm -f "$jar"' EXIT
json() { python3 -c "import json,sys; print(json.load(sys.stdin)$1)"; }

curl -fsS -c "$jar" -X POST "$API/auth/login" -H 'content-type: application/json' \
  -d "{\"username\":\"${CV_USER:-admin}\",\"password\":\"${CV_PASSWORD:-admin}\"}" >/dev/null

find "$DIR/movies" -type f \( -name '*.mp4' -o -name '*.mkv' -o -name '*.webm' -o -name '*.mov' \) -print0 |
  while IFS= read -r -d '' file; do
    name=$(basename "$file")
    size=$(wc -c <"$file" | tr -d ' ')
    body=$(python3 -c 'import json,sys; print(json.dumps({"filename": sys.argv[1], "size": int(sys.argv[2])}))' "$name" "$size")
    id=$(curl -fsS -b "$jar" -X POST "$API/admin/uploads" -H 'content-type: application/json' -d "$body" | json '["id"]')
    curl -fsS -b "$jar" -X PUT "$API/admin/uploads/$id?offset=0" -H 'content-type: application/octet-stream' \
      --data-binary @"$file" >/dev/null
    curl -fsS -b "$jar" -X POST "$API/admin/uploads/$id/complete" -H 'content-type: application/json' \
      -d '{"libraryKind":"movies"}' >/dev/null
    echo "uploaded $name"
  done

printf 'waiting for probes and transcodes'
while true; do
  # the hourly cleanup is always pending; only media jobs count
  busy=$(curl -fsS -b "$jar" "$API/admin/jobs" | python3 -c '
import json, sys
print(sum(1 for j in json.load(sys.stdin) if j["status"] in ("pending", "running") and j["type"] != "cleanup"))')
  [ "$busy" = 0 ] && break
  printf '.'
  sleep 3
done
echo " done"
curl -fsS -b "$jar" "$API/admin/jobs?status=failed" | python3 -c '
import json, sys
for job in json.load(sys.stdin):
    print("failed:", job["type"], job.get("payload"), job.get("error"))'
