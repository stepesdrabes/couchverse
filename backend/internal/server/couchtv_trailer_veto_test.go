package server_test

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"testing"
	"time"

	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// vetoScenes are four scenes of the seeded movie's file for a trailer, as the picker stores them.
func vetoScenes() []couchtv.CouchTvTrailerScene {
	scene := func(start int, source string, lines ...string) couchtv.CouchTvTrailerScene {
		s := couchtv.CouchTvTrailerScene{MediaFileID: movieFileID, AudioFileID: movieFileID, StartMs: start, DurationMs: 5000, Score: 0.7,
			Source: source, Lines: []couchtv.CouchTvTrailerLine{}}
		for i, text := range lines {
			s.Lines = append(s.Lines, couchtv.CouchTvTrailerLine{FromMs: 500 + 1500*i, ToMs: 1800 + 1500*i, Text: text})
		}
		return s
	}
	return []couchtv.CouchTvTrailerScene{
		scene(600_000, "subtitles", "Who are you?", "Nobody leaves this world alive."),
		scene(900_000, "subtitles", "Where is the key?"),
		scene(1_200_000, "speech"),
		scene(1_500_000, "subtitles", "There is a price for everything."),
	}
}

// frameStart picks, from the URL of a scene's first frame, where the scene starts in its file (ms).
var frameStart = regexp.MustCompile(`/frames/[0-9a-f-]{36}/(\d+)_`)

func vetoRef(start int) map[string]any {
	return map[string]any{"mediaFileId": movieFileID, "startMs": start}
}

func vetoKeep(starts ...int) map[string]any {
	keep := []map[string]any{}
	for _, s := range starts {
		keep = append(keep, vetoRef(s))
	}
	return map[string]any{"excluded": false, "trailer": map[string]any{"keep": keep}}
}

