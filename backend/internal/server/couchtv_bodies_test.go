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

// TestCouchTvStaleBodies composes breaks, changes what a title shows (its tagline, which the
// bodies of its promo and trailer carry) and composes again: the bodies asked for before the
// change are cancelled and their clips dropped (a pending trailer would count against the
// night's three), the bodies of the titles that did not change stay as they were, and the
// title's new ones are asked for once, to render in the quiet hours.
func TestCouchTvStaleBodies(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	prague, _ := time.LoadLocation("Europe/Prague")
	clock := time.Date(2026, 10, 12, 10, 0, 0, 0, prague)
	now := func() time.Time { return clock }
	jb, set, store := jobs.NewStore(env.pool), settings.NewStore(env.pool), couchtv.NewStore(env.pool)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := env.pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := env.pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	var movie string
	if err := env.pool.QueryRow(ctx, `SELECT name FROM titles WHERE id = $1`, movieID).Scan(&movie); err != nil {
		t.Fatal(err)
	}
	const file = "00000000-0000-4000-8000-000000000501"
	scene := func(start int) string {
		return `{"mediaFileId": "` + file + `", "audioFileId": "` + file + `", "startMs": ` + strconv.Itoa(start) +
			`, "durationMs": 5000, "lines": [{"fromMs": 500, "toMs": 2500, "text": "Hello."}], "score": 0.7, "source": "subtitles"}`
	}
	exec(`INSERT INTO couchtv_titles (title_id, trailer, trailer_source, trailer_version) VALUES ($1, $2, 'auto', $3)`,
		movieID, `[`+scene(600000)+`,`+scene(900000)+`,`+scene(1200000)+`]`, couchtv.TrailerVersion)
	if err := (&couchtv.Scheduler{Store: store, Settings: set, Jobs: jb, Now: now}).Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	if err := set.Set(ctx, "couchtv_render", json.RawMessage(`{"versions": {"ident": 1, "sting": 1, "upNext": 1, "promo": 1,
		"evergreenEnd": 1, "stripPromo": 1, "shortPromo": 1, "lineup": 1, "squeeze": 1, "standby": 1, "trailer": 1}, "seenAt": "2026-10-12T08:00:00Z"}`)); err != nil {
		t.Fatal(err)
	}
	composer := &couchtv.Composer{Store: store, Settings: set, Jobs: jb, Now: now}
	compose := func() {
		t.Helper()
		if err := composer.Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
			t.Fatal(err)
		}
	}
	// queued maps each body whose render waits to its clip, by "kind title"
	queued := func() map[string][]string {
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

	compose()
	before := queued()
	if len(before["promo_body "+movie]) != 1 || len(before["trailer_body "+movie]) != 1 || len(before) < 3 {
		t.Fatalf("bodies queued: %v, want one promo and one trailer for the movie and a promo for another title", before)
	}
	for what, ids := range before {
		if len(ids) != 1 {
			t.Errorf("%s: %d bodies queued", what, len(ids))
		}
	}

	// the movie's tagline changes, and the breaks are composed again around its new bodies
	exec(`UPDATE titles SET tagline = 'Rewritten after the bodies were asked for' WHERE id = $1`, movieID)
	exec(`DELETE FROM couchtv_breaks WHERE kind = 'airing'`)
	compose()
	after := queued()
	for what, ids := range before {
		now := after[what]
		if len(now) != 1 {
			t.Errorf("%s: %v queued after the change, %v before", what, now, ids)
			continue
		}
		changed := what == "promo_body "+movie || what == "trailer_body "+movie
		switch {
		case changed && now[0] == ids[0]:
			t.Errorf("%s: the body asked for before the change is still the one queued", what)
		case !changed && now[0] != ids[0]:
			t.Errorf("%s: a title that did not change got a new body", what)
		case changed:
			if n := count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'cancelled' AND payload->>'clipId' = $2`, couchtv.RenderJob, ids[0]); n != 1 {
				t.Errorf("%s: %d cancelled renders of the stale body", what, n)
			}
			if n := count(`SELECT count(*) FROM couchtv_clips WHERE id = $1`, ids[0]); n != 0 {
				t.Errorf("%s: the stale body is still a clip", what)
			}
		}
	}
	if n := count(`SELECT count(*) FROM couchtv_clips WHERE kind = 'trailer_body'`); n != 1 {
		t.Errorf("%d trailer clips, want the current one", n)
	}
	// the new bodies wait for the quiet hours behind every break, as the old ones did
	if n := count(`SELECT count(*) FROM jobs j JOIN couchtv_clips c ON c.id::text = j.payload->>'clipId'
		WHERE j.type = $1 AND j.status = 'pending' AND c.kind IN ('promo_body', 'trailer_body') AND j.run_at <= $2`,
		couchtv.RenderJob, clock); n != 0 {
		t.Errorf("%d bodies due at once", n)
	}
}
