#!/usr/bin/env bash
# Serves the built app on a throwaway, seeded database for the Playwright smoke suite
# (clients/web/playwright.config.ts runs it as its webServer). Prints
# "e2e server ready at <url>" once seeded and tears everything down on SIGTERM.
#
#   E2E_DATABASE_URL  a Postgres it may create databases on (default: the dev compose db)
#   E2E_PG_CONTAINER  container whose psql is used when the host has none (couchverse-db-1)
#   E2E_PORT          port to serve on (default: a free one)
#   E2E_SKIP_BUILD=1  reuse backend/bin/couchverse instead of running `make build`
#   E2E_SERVER_LOG    where the server log goes (default: inside the temp data dir)
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
base_url=${E2E_DATABASE_URL:-postgres://couchverse:couchverse@localhost:5432/couchverse}
container=${E2E_PG_CONTAINER:-couchverse-db-1}

log() { echo "e2e: $*" >&2; }

if ! command -v ffmpeg >/dev/null; then
  log "ffmpeg is required (brew install ffmpeg, apt-get install ffmpeg)"
  exit 1
fi

# the dev Mac usually has no psql of its own; the compose container does
psql_() {
  if command -v psql >/dev/null; then
    psql -X -q -v ON_ERROR_STOP=1 "$@"
  else
    docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 "$@"
  fi
}

db=couchverse_e2e_$$
db_url=$(echo "$base_url" | sed -E "s#/[^/?]+(\?|$)#/$db\1#")
data_dir=$(mktemp -d "${TMPDIR:-/tmp}/couchverse-e2e.XXXXXX")
server_log=${E2E_SERVER_LOG:-$data_dir/server.log}
server_pid=

cleanup() {
  if [ -n "$server_pid" ]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  psql_ "$base_url" -c "DROP DATABASE IF EXISTS $db WITH (FORCE)" >/dev/null 2>&1 || true
  rm -rf "$data_dir"
}
trap cleanup EXIT
trap 'exit 0' TERM INT

if [ -z "${E2E_SKIP_BUILD:-}" ]; then
  log "building the web client and the server"
  if ! make -C "$root" build >"$data_dir/build.log" 2>&1; then
    cat "$data_dir/build.log" >&2
    exit 1
  fi
fi

if ! psql_ "$base_url" -c "CREATE DATABASE $db" >/dev/null; then
  log "cannot create a database on $base_url (is it up? docker compose up db -d)"
  exit 1
fi

port=${E2E_PORT:-$(node -e '
  const srv = require("net").createServer().listen(0, "127.0.0.1", () => {
    process.stdout.write(String(srv.address().port));
    srv.close();
  });
')}

DATABASE_URL=$db_url DATA_DIR=$data_dir PORT=$port ADMIN_USERNAME=admin ADMIN_PASSWORD=admin \
  "$root/backend/bin/couchverse" >"$server_log" 2>&1 &
server_pid=$!

# migrations and the admin bootstrap run before the server listens, and the seed needs both
until curl -fs "http://127.0.0.1:$port/healthz" >/dev/null; do
  if ! kill -0 "$server_pid" 2>/dev/null; then
    log "server exited during startup:"
    cat "$server_log" >&2
    exit 1
  fi
  sleep 0.1
done

psql_ "$db_url" <"$root/backend/internal/server/testdata/seed.sql" >/dev/null

# A real clip for the direct-play movie, in the Movies library the server created on
# boot. It runs long enough for the player to pass its 5 s reporting threshold and a
# 10 s skip without reaching the end.
movies_dir=$(psql_ "$db_url" -At -c "SELECT path FROM libraries WHERE kind = 'movies' ORDER BY id LIMIT 1")
movie='Glass Harbor (2025)/Glass Harbor (2025).mp4'
mkdir -p "$movies_dir/$(dirname "$movie")"
ffmpeg -hide_banner -loglevel error -y \
  -f lavfi -i "testsrc2=size=320x180:rate=12:duration=60" \
  -f lavfi -i "sine=frequency=440:sample_rate=48000:duration=60" \
  -c:v libx264 -preset ultrafast -pix_fmt yuv420p -c:a aac -b:a 64k \
  -movflags +faststart -shortest "$movies_dir/$movie"
psql_ "$db_url" -v movie="$movie" -v movie_size="$(wc -c <"$movies_dir/$movie" | tr -d ' ')" \
  <"$root/clients/web/e2e/setup.sql" >/dev/null

placeholder=$data_dir/placeholder.jpg
ffmpeg -hide_banner -loglevel error -y -f lavfi -i "testsrc2=size=640x360:duration=1" \
  -frames:v 1 "$placeholder"

# the seed's artwork and subtitle rows point at files that must exist under DATA_DIR,
# or every image and caption request in the suite would 404
psql_ "$db_url" -At -c "SELECT path FROM artwork" | while read -r rel; do
  mkdir -p "$(dirname "$data_dir/$rel")"
  cp "$placeholder" "$data_dir/$rel"
done
psql_ "$db_url" -At -c "SELECT path FROM subtitles" | while read -r rel; do
  mkdir -p "$(dirname "$data_dir/$rel")"
  printf 'WEBVTT\n\n00:00.000 --> 01:00.000\nThe sea keeps a door.\n' >"$data_dir/$rel"
done

echo "e2e server ready at http://127.0.0.1:$port"
wait "$server_pid"
