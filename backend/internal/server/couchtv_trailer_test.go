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

// TestCouchTvTrailers composes breaks with a title that has scenes for a trailer (D43): the
// scenes of a file that is gone are left out, a trailer is asked for once, a few a night, and
// airs once rendered at most once a break and a few hours apart, in a break that has room for it.
func TestCouchTvTrailers(t *testing.T) {
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
	// the movie has four scenes: three of its file (the sound of one from its stream 1) and one of
	// a file that is gone
	const file, gone = "00000000-0000-4000-8000-000000000501", "00000000-0000-4000-8000-000000000599"
	scene := func(mediaFile string, start int, stream string) string {
		return `{"mediaFileId": "` + mediaFile + `", "audioFileId": "` + mediaFile + `", ` + stream + `"startMs": ` + strconv.Itoa(start) +
			`, "durationMs": 5000, "lines": [{"fromMs": 500, "toMs": 2500, "text": "Hello."}], "score": 0.7, "source": "subtitles"}`
	}
	scenes := `[` + scene(file, 600000, `"audioStream": 1, `) + `,` + scene(gone, 700000, "") + `,` + scene(file, 900000, "") + `,` +
		scene(file, 1200000, "") + `]`
	exec(`INSERT INTO couchtv_titles (title_id, trailer, trailer_source, trailer_version) VALUES ($1, $2, 'auto', $3)`,
		movieID, scenes, couchtv.TrailerVersion)

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

	// enough trailers rendered this night already: no more
	for i := range 3 {
		exec(`INSERT INTO couchtv_clips (kind, key, spec, state, needed_at, last_used_at, rendered_at, version)
			VALUES ('trailer_body', $1, '{}', 'ready', now(), now(), now(), 1)`, "queued-"+strconv.Itoa(i))
	}
	compose()
	if n := count(`SELECT count(*) FROM couchtv_clips WHERE kind = 'trailer_body' AND key NOT LIKE 'queued-%'`); n != 0 {
		t.Fatalf("%d trailers asked for beyond the night's three", n)
	}
	exec(`DELETE FROM couchtv_clips WHERE key LIKE 'queued-%'`)
	exec(`DELETE FROM couchtv_breaks WHERE kind = 'airing'`)

	compose()
	var spec couchtv.Spec
	var id string
	if err := env.pool.QueryRow(ctx, `SELECT id, spec FROM couchtv_clips WHERE kind = 'trailer_body'`).Scan(&id, &spec); err != nil {
		t.Fatalf("no trailer asked for: %v", err)
	}
	if spec.Template != "trailer" || spec.Part != "body" || spec.Airing != nil || len(spec.Scenes) != 3 {
		t.Fatalf("the trailer is a %s %s with %d scenes (the file that is gone left out)", spec.Template, spec.Part, len(spec.Scenes))
	}
	if first, second := spec.Scenes[0], spec.Scenes[1]; len(first.Frames) != 125 ||
		first.Frames[0] != "/media/v1/frames/"+file+"/600000_125/0.jpg" ||
		first.Sound.URL != "/media/v1/audio/"+file+"/1/600000_5000.flac" || second.Sound.URL != "/media/v1/audio/"+file+"/900000_5000.flac" ||
		first.Sound.Out != 5 || len(first.Lines) != 1 || first.Lines[0] != [2]float64{0.5, 2.5} {
		t.Errorf("scene 1 has %d frames from %s, its sound %+v; scene 2's sound %s", len(first.Frames), first.Frames[0], first.Sound, second.Sound.URL)
	}
	if n := count(`SELECT count(*) FROM couchtv_clips WHERE kind = 'trailer_body'`); n != 1 {
		t.Errorf("%d trailers asked for", n)
	}
	// asked for once; it waits for the quiet hours behind the breaks that need clips sooner
	if n := count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'pending' AND payload->>'clipId' = $2`, couchtv.RenderJob, id); n != 1 {
		t.Errorf("%d renders of the trailer queued", n)
	}
	compose()
	if n := count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'pending' AND payload->>'clipId' = $2`, couchtv.RenderJob, id); n != 1 {
		t.Errorf("%d renders of the trailer queued after another run", n)
	}

	// rendered (76 s), it airs in the breaks composed afterwards: once a break, an end card behind
	// it, never again within four hours
	exec(`UPDATE couchtv_clips SET state = 'ready', rendered_at = now(), version = 1, duration_ms = 76000 WHERE kind = 'trailer_body'`)
	exec(`DELETE FROM couchtv_breaks WHERE kind = 'airing'`)
	compose()
	if n := count(`SELECT count(*) FROM couchtv_breaks WHERE state = 'failed'`); n != 0 {
		t.Fatalf("%d breaks failed", n)
	}
	rows, err := env.pool.Query(ctx, `SELECT a.starts_at, b.recipe FROM couchtv_breaks b JOIN couchtv_airings a ON a.id = b.airing_id
		WHERE b.recipe->'trailers' IS NOT NULL ORDER BY a.starts_at`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var aired []time.Time
	for rows.Next() {
		var starts time.Time
		var r couchtv.Recipe
		if err := rows.Scan(&starts, &r); err != nil {
			t.Fatal(err)
		}
		bodies, at := 0, 0
		for i, c := range r.Clips {
			if c.At != at {
				t.Fatalf("break at %s: clip at %d, want %d", starts, c.At, at)
			}
			at += c.Frames
			if c.Kind == couchtv.ClipTrailerBody {
				bodies++
				if c.Frames != 1900 || i+1 == len(r.Clips) || r.Clips[i+1].Kind != couchtv.ClipTrailerEnd || r.Clips[i+1].Frames != 100 {
					t.Errorf("break at %s: the trailer is %d frames, then %+v", starts, c.Frames, r.Clips[i+1:])
				}
			}
		}
		if bodies != 1 || len(r.Trailers) != 1 || r.Trailers[0] != movieID || at != r.Frames {
			t.Errorf("break at %s: %d trailers %v over %d of %d frames", starts, bodies, r.Trailers, at, r.Frames)
		}
		for _, o := range aired {
			if starts.Sub(o) < 4*time.Hour {
				t.Errorf("trailers at %s and %s", o.In(prague), starts.In(prague))
			}
		}
		aired = append(aired, starts)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(aired) == 0 {
		t.Error("no break airs the trailer")
	}
	// the break that airs it sounds it whole: the body's sound is its full promo and the end card plays none
	var sounds int
	if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM couchtv_breaks b, jsonb_array_elements(b.recipe->'sound'->'items') it
		WHERE it->'spec'->>'template' = 'trailer' AND it->'spec'->>'part' = 'full'`).Scan(&sounds); err != nil || sounds != len(aired) {
		t.Errorf("%d trailer sounds for %d breaks (%v)", sounds, len(aired), err)
	}

	// with fewer than three scenes left (their files are gone) there is no trailer at all
	exec(`UPDATE media_files SET source_deleted_at = now() WHERE id = $1`, file)
	exec(`DELETE FROM couchtv_breaks WHERE kind = 'airing'`)
	exec(`DELETE FROM couchtv_clips WHERE kind IN ('trailer_body', 'trailer_end')`)
	compose()
	if n := count(`SELECT count(*) FROM couchtv_clips WHERE kind IN ('trailer_body', 'trailer_end')`); n != 0 {
		t.Errorf("%d trailer clips for a title whose scenes are gone", n)
	}
}
