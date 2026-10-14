package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"couchverse/internal/feature/couchtv"
)

// TestCouchTvTitleHandbacks: what an admin's edit queues for the analysis. A title brought back to
// the channel with older picks is picked again for them alone, with no loudness of the whole film
// measured (a title never analyzed is analyzed in full); shots handed back to the analysis are
// picked again alone; a promo excerpt handed back, which may need the film's loudness, takes the
// full analysis; an edit that hands nothing back queues nothing.
func TestCouchTvTitleHandbacks(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	const shot = `[{"mediaFileId": "` + movieFileID + `", "startMs": 600000, "durationMs": 2000, "score": 1}]`
	// state sets the title's channel row: whether it is excluded, the analysis' and the picks'
	// versions, and who chose the shots and the promo excerpt
	state := func(excluded bool, analyzed, shots, trailer int, shotsSource, promoSource string) {
		t.Helper()
		if _, err := env.pool.Exec(ctx, `DELETE FROM couchtv_titles WHERE title_id = $1`, movieID); err != nil {
			t.Fatal(err)
		}
		if _, err := env.pool.Exec(ctx, `INSERT INTO couchtv_titles (title_id, excluded, analyzed_version, shots_version, trailer_version,
			shots, shots_source, promo_audio_file_id, promo_audio_start_ms, promo_audio_duration_ms, promo_audio_source)
			VALUES ($1, $2, $3, $4, $5, CASE WHEN $6 = '' THEN '[]'::jsonb ELSE $8::jsonb END, nullif($6, ''),
			        CASE WHEN $7 = '' THEN NULL ELSE $9::uuid END, CASE WHEN $7 = '' THEN NULL ELSE 6500000 END,
			        CASE WHEN $7 = '' THEN NULL ELSE 30000 END, nullif($7, ''))`,
			movieID, excluded, analyzed, shots, trailer, shotsSource, promoSource, shot, movieFileID); err != nil {
			t.Fatal(err)
		}
	}
	// put edits the title as the admin does, with whatever the body keeps, and returns the analyses it queued
	put := func(body map[string]any) []couchtv.AnalyzePayload {
		t.Helper()
		if status, raw, _ := env.request(t, env.clients["admin"], "PUT", "/admin/couchtv/titles/"+movieID, body); status != http.StatusOK {
			t.Fatalf("PUT: %d %s", status, raw)
		}
		rows, err := env.pool.Query(ctx, `UPDATE jobs SET status = 'done' WHERE type = $1 AND status = 'pending' AND payload->>'titleId' = $2
			RETURNING payload`, couchtv.AnalyzeJob, movieID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []couchtv.AnalyzePayload
		for rows.Next() {
			var raw []byte
			var p couchtv.AnalyzePayload
			if err := rows.Scan(&raw); err != nil || json.Unmarshal(raw, &p) != nil {
				t.Fatalf("a queued analysis: %s (%v)", raw, err)
			}
			out = append(out, p)
		}
		return out
	}
	want := func(name string, got []couchtv.AnalyzePayload, wants ...couchtv.AnalyzePayload) {
		t.Helper()
		if len(got) != len(wants) {
			t.Errorf("%s: queued %+v, want %+v", name, got, wants)
			return
		}
		for i := range got {
			if got[i] != wants[i] {
				t.Errorf("%s: queued %+v, want %+v", name, got, wants)
			}
		}
	}
	keep := map[string]any{"excluded": false}
	keepShots := map[string]any{"excluded": false, "shots": []map[string]any{{"mediaFileId": movieFileID, "startMs": 600_000, "durationMs": 2_000}}}

	// back to the channel with picks of an older version: those alone, not the film's loudness
	state(true, couchtv.AnalyzeVersion, 0, 0, "auto", "")
	want("coming back with old picks", put(keep), couchtv.AnalyzePayload{TitleID: movieID, PicksOnly: true})
	state(true, 0, 0, 0, "", "")
	want("coming back unanalyzed", put(keep), couchtv.AnalyzePayload{TitleID: movieID, Force: true})
	state(true, couchtv.AnalyzeVersion, couchtv.ShotsVersion, couchtv.TrailerVersion, "auto", "")
	want("coming back with current picks", put(keep))

	// shots handed back are picked again alone; kept, or an admin's own again, they are not
	state(false, couchtv.AnalyzeVersion, couchtv.ShotsVersion, couchtv.TrailerVersion, "admin", "")
	want("handing the shots back", put(keep), couchtv.AnalyzePayload{TitleID: movieID, PicksOnly: true, Shots: true})
	state(false, couchtv.AnalyzeVersion, couchtv.ShotsVersion, couchtv.TrailerVersion, "admin", "")
	want("choosing the shots again", put(keepShots))

	// a promo excerpt handed back may need the film's loudness
	state(false, couchtv.AnalyzeVersion, couchtv.ShotsVersion, couchtv.TrailerVersion, "auto", "admin")
	want("handing the excerpt back", put(keep), couchtv.AnalyzePayload{TitleID: movieID, Force: true})
	state(false, couchtv.AnalyzeVersion, couchtv.ShotsVersion, couchtv.TrailerVersion, "auto", "")
	want("an edit that hands nothing back", put(map[string]any{"excluded": false, "mood": "grit"}))
}