// TestCouchTvTrailerVeto is the admin's word on a title's trailer: the scenes as the admin sees
// them, a subset kept, none, a pick of a scene the trailer has not got (409), and the trailer
// handed back, which has the picker choose again and nothing else. An admin's pick is stamped
// with the picker's version, and only the breaks that have not gone on air and air the title's
// trailer are composed again.
func TestCouchTvTrailerVeto(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
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
	scenes, err := json.Marshal(vetoScenes())
	if err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO couchtv_titles (title_id, trailer, trailer_source, trailer_version) VALUES ($1, $2, 'auto', $3)`,
		movieID, scenes, couchtv.TrailerVersion)

	titlePath := "/admin/couchtv/titles/" + movieID
	get := func() couchtv.CouchTvTitle {
		t.Helper()
		status, body, _ := env.request(t, env.clients["admin"], "GET", titlePath, nil)
		if status != 200 {
			t.Fatalf("get: %d %s", status, body)
		}
		var title couchtv.CouchTvTitle
		if err := json.Unmarshal(body, &title); err != nil {
			t.Fatal(err)
		}
		return title
	}
	put := func(status int, body any) couchtv.CouchTvTitle {
		t.Helper()
		got, raw, _ := env.request(t, env.clients["admin"], "PUT", titlePath, body)
		if got != status {
			t.Fatalf("put %v: %d, want %d: %s", body, got, status, raw)
		}
		var title couchtv.CouchTvTitle
		if status == 200 {
			if err := json.Unmarshal(raw, &title); err != nil {
				t.Fatal(err)
			}
		}
		return title
	}
	startsOf := func(title couchtv.CouchTvTitle) []int {
		out := []int{}
		for _, c := range title.Trailer {
			out = append(out, c.StartMs)
		}
		return out
	}
	analyses := func() (n, forced, picksOnly int) {
		rows, err := env.pool.Query(ctx, `SELECT payload FROM jobs WHERE type = $1 AND status = 'pending'`, couchtv.AnalyzeJob)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var p couchtv.AnalyzePayload
			if err := rows.Scan(&p); err != nil {
				t.Fatal(err)
			}
			n++
			if p.Force {
				forced++
			}
			if p.PicksOnly {
				picksOnly++
			}
		}
		return n, forced, picksOnly
	}
	stored := func() (source *string, version int, starts []int) {
		var raw []byte
		if err := env.pool.QueryRow(ctx, `SELECT trailer_source, trailer_version, trailer FROM couchtv_titles WHERE title_id = $1`, movieID).
			Scan(&source, &version, &raw); err != nil {
			t.Fatal(err)
		}
		var list []couchtv.CouchTvTrailerScene
		if err := json.Unmarshal(raw, &list); err != nil {
			t.Fatal(err)
		}
		for _, s := range list {
			starts = append(starts, s.StartMs)
		}
		return source, version, starts
	}

	// the admin sees the scenes of the picker's trailer: where, how long, what is said, how found
	title := get()
	if title.TrailerSource != "auto" || title.TrailerMinScenes != 3 || !slices.Equal(startsOf(title), []int{600_000, 900_000, 1_200_000, 1_500_000}) {
		t.Fatalf("the trailer: %s, %d scenes at least, %v", title.TrailerSource, title.TrailerMinScenes, startsOf(title))
	}
	if c := title.Trailer[0]; c.MediaFileID != movieFileID || c.DurationMs != 5000 || c.Source != "subtitles" ||
		!slices.Equal(c.Lines, []string{"Who are you?", "Nobody leaves this world alive."}) {
		t.Errorf("scene 1: %+v", c)
	}
	if c := title.Trailer[2]; c.Source != "speech" || c.Lines == nil || len(c.Lines) != 0 {
		t.Errorf("a scene found by listening: %+v", c)
	}

	// breaks composed with the trailer: two to come, one on air already, one gone by, and one to
	// come without a trailer. A pick composes again the breaks to come that air it, and none else.
	now := time.Now()
	type composed struct {
		name     string
		starts   time.Time
		trailers []string
		onAir    bool
		dropped  bool
	}
	breaks := []composed{
		{name: "to come", starts: now.Add(2 * time.Hour), trailers: []string{movieID}, dropped: true},
		{name: "to come later", starts: now.Add(30 * time.Hour), trailers: []string{movieID}, dropped: true},
		{name: "to come without a trailer", starts: now.Add(4 * time.Hour), trailers: []string{seriesID}},
		{name: "on air", starts: now.Add(-time.Minute), trailers: []string{movieID}, onAir: true},
		{name: "pinned to the live stream ahead", starts: now.Add(time.Minute), trailers: []string{movieID}, onAir: true},
		{name: "gone by", starts: now.Add(-3 * time.Hour), trailers: []string{movieID}},
	}
	ids := map[string]string{}
	for _, b := range breaks {
		var id string
		if err := env.pool.QueryRow(ctx, `INSERT INTO couchtv_airings (starts_at, ends_at, kind, origin, feed)
			VALUES ($1, $2, 'break', CASE WHEN $3 THEN '{}'::jsonb END, CASE WHEN $3 THEN '{}'::jsonb END) RETURNING id`,
			b.starts, b.starts.Add(5*time.Minute), b.onAir).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[b.name] = id
		recipe, _ := json.Marshal(couchtv.Recipe{Frames: 7500, Promos: []string{}, Trailers: b.trailers})
		exec(`INSERT INTO couchtv_breaks (airing_id, kind, recipe, state) VALUES ($1, 'airing', $2, 'composed')`, id, recipe)
	}
	composeJobs := func() int {
		return count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'pending' AND payload = '{}'::jsonb`, couchtv.ComposeJob)
	}
	if composeJobs() != 0 {
		t.Fatal("a compose job is queued before any pick")
	}

	// keeping some scenes: the others are gone from the trailer, the admin's pick is stamped with the
	// picker's version (the backfill does not queue the title) and nothing is analyzed
	title = put(200, vetoKeep(1_500_000, 600_000, 900_000))
	if title.TrailerSource != "admin" || !slices.Equal(startsOf(title), []int{600_000, 900_000, 1_500_000}) {
		t.Fatalf("after keeping three: %s %v (the scenes keep the trailer's order)", title.TrailerSource, startsOf(title))
	}
	if source, version, starts := stored(); source == nil || *source != "admin" || version != couchtv.TrailerVersion || !slices.Equal(starts, startsOf(title)) {
		t.Errorf("stored: %v at version %d, %v", source, version, starts)
	}
	if n, _, _ := analyses(); n != 0 {
		t.Errorf("%d analyses queued by a pick", n)
	}
	for _, b := range breaks {
		composedAgain := count(`SELECT count(*) FROM couchtv_breaks WHERE airing_id = $1`, ids[b.name]) == 0
		if composedAgain != b.dropped {
			t.Errorf("the break %s: composition dropped %v, want %v", b.name, composedAgain, b.dropped)
		}
	}
	if composeJobs() != 1 {
		t.Errorf("%d compose jobs queued to compose the dropped breaks again, want 1", composeJobs())
	}
	if stale, err := (couchtv.NewStore(env.pool)).StaleTitles(ctx, couchtv.AnalyzeVersion, couchtv.ShotsVersion, couchtv.TrailerVersion); err != nil {
		t.Fatal(err)
	} else if slices.ContainsFunc(stale, func(s couchtv.StaleTitle) bool { return s.ID == movieID && s.PicksOnly }) {
		t.Error("the backfill queues the title again after the admin's pick")
	}

	// the same pick again changes nothing, so no break is composed again
	exec(`INSERT INTO couchtv_breaks (airing_id, kind, recipe, state) VALUES ($1, 'airing', $2, 'composed')`, ids["to come"],
		[]byte(`{"trailers": ["`+movieID+`"]}`))
	put(200, vetoKeep(600_000, 900_000, 1_500_000))
	if count(`SELECT count(*) FROM couchtv_breaks WHERE airing_id = $1`, ids["to come"]) != 1 {
		t.Error("an unchanged pick composed a break again")
	}

	// a scene the trailer has not got (it was picked again since the admin looked), or has in
	// another file, is refused whole: nothing changes
	for _, body := range []map[string]any{
		vetoKeep(600_000, 1_200_000), // dropped by the first pick
		vetoKeep(600_040),            // no scene starts there
		{"excluded": false, "trailer": map[string]any{"keep": []map[string]any{{"mediaFileId": hdrFileID, "startMs": 600_000}}}},
	} {
		put(409, body)
	}
	if _, _, starts := stored(); !slices.Equal(starts, []int{600_000, 900_000, 1_500_000}) {
		t.Errorf("a refused pick changed the trailer: %v", starts)
	}
	status, raw, _ := env.request(t, env.clients["admin"], "PUT", titlePath, vetoKeep(600_040))
	var problem struct{ Error struct{ Code string } }
	if err := json.Unmarshal(raw, &problem); err != nil || status != 409 || problem.Error.Code != couchtv.ErrTrailerSceneGone {
		t.Errorf("the refusal: %d %s (%v)", status, raw, err)
	}

	// handing the trailer back has the picker choose again, the trailer alone: the scenes the
	// admin kept stay until it has, and no breaks are composed again
	title = put(200, map[string]any{"excluded": false})
	if title.TrailerSource != "auto" || !slices.Equal(startsOf(title), []int{600_000, 900_000, 1_500_000}) {
		t.Errorf("after handing back: %s %v", title.TrailerSource, startsOf(title))
	}
	if source, version, _ := stored(); source == nil || *source != "auto" || version != 0 {
		t.Errorf("handed back: %v at version %d, which must be due for a pick", source, version)
	}
	if n, forced, picksOnly := analyses(); n != 1 || forced != 0 || picksOnly != 1 {
		t.Errorf("handing back queued %d analyses, %d forced, %d picks only: want one pick of the trailer, no whole analysis", n, forced, picksOnly)
	}
	// handing back what is the picker's already does nothing
	put(200, map[string]any{"excluded": false})
	if n, _, _ := analyses(); n != 1 {
		t.Errorf("%d analyses after handing back twice", n)
	}

	// no trailer for the title: an admin's pick of no scenes, which the picker never overwrites
	title = put(200, vetoKeep())
	if title.TrailerSource != "admin" || title.Trailer == nil || len(title.Trailer) != 0 {
		t.Errorf("no trailer: %s %v (the scenes must be an empty list)", title.TrailerSource, title.Trailer)
	}
	if source, _, starts := stored(); source == nil || *source != "admin" || len(starts) != 0 {
		t.Errorf("stored: %v %v", source, starts)
	}

	// a newer picker leaves an admin's scenes alone, and the title is not due for it again
	exec(`UPDATE couchtv_titles SET trailer_version = 0, analyzed_version = $2, shots_version = $3 WHERE title_id = $1`,
		movieID, couchtv.AnalyzeVersion, couchtv.ShotsVersion)
	analyzer := &couchtv.Analyzer{Store: couchtv.NewStore(env.pool), Jobs: jobs.NewStore(env.pool), DataDir: env.cfg.DataDir}
	if err := analyzer.Handle(ctx, &jobs.Job{Payload: json.RawMessage(`{"titleId":"` + movieID + `","picksOnly":true}`)}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	if source, version, starts := stored(); source == nil || *source != "admin" || version != couchtv.TrailerVersion || len(starts) != 0 {
		t.Errorf("after a new pick: %v at version %d with %d scenes", source, version, len(starts))
	}

	// the admin's other choices are not touched by a word on the trailer
	title = put(200, map[string]any{"excluded": false, "mood": "grit", "trailer": map[string]any{"keep": []any{}}})
	if title.Mood == nil || *title.Mood != "grit" || title.TrailerSource != "admin" {
		t.Errorf("mood %v, trailer %s", title.Mood, title.TrailerSource)
	}
}

