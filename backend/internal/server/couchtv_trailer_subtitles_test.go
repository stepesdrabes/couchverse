package server_test

import (
	"context"
	"encoding/json"
	"testing"

	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
)

// TestCouchTvTrailerSubtitlesGained: a trailer is cut from subtitles only, so a title whose pick is
// empty is picked again, once, when its file gains subtitles in the language of its dialogue (not
// when it gains ones in another), and an admin's trailer is not picked over but is stamped with the
// picker's version so the backfill leaves it alone.
func TestCouchTvTrailerSubtitlesGained(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	jb, store := jobs.NewStore(env.pool), couchtv.NewStore(env.pool)
	analyzer := &couchtv.Analyzer{Store: store, Jobs: jb, FFmpegPath: "ffmpeg", DataDir: env.cfg.DataDir}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := env.pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	const file = "00000000-0000-4000-8000-000000000501"
	exec(`UPDATE titles SET status = 'published' WHERE id = $1`, movieID)
	exec(`DELETE FROM subtitles WHERE media_file_id = $1`, file)
	exec(`INSERT INTO audio_streams (media_file_id, stream_index, lang, is_default) VALUES ($1, 1, 'en', true)`, file)
	exec(`INSERT INTO couchtv_titles (title_id, analyzed_version, shots_version, trailer, trailer_source, trailer_version, trailer_subs)
		VALUES ($1, $2, $3, '[]', 'auto', $4, '')`, movieID, couchtv.AnalyzeVersion, couchtv.ShotsVersion, couchtv.TrailerVersion)

	// backfill runs the analyzer's backfill and returns the payloads it queued for the movie
	backfill := func() []string {
		t.Helper()
		if err := analyzer.Handle(ctx, &jobs.Job{Payload: json.RawMessage(`{}`)}, func(int) {}); err != nil {
			t.Fatal(err)
		}
		rows, err := env.pool.Query(ctx, `SELECT payload::text FROM jobs WHERE type = $1 AND status = 'pending'
			AND payload->>'titleId' = $2 ORDER BY id`, couchtv.AnalyzeJob, movieID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				t.Fatal(err)
			}
			out = append(out, p)
		}
		return out
	}
	// pickAgain runs the queued job, as the runner does
	pickAgain := func() {
		t.Helper()
		var id int64
		var payload []byte
		if err := env.pool.QueryRow(ctx, `UPDATE jobs SET status = 'done' WHERE id = (SELECT id FROM jobs WHERE type = $1 AND status = 'pending'
			AND payload->>'titleId' = $2 ORDER BY id LIMIT 1) RETURNING id, payload`, couchtv.AnalyzeJob, movieID).Scan(&id, &payload); err != nil {
			t.Fatal(err)
		}
		if err := analyzer.Handle(ctx, &jobs.Job{ID: id, Payload: payload}, func(int) {}); err != nil {
			t.Fatal(err)
		}
	}
	subs := func() string {
		t.Helper()
		var s string
		if err := env.pool.QueryRow(ctx, `SELECT trailer_subs FROM couchtv_titles WHERE title_id = $1`, movieID).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	if got := backfill(); len(got) != 0 {
		t.Fatalf("a title with an empty pick and no subtitles was queued: %v", got)
	}
	// subtitles in the language of the dialogue: picked again, once
	exec(`INSERT INTO subtitles (media_file_id, lang, label, source, forced, path) VALUES ($1, 'en', 'English', 'uploaded', false, 'subtitles/a/en.vtt')`, file)
	got := backfill()
	var queued couchtv.AnalyzePayload
	if len(got) != 1 || json.Unmarshal([]byte(got[0]), &queued) != nil ||
		queued != (couchtv.AnalyzePayload{TitleID: movieID, PicksOnly: true, Trailer: true}) {
		t.Fatalf("after English subtitles: %v", got)
	}
	if got := backfill(); len(got) != 1 {
		t.Errorf("the backfill queued the pick twice: %v", got)
	}
	pickAgain()
	if s := subs(); s != "subtitles/a/en.vtt" {
		t.Errorf("the pick read %q", s)
	}
	var version int
	if err := env.pool.QueryRow(ctx, `SELECT trailer_version FROM couchtv_titles WHERE title_id = $1`, movieID).Scan(&version); err != nil || version != couchtv.TrailerVersion {
		t.Errorf("trailer version %d (%v)", version, err)
	}
	if got := backfill(); len(got) != 0 {
		t.Fatalf("a title whose empty pick read its subtitles was queued again: %v", got)
	}

	// subtitles in another language, or forced ones, change nothing; a second English track does
	exec(`INSERT INTO subtitles (media_file_id, lang, label, source, forced, path) VALUES
		($1, 'cs', 'Czech', 'uploaded', false, 'subtitles/a/cs.vtt'), ($1, 'en', 'Forced', 'embedded', true, 'subtitles/a/forced.vtt')`, file)
	if got := backfill(); len(got) != 0 {
		t.Fatalf("Czech and forced subtitles queued a pick: %v", got)
	}
	exec(`INSERT INTO subtitles (media_file_id, lang, label, source, forced, path) VALUES ($1, 'en', 'SDH', 'uploaded', false, 'subtitles/a/sdh.vtt')`, file)
	if got := backfill(); len(got) != 1 {
		t.Fatalf("a second English track: %v", got)
	}
	pickAgain()
	if s := subs(); s != "subtitles/a/en.vtt,subtitles/a/sdh.vtt" {
		t.Errorf("the pick read %q", s)
	}

	// an admin's trailer is not picked over, and its version is stamped, so no start queues it
	exec(`UPDATE couchtv_titles SET trailer_source = 'admin', trailer_version = 0, trailer = '[]' WHERE title_id = $1`, movieID)
	exec(`UPDATE jobs SET status = 'done' WHERE type = $1 AND status = 'pending'`, couchtv.AnalyzeJob)
	if err := analyzer.Handle(ctx, &jobs.Job{Payload: json.RawMessage(`{}`)}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	pickAgain()
	var source string
	if err := env.pool.QueryRow(ctx, `SELECT trailer_source, trailer_version FROM couchtv_titles WHERE title_id = $1`, movieID).Scan(&source, &version); err != nil ||
		source != "admin" || version != couchtv.TrailerVersion {
		t.Errorf("the admin's trailer: source %q at version %d (%v)", source, version, err)
	}
	if got := backfill(); len(got) != 0 {
		t.Errorf("the backfill queued an admin's trailer: %v", got)
	}
}
