package server_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// bodies is a seeded day, its schedule planned, with a trailer picked for the movie, whose
// breaks the test composes at a clock it moves.
type bodies struct {
	movie   string
	clock   time.Time
	exec    func(query string, args ...any)
	count   func(query string, args ...any) int
	compose func()
	// recompose composes every break again, as a changed title or a failed clip makes it
	recompose func()
	// queued maps each body whose render waits to its clips, by "kind title"
	queued func() map[string][]string
}

func newBodies(t *testing.T) *bodies {
	t.Helper()
	env := newTestEnv(t)
	ctx := context.Background()
	prague, _ := time.LoadLocation("Europe/Prague")
	b := &bodies{clock: time.Date(2026, 10, 12, 10, 0, 0, 0, prague)}
	now := func() time.Time { return b.clock }
	jb, set, store := jobs.NewStore(env.pool), settings.NewStore(env.pool), couchtv.NewStore(env.pool)
	b.exec = func(query string, args ...any) {
		t.Helper()
		if _, err := env.pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	b.count = func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := env.pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if err := env.pool.QueryRow(ctx, `SELECT name FROM titles WHERE id = $1`, movieID).Scan(&b.movie); err != nil {
		t.Fatal(err)
	}
	const file = "00000000-0000-4000-8000-000000000501"
	scene := func(start int) string {
		return `{"mediaFileId": "` + file + `", "audioFileId": "` + file + `", "startMs": ` + strconv.Itoa(start) +
			`, "durationMs": 5000, "lines": [{"fromMs": 500, "toMs": 2500, "text": "Hello."}], "score": 0.7, "source": "subtitles"}`
	}
	b.exec(`INSERT INTO couchtv_titles (title_id, trailer, trailer_source, trailer_version) VALUES ($1, $2, 'auto', $3)`,
		movieID, `[`+scene(600000)+`,`+scene(900000)+`,`+scene(1200000)+`]`, couchtv.TrailerVersion)
	if err := (&couchtv.Scheduler{Store: store, Settings: set, Jobs: jb, Now: now}).Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	if err := set.Set(ctx, "couchtv_render", json.RawMessage(`{"versions": {"ident": 1, "sting": 1, "upNext": 1, "promo": 1,
		"evergreenEnd": 1, "stripPromo": 1, "shortPromo": 1, "lineup": 1, "squeeze": 1, "standby": 1, "trailer": 1}, "seenAt": "2026-10-12T08:00:00Z"}`)); err != nil {
		t.Fatal(err)
	}
	composer := &couchtv.Composer{Store: store, Settings: set, Jobs: jb, Now: now}
	b.compose = func() {
		t.Helper()
		if err := composer.Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
			t.Fatal(err)
		}
	}
	b.recompose = func() {
		t.Helper()
		b.exec(`DELETE FROM couchtv_breaks WHERE kind = 'airing'`)
		b.compose()
	}
	b.queued = func() map[string][]string {
		t.Helper()
		rows, err := env.pool.Query(ctx, `SELECT c.kind || ' ' || coalesce(c.spec->'title'->>'name', ''), c.id FROM couchtv_clips c
			JOIN jobs j ON j.type = $1 AND j.status = 'pending' AND j.payload->>'clipId' = c.id::text
			WHERE c.kind IN ('promo_body', 'trailer_body') ORDER BY c.created_at, c.id`, couchtv.RenderJob)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string][]string{}
		for rows.Next() {
			var what, id string
			if err := rows.Scan(&what, &id); err != nil {
				t.Fatal(err)
			}
			out[what] = append(out[what], id)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	return b
}

// TestCouchTvStaleBodies composes breaks, changes what a title shows (its tagline, which the
// bodies of its promo and trailer carry) and composes again: the bodies asked for before the
// change are cancelled and their clips dropped (a pending trailer would count against the
// night's three), the bodies of the titles that did not change stay as they were, and the
// title's new ones are asked for once, to render in the quiet hours.
func TestCouchTvStaleBodies(t *testing.T) {
	b := newBodies(t)
	b.compose()
	before := b.queued()
	if len(before["promo_body "+b.movie]) != 1 || len(before["trailer_body "+b.movie]) != 1 || len(before) < 3 {
		t.Fatalf("bodies queued: %v, want one promo and one trailer for the movie and a promo for another title", before)
	}
	for what, ids := range before {
		if len(ids) != 1 {
			t.Errorf("%s: %d bodies queued", what, len(ids))
		}
	}

	// the movie's tagline changes, and the breaks are composed again around its new bodies
	b.exec(`UPDATE titles SET tagline = 'Rewritten after the bodies were asked for' WHERE id = $1`, movieID)
	b.recompose()
	after := b.queued()
	for what, ids := range before {
		now := after[what]
		if len(now) != 1 {
			t.Errorf("%s: %v queued after the change, %v before", what, now, ids)
			continue
		}
		changed := what == "promo_body "+b.movie || what == "trailer_body "+b.movie
		switch {
		case changed && now[0] == ids[0]:
			t.Errorf("%s: the body asked for before the change is still the one queued", what)
		case !changed && now[0] != ids[0]:
			t.Errorf("%s: a title that did not change got a new body", what)
		case changed:
			if n := b.count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'cancelled' AND payload->>'clipId' = $2`, couchtv.RenderJob, ids[0]); n != 1 {
				t.Errorf("%s: %d cancelled renders of the stale body", what, n)
			}
			if n := b.count(`SELECT count(*) FROM couchtv_clips WHERE id = $1`, ids[0]); n != 0 {
				t.Errorf("%s: the stale body is still a clip", what)
			}
		}
	}
	if n := b.count(`SELECT count(*) FROM couchtv_clips WHERE kind = 'trailer_body'`); n != 1 {
		t.Errorf("%d trailer clips, want the current one", n)
	}
	// the new bodies wait for the quiet hours behind every break, as the old ones did
	if n := b.count(`SELECT count(*) FROM jobs j JOIN couchtv_clips c ON c.id::text = j.payload->>'clipId'
		WHERE j.type = $1 AND j.status = 'pending' AND c.kind IN ('promo_body', 'trailer_body') AND j.run_at <= $2`,
		couchtv.RenderJob, b.clock); n != 0 {
		t.Errorf("%d bodies due at once", n)
	}
}

// TestCouchTvTrailerCap: at most three trailers are asked for in 18 hours, counting those
// rendered in that time, whatever they were for, and those waiting or rendering that a title still
// asks for; a stale one that waits does not hold a place, nor does a failed one.
func TestCouchTvTrailerCap(t *testing.T) {
	b := newBodies(t)
	clips := func(state string) {
		t.Helper()
		for i := range 3 {
			b.exec(`INSERT INTO couchtv_clips (kind, key, spec, state, needed_at, last_used_at, rendered_at, version)
				VALUES ('trailer_body', $1, '{}', $2, now(), now(), CASE WHEN $2 = 'ready' THEN $3::timestamptz END, 1)`,
				state+"-"+strconv.Itoa(i), state, b.clock)
		}
	}
	trailers := func() int {
		return b.count(`SELECT count(*) FROM couchtv_clips WHERE kind = 'trailer_body' AND key !~ '^(pending|failed|ready)-'`)
	}
	// three stale clips waiting, three that failed: none holds the cap
	clips("pending")
	clips("failed")
	b.compose()
	if n := trailers(); n != 1 {
		t.Fatalf("%d trailers asked for with only stale and failed ones in the count", n)
	}
	// three rendered in the last hours, for any title: the cap holds
	b.exec(`DELETE FROM couchtv_clips WHERE key !~ '^(pending|failed)-' AND kind = 'trailer_body'`)
	clips("ready")
	b.recompose()
	if n := trailers(); n != 0 {
		t.Errorf("%d trailers asked for beyond the night's three", n)
	}
	// 18 hours on, they do not count
	b.clock = b.clock.Add(19 * time.Hour)
	b.recompose()
	if n := trailers(); n != 1 {
		t.Errorf("%d trailers asked for after the night", n)
	}
}

// TestCouchTvFailedBodiesBackOff: a body that failed is tried again once in six hours, as a break
// is, however often the breaks are composed: a render that fails every time must not take every
// night's quiet hours.
func TestCouchTvFailedBodiesBackOff(t *testing.T) {
	b := newBodies(t)
	b.compose()
	asked := b.queued()
	if len(asked) < 3 {
		t.Fatalf("bodies queued: %v", asked)
	}
	b.exec(`UPDATE couchtv_clips SET state = 'failed', error = 'out of memory', updated_at = $1 WHERE kind IN ('promo_body', 'trailer_body')`, b.clock)
	b.exec(`UPDATE jobs SET status = 'failed' WHERE type = $1 AND status = 'pending' AND payload->>'clipId' IN (
		SELECT id::text FROM couchtv_clips WHERE kind IN ('promo_body', 'trailer_body'))`, couchtv.RenderJob)

	failed := b.clock
	for _, hours := range []time.Duration{1, 4, 5} {
		b.clock = failed.Add(hours * time.Hour)
		b.recompose()
		if q := b.queued(); len(q) != 0 {
			t.Fatalf("%d hours after failing, the bodies are queued again: %v", hours, q)
		}
	}
	b.clock = failed.Add(6*time.Hour + time.Minute)
	b.recompose()
	again := b.queued()
	if len(again) != len(asked) {
		t.Fatalf("after six hours: %v queued again, want %v", again, asked)
	}
	for what, ids := range asked {
		if len(again[what]) != 1 || again[what][0] != ids[0] {
			t.Errorf("%s: %v, want the same clip %v", what, again[what], ids)
		}
	}
}

// TestCouchTvVetoedTrailerRender: when an admin vetoes a title's trailer (its scenes emptied), the
// render queued for it is cancelled and its clip dropped, not timed again for the quiet hours: it
// would run for a quarter of an hour on the Pi and its pending row would count toward the night's
// three, which could keep the title's next trailer from being asked for.
func TestCouchTvVetoedTrailerRender(t *testing.T) {
	b := newBodies(t)
	b.compose()
	asked := b.queued()
	trailer := asked["trailer_body "+b.movie]
	if len(trailer) != 1 {
		t.Fatalf("bodies queued: %v, want a trailer for the movie", asked)
	}
	b.exec(`UPDATE couchtv_titles SET trailer = '[]', trailer_source = 'admin' WHERE title_id = $1`, movieID)
	b.compose()
	if n := b.count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'cancelled' AND payload->>'clipId' = $2`, couchtv.RenderJob, trailer[0]); n != 1 {
		t.Errorf("%d cancelled renders of the vetoed trailer", n)
	}
	if n := b.count(`SELECT count(*) FROM couchtv_clips WHERE kind = 'trailer_body'`); n != 0 {
		t.Errorf("%d trailer clips are left to be counted", n)
	}
	if got := b.queued(); len(got["trailer_body "+b.movie]) != 0 || len(got["promo_body "+b.movie]) != 1 {
		t.Errorf("queued after the veto: %v", got)
	}
}