// TestCouchTvTrailerVetoComposes: the composer reads the scenes the title has now, so a trailer
// an admin cut is another clip (its key changes), the old one airs no more, and a title with no
// trailer is asked for none.
func TestCouchTvTrailerVetoComposes(t *testing.T) {
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
	scenes, err := json.Marshal(vetoScenes())
	if err != nil {
		t.Fatal(err)
	}
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
		exec(`DELETE FROM couchtv_breaks WHERE kind = 'airing'`)
		if err := composer.Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
			t.Fatal(err)
		}
	}
	// the trailer clips asked for so far, with their scenes' starts, oldest first
	type clip struct {
		id     string
		starts []int
	}
	clips := func() []clip {
		t.Helper()
		rows, err := env.pool.Query(ctx, `SELECT id, spec FROM couchtv_clips WHERE kind = 'trailer_body' ORDER BY created_at, id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []clip
		for rows.Next() {
			var c clip
			var spec couchtv.Spec
			if err := rows.Scan(&c.id, &spec); err != nil {
				t.Fatal(err)
			}
			for _, s := range spec.Scenes {
				start, err := strconv.Atoi(frameStart.FindStringSubmatch(s.Frames[0])[1])
				if err != nil {
					t.Fatal(err)
				}
				c.starts = append(c.starts, start)
			}
			out = append(out, c)
		}
		return out
	}
	// the clip each break that airs a trailer plays
	aired := func() []string {
		t.Helper()
		rows, err := env.pool.Query(ctx, `SELECT rc->>'clipId' FROM couchtv_breaks b, jsonb_array_elements(b.recipe->'clips') rc
			WHERE rc->>'kind' = 'trailer_body'`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			out = append(out, id)
		}
		return out
	}
	render := func(id string) {
		exec(`UPDATE couchtv_clips SET state = 'ready', rendered_at = now(), version = 1, duration_ms = 76000 WHERE id = $1`, id)
	}
	put := func(body any) {
		t.Helper()
		if status, raw, _ := env.request(t, env.clients["admin"], "PUT", "/admin/couchtv/titles/"+movieID, body); status != 200 {
			t.Fatalf("put: %d %s", status, raw)
		}
	}

	// the picker's trailer: four scenes, rendered, aired
	compose()
	first := clips()
	if len(first) != 1 || !slices.Equal(first[0].starts, []int{600_000, 900_000, 1_200_000, 1_500_000}) {
		t.Fatalf("the trailer asked for: %+v", first)
	}
	render(first[0].id)
	compose()
	if got := aired(); len(got) == 0 || slices.ContainsFunc(got, func(id string) bool { return id != first[0].id }) {
		t.Fatalf("the breaks air clips %v, want only the picker's trailer %s", got, first[0].id)
	}

	// the admin drops the third scene: another trailer, asked for under its own key; until it is
	// rendered no break airs a trailer, so the dropped scene airs from none
	put(vetoKeep(600_000, 900_000, 1_500_000))
	compose()
	all := clips()
	if len(all) != 2 || !slices.Equal(all[1].starts, []int{600_000, 900_000, 1_500_000}) {
		t.Fatalf("after dropping a scene the clips are %+v", all)
	}
	if got := aired(); len(got) != 0 {
		t.Errorf("the breaks air %v while the cut trailer is not rendered", got)
	}
	render(all[1].id)
	compose()
	if got := aired(); len(got) == 0 || slices.ContainsFunc(got, func(id string) bool { return id != all[1].id }) {
		t.Errorf("the breaks air clips %v, want only the cut trailer %s", got, all[1].id)
	}

	// fewer than three scenes make no trailer either: none is asked for and none airs, though a
	// trailer of the title is rendered
	put(vetoKeep(600_000, 900_000))
	compose()
	if got := clips(); len(got) != 2 {
		t.Errorf("a trailer was asked for two scenes: %+v", got)
	}
	if got := aired(); len(got) != 0 {
		t.Errorf("the breaks air %v for two scenes", got)
	}

	// no trailer for the title
	put(vetoKeep())
	compose()
	if got := clips(); len(got) != 2 {
		t.Errorf("a trailer was asked for a title with none: %+v", got)
	}
	if got := aired(); len(got) != 0 {
		t.Errorf("the breaks air %v for a title with no trailer", got)
	}
	var recipes int
	if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM couchtv_breaks WHERE jsonb_array_length(coalesce(recipe->'trailers', '[]')) > 0`).Scan(&recipes); err != nil || recipes != 0 {
		t.Errorf("%d breaks list a trailer (%v)", recipes, err)
	}
}
