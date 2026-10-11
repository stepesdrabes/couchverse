#!/usr/bin/env bash
# Fills assets/ for the Couch TV sample reel from a running Couchverse server, read-only:
# soundtrack excerpts are cut with the server's own ffmpeg (inside the app container) and
# the titles' artwork is copied. Titles are found by name in the server's database, so the
# reel only works against a library that has them. Fonts and the logo come from the web app.
#
#   COUCHVERSE_SSH=user@host tools/couch-tv-samples/fetch-media.sh
set -euo pipefail

: "${COUCHVERSE_SSH:?set COUCHVERSE_SSH=user@host (the server running the Docker stack)}"
APP=${APP_CONTAINER:-couchverse-app-1}
DB=${DB_CONTAINER:-couchverse-db-1}
here=$(cd "$(dirname "$0")" && pwd)
web=$here/../../clients/web
mkdir -p "$here/assets/audio" "$here/assets/art" "$here/assets/fonts"

# name|title|season|episode|offset|duration
# A negative offset counts from the end (movie end credits); excerpts were picked from
# loudness profiles (movies) and fingerprint matches between episodes (series themes).
excerpts='
ironman|Iron Man|||-412|30
pulpfiction|Pulp Fiction|||284|30
harrypotter|Harry Potter a Kámen mudrců|||-402|30
simpsons|Simpsonovi|1|2|20.5|30
southpark|South Park|1|2|106.4|36
dexter|Dexter|1|2|187|34
invincible|INVINCIBLE|1|3|466.3|10.5
'
art='
ironman|Iron Man
pulp|Pulp Fiction
hp1|Harry Potter a Kámen mudrců
dexter|Dexter
simpsons|Simpsonovi
invincible|INVINCIBLE
southpark|South Park
'

# -n keeps ssh off the loops' stdin; sql is the one call that reads its stdin.
remote() { ssh -n -o ConnectTimeout=10 "$COUCHVERSE_SSH" "$@"; }
sql() { ssh -o ConnectTimeout=10 "$COUCHVERSE_SSH" docker exec -i "$DB" psql -U couchverse -d couchverse -Atq -v ON_ERROR_STOP=1; }
quote() { printf "'%s'" "${1//\'/\'\'}"; }

while IFS='|' read -r name title season episode offset duration; do
	[ -n "$name" ] || continue
	if [ -z "$season" ]; then
		where="m.title_id = (select id from titles where name = $(quote "$title") and kind = 'movie')"
	else
		where="m.episode_id = (select e.id from episodes e join seasons s on s.id = e.season_id
			join titles t on t.id = s.title_id where t.name = $(quote "$title")
			and s.season_number = $season and e.episode_number = $episode)"
	fi
	src=$(echo "select l.path || '/' || m.path from media_files m join libraries l on l.id = m.library_id
		where $where order by (m.audio_role = 'primary') desc, m.id limit 1;" | sql)
	[ -n "$src" ] || { echo "skip $name: $title not found" >&2; continue; }
	if [[ $offset == -* ]]; then seek=(-sseof "$offset"); else seek=(-ss "$offset"); fi
	fadeout=$(awk "BEGIN { print $duration - 2.5 }")
	tmp=/tmp/couch-tv-$name.m4a
	remote docker exec "$APP" nice -n 15 ffmpeg -hide_banner -loglevel error -y "${seek[@]}" -t "$duration" \
		-i "$(printf %q "$src")" -map 0:a:0 -vn \
		-af "afade=t=in:d=0.6,afade=t=out:st=$fadeout:d=2.5,loudnorm=I=-16:TP=-1.5:LRA=11" \
		-ar 44100 -ac 2 -c:a aac -b:a 160k "$tmp"
	remote "docker exec $APP sh -c 'cat $tmp && rm -f $tmp'" >"$here/assets/audio/$name.m4a"
	echo "audio $name"
done <<<"$excerpts"

while IFS='|' read -r dir title; do
	[ -n "$dir" ] || continue
	id=$(echo "select id from titles where name = $(quote "$title");" | sql)
	[ -n "$id" ] || { echo "skip art $dir: $title not found" >&2; continue; }
	mkdir -p "$here/assets/art/$dir"
	for kind in backdrop poster; do
		remote docker exec "$APP" cat "/data/artwork/title/$id/$kind.jpg" >"$here/assets/art/$dir/$kind.jpg"
	done
	echo "art $dir"
done <<<"$art"

cp "$web"/node_modules/@fontsource-variable/plus-jakarta-sans/files/plus-jakarta-sans-latin-wght-normal.woff2 \
	"$web"/node_modules/@fontsource-variable/plus-jakarta-sans/files/plus-jakarta-sans-latin-ext-wght-normal.woff2 \
	"$here/assets/fonts/"
cp "$web/static/logo.svg" "$here/assets/"
echo "done: serve with python3 -m http.server -d $here and open http://localhost:8000/"
