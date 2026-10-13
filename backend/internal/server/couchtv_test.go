package server_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

type airingRow struct {
	id, kind, episodeID string
	starts, ends        time.Time
	locked              bool
}

func couchTvRows(t *testing.T, env *testEnv) []airingRow {
	t.Helper()
	rows, err := env.pool.Query(context.Background(),
		`SELECT id, kind, coalesce(episode_id::text, ''), starts_at, ends_at, locked FROM couchtv_airings ORDER BY starts_at`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []airingRow
	for rows.Next() {
		var r airingRow
		if err := rows.Scan(&r.id, &r.kind, &r.episodeID, &r.starts, &r.ends, &r.locked); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func ids(rows []airingRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.id
	}
	slices.Sort(out)
	return out
}

// TestCouchTvPlanJob runs the couchtv_plan job over the seeded library: it plans a week
// without gaps, queues its next daily run, keeps an unchanged plan's rows, keeps frozen
// days and locked rows the next day, and swaps unprepared programs about to air.
func TestCouchTvPlanJob(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	prague, _ := time.LoadLocation("Europe/Prague")
	clock := time.Date(2026, 10, 12, 10, 0, 0, 0, prague)
	jb := jobs.NewStore(env.pool)
	s := &couchtv.Scheduler{Store: couchtv.NewStore(env.pool), Settings: settings.NewStore(env.pool), Jobs: jb,
		Now: func() time.Time { return clock }}
	run := func() {
		t.Helper()
		if err := s.Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
			t.Fatal(err)
		}
	}

	run()
	rows := couchTvRows(t, env)
	if len(rows) == 0 {
		t.Fatal("nothing planned")
	}
	if want := time.Date(2026, 10, 12, 6, 0, 0, 0, prague); !rows[0].starts.Equal(want) {
		t.Errorf("the plan starts at %s, want %s", rows[0].starts, want)
	}
	// a week from now falls in Monday's broadcast day, planned whole, up to Tuesday 06:00
	if end := rows[len(rows)-1].ends; end.Before(time.Date(2026, 10, 20, 4, 0, 0, 0, prague)) || end.After(time.Date(2026, 10, 20, 6, 0, 0, 0, prague)) {
		t.Errorf("the plan ends at %s", end)
	}
	for i := 1; i < len(rows); i++ {
		if !rows[i].starts.Equal(rows[i-1].ends) {
			t.Fatalf("row %d starts at %s, the row before ends at %s", i, rows[i].starts, rows[i-1].ends)
		}
	}
	var runAt time.Time
	if err := env.pool.QueryRow(ctx, `SELECT run_at FROM jobs WHERE type = $1 AND payload->>'day' = '2026-10-13'`, couchtv.PlanJob).Scan(&runAt); err != nil {
		t.Fatalf("the next daily run: %v", err)
	}
	if want := time.Date(2026, 10, 13, 4, 0, 0, 0, prague); !runAt.Equal(want) {
		t.Errorf("the next run is at %s, want %s", runAt, want)
	}

	// planning an unchanged week again keeps every row
	run()
	if again := couchTvRows(t, env); !slices.Equal(ids(again), ids(rows)) {
		t.Errorf("an unchanged replan replaced rows: %d rows, then %d", len(rows), len(again))
	}

	// the next morning the frozen days keep their rows, and so does a locked one
	var locked string
	if err := env.pool.QueryRow(ctx, `UPDATE couchtv_airings SET locked = true
		WHERE id = (SELECT id FROM couchtv_airings WHERE kind = 'program' AND starts_at > $1 ORDER BY starts_at LIMIT 1)
		RETURNING id`, time.Date(2026, 10, 16, 20, 0, 0, 0, prague)).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	clock = time.Date(2026, 10, 13, 4, 0, 0, 0, prague)
	frozenEnd := time.Date(2026, 10, 14, 6, 0, 0, 0, prague)
	var frozen []airingRow
	for _, r := range rows {
		if r.starts.Before(frozenEnd) {
			frozen = append(frozen, r)
		}
	}
	run()
	next := couchTvRows(t, env)
	var stillFrozen []airingRow
	for _, r := range next {
		if r.starts.Before(frozenEnd) {
			stillFrozen = append(stillFrozen, r)
		}
	}
	if !slices.Equal(ids(stillFrozen), ids(frozen)) {
		t.Errorf("the frozen days changed")
	}
	if !slices.ContainsFunc(next, func(r airingRow) bool { return r.id == locked && r.locked }) {
		t.Errorf("the locked row is gone")
	}
	if end := next[len(next)-1].ends; end.Before(clock.AddDate(0, 0, 7)) {
		t.Errorf("the plan ends at %s, less than a week ahead", end)
	}

	// the pilot gets prepared (couchtv_prepare found an H.264 rung to air): an
	// unprepared episode about to air gives way to it
	if _, err := env.pool.Exec(ctx, `INSERT INTO couchtv_media (media_file_id, video, on_air_ms) VALUES ($1, '720p', 2516000)`,
		hdrFileID); err != nil {
		t.Fatal(err)
	}
	run()
	swapped := couchTvRows(t, env)
	for i, r := range swapped {
		if r.kind == "program" && !r.starts.Before(clock) && r.starts.Before(clock.Add(6*time.Hour)) && r.episodeID == episode2ID {
			t.Errorf("unprepared episode still airs at %s", r.starts)
		}
		if i > 0 && !r.starts.Equal(swapped[i-1].ends) {
			t.Fatalf("a swap left a gap or overlap at %s", r.starts)
		}
	}
	if !slices.ContainsFunc(swapped, func(r airingRow) bool {
		return r.episodeID == pilotID && !r.starts.Before(clock) && r.starts.Before(clock.Add(6*time.Hour))
	}) {
		t.Error("the prepared pilot never stood in")
	}
}

// TestCouchTvAiringEdits drives the schedule editor: a removed program stays off its day,
// a locked one survives a replan, an inserted one is flowed around, moved and removed, a
// break next to a changed program loses its composition, and the schedule stays
// gapless throughout.
func TestCouchTvAiringEdits(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	prague, _ := time.LoadLocation("Europe/Prague")
	at := func(days, hour, minute int) time.Time {
		n := time.Now().In(prague)
		return time.Date(n.Year(), n.Month(), n.Day()+days, hour, minute, 0, 0, prague)
	}
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		status, out, _ := env.request(t, env.clients["admin"], method, path, body)
		if status != want {
			t.Fatalf("%s %s: %d, want %d: %s", method, path, status, want, out)
		}
		return out
	}
	type program struct{ id, key, source, fill string }
	first := func(after time.Time) program {
		t.Helper()
		var p program
		if err := env.pool.QueryRow(ctx, `SELECT id, coalesce(episode_id, title_id)::text, source, coalesce(fill, '')
			FROM couchtv_airings WHERE kind = 'program' AND state <> 'replaced' AND starts_at > $1 ORDER BY starts_at LIMIT 1`,
			after).Scan(&p.id, &p.key, &p.source, &p.fill); err != nil {
			t.Fatal(err)
		}
		return p
	}
	gapless := func(step string) {
		t.Helper()
		rows, err := env.pool.Query(ctx, `SELECT starts_at, ends_at FROM couchtv_airings WHERE state <> 'replaced' ORDER BY starts_at`)
		if err != nil {
			t.Fatal(err)
		}
		var prev time.Time
		for rows.Next() {
			var s, e time.Time
			if err := rows.Scan(&s, &e); err != nil {
				t.Fatal(err)
			}
			if !prev.IsZero() && !s.Equal(prev) {
				rows.Close()
				t.Fatalf("%s: a gap or an overlap at %s", step, s.In(prague))
			}
			prev = e
		}
		rows.Close()
	}
	call("POST", "/admin/couchtv/days/"+couchTvDay(1)+"/regenerate", nil, 200)
	gapless("planned")

	// a removed program is kept as a removal and stays off its broadcast day
	removed := first(at(2, 12, 0))
	call("PATCH", "/admin/couchtv/airings/"+removed.id, map[string]any{"action": "remove"}, 200)
	var state string
	var still int
	if err := env.pool.QueryRow(ctx, `SELECT state FROM couchtv_airings WHERE id = $1`, removed.id).Scan(&state); err != nil || state != "replaced" {
		t.Fatalf("the removed program is %q (%v)", state, err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM couchtv_airings WHERE kind = 'program' AND state <> 'replaced'
		AND coalesce(episode_id, title_id)::text = $1 AND starts_at >= $2 AND starts_at < $3`,
		removed.key, at(2, 6, 0), at(3, 6, 0)).Scan(&still); err != nil || still != 0 {
		t.Errorf("the removed program still airs %d times that day (%v)", still, err)
	}
	gapless("removed")

	// a locked program survives the day's regeneration
	locked := first(at(4, 12, 0))
	call("PATCH", "/admin/couchtv/airings/"+locked.id, map[string]any{"action": "lock"}, 200)
	call("POST", "/admin/couchtv/days/"+couchTvDay(1)+"/regenerate", nil, 200)
	var isLocked bool
	if err := env.pool.QueryRow(ctx, `SELECT locked FROM couchtv_airings WHERE id = $1`, locked.id).Scan(&isLocked); err != nil || !isLocked {
		t.Errorf("the locked program did not survive a replan (%v)", err)
	}

	// an inserted episode airs at its time, off the 5-minute grid, and the day flows around it
	var inserted couchtv.CouchTvAiring
	out := call("POST", "/admin/couchtv/airings", map[string]any{"titleId": seriesID, "episodeId": episode2ID,
		"startsAt": at(5, 14, 7)}, 201)
	if err := json.Unmarshal(out, &inserted); err != nil {
		t.Fatal(err)
	}
	if !inserted.StartsAt.Equal(at(5, 14, 7)) || inserted.Source != "admin" || inserted.Fill != "reruns" {
		t.Errorf("inserted %+v", inserted)
	}
	gapless("inserted")

	// pinned to the movie half an hour later, it keeps its id and becomes a matinee
	call("PATCH", "/admin/couchtv/airings/"+inserted.ID, map[string]any{"action": "pin", "titleId": movieID,
		"startsAt": at(5, 15, 0)}, 200)
	var title, fill string
	var starts time.Time
	if err := env.pool.QueryRow(ctx, `SELECT title_id::text, fill, starts_at FROM couchtv_airings WHERE id = $1`, inserted.ID).
		Scan(&title, &fill, &starts); err != nil || title != movieID || fill != "matinee" || !starts.Equal(at(5, 15, 0)) {
		t.Errorf("pinned: %s %s %s (%v)", title, fill, starts, err)
	}
	gapless("pinned")
	call("POST", "/admin/couchtv/airings", map[string]any{"titleId": seriesID, "episodeId": episode2ID,
		"startsAt": at(5, 15, 30)}, 409)

	// removing an admin's program deletes it; the planner fills its place
	call("PATCH", "/admin/couchtv/airings/"+inserted.ID, map[string]any{"action": "remove"}, 200)
	var gone bool
	if err := env.pool.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM couchtv_airings WHERE id = $1)`, inserted.ID).Scan(&gone); err != nil || !gone {
		t.Errorf("the removed admin program is still stored (%v)", err)
	}
	gapless("admin program removed")

	// the break announcing a program that changes loses its composition
	target := first(at(6, 12, 0))
	var breakID string
	if err := env.pool.QueryRow(ctx, `SELECT b.id FROM couchtv_airings b JOIN couchtv_airings p ON p.starts_at = b.ends_at
		WHERE b.kind = 'break' AND p.id = $1`, target.id).Scan(&breakID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO couchtv_breaks (airing_id, state) VALUES ($1, 'composed')`, breakID); err != nil {
		t.Fatal(err)
	}
	other := pilotID
	if target.key == pilotID {
		other = episode2ID
	}
	call("PATCH", "/admin/couchtv/airings/"+target.id, map[string]any{"action": "pin", "titleId": seriesID, "episodeId": other}, 200)
	var composed int
	if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM couchtv_breaks WHERE airing_id = $1`, breakID).Scan(&composed); err != nil || composed != 0 {
		t.Errorf("the break before the changed program keeps its composition (%v)", err)
	}
	gapless("swapped in")
}

// TestCouchTvConfigStored checks the config round trip: defaults until saved, the
// guest link hash kept out of the API and kept across saves.
func TestCouchTvConfigStored(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	set := settings.NewStore(env.pool)
	if got := couchtv.LoadConfig(ctx, set); got.TimeZone != "Europe/Prague" || len(got.Template.Slots) == 0 {
		t.Fatalf("defaults: %+v", got)
	}
	if err := set.Set(ctx, "couchtv", json.RawMessage(`{"timeZone": "America/New_York", "breakMinSeconds": 30, "guestLinkHash": "abc"}`)); err != nil {
		t.Fatal(err)
	}
	cfg := couchtv.DefaultConfig()
	cfg.BreakMinSeconds = 40
	status, body, _ := env.request(t, env.clients["admin"], "PUT", "/admin/couchtv", cfg)
	if status != 200 {
		t.Fatalf("update: %d %s", status, body)
	}
	var raw map[string]any
	stored, _ := set.Get(ctx, "couchtv")
	_ = json.Unmarshal(stored, &raw)
	if raw["guestLinkHash"] != "abc" || raw["breakMinSeconds"] != float64(40) {
		t.Errorf("stored %s", stored)
	}
	var overview map[string]map[string]any
	_ = json.Unmarshal(body, &overview)
	if _, ok := overview["config"]["guestLinkHash"]; ok {
		t.Error("the API shows the guest link hash")
	}
}

// TestCouchTvRandomPlayback checks that the channel reads whether a series may play at
// random, and that the title editor queues a plan run when an admin changes it but not
// for a save that leaves it as it was.
func TestCouchTvRandomPlayback(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	st := couchtv.NewStore(env.pool)
	random := func() bool {
		t.Helper()
		lib, err := st.Library(ctx)
		if err != nil {
			t.Fatal(err)
		}
		i := slices.IndexFunc(lib.Series, func(s couchtv.Series) bool { return s.ID == seriesID })
		if i < 0 {
			t.Fatal("the seeded series is not on the channel")
		}
		return lib.Series[i].Random
	}
	queued := func() int {
		t.Helper()
		var n int
		if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE type = $1 AND status = 'pending'`,
			couchtv.PlanJob).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	save := func(body map[string]any) {
		t.Helper()
		if status, out, _ := env.request(t, env.clients["admin"], "PATCH", "/admin/titles/"+seriesID, body); status != 200 {
			t.Fatalf("save: %d %s", status, out)
		}
	}
	if !random() || queued() != 0 {
		t.Fatalf("seeded: random %v with %d plan runs queued", random(), queued())
	}
	save(map[string]any{"overview": "Radio waves, shuffled.", "allowRandomPlayback": true})
	if queued() != 0 {
		t.Error("a save that keeps the flag queued a plan run")
	}
	save(map[string]any{"allowRandomPlayback": false})
	if random() || queued() != 1 {
		t.Errorf("turned off: random %v with %d plan runs queued", random(), queued())
	}
}

// TestCouchTvPrune checks what the hourly cleanup keeps of the renders: an aired break
// loses its package, an unused clip goes unless a waiting break uses it, only the newest
// assembled evergreen break stays, and extracted media unused for weeks goes.
func TestCouchTvPrune(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	now := time.Now()
	dir := func(rel string, age time.Duration) string {
		t.Helper()
		path := filepath.Join(env.cfg.DataDir, rel)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		return rel
	}
	file := func(rel string, age time.Duration) string {
		t.Helper()
		dir(filepath.Dir(rel), 0)
		path := filepath.Join(env.cfg.DataDir, rel)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		return rel
	}
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(env.cfg.DataDir, rel))
		return err == nil
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := env.pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	aired, waiting := dir("couchtv/breaks/aired", 0), dir("couchtv/breaks/waiting", 0)
	oldClip, usedClip := dir("couchtv/clips/old", 0), dir("couchtv/clips/used", 0)
	oldGreen, newGreen := dir("couchtv/breaks/old-evergreen", 0), dir("couchtv/breaks/new-evergreen", 0)
	staleFrames, freshFrames := dir("couchtv/frames/f/0_75", 20*24*time.Hour), dir("couchtv/frames/f/40_75", time.Hour)
	// soundtrack previews: the movie's pick, one the admin heard lately, one long ago
	picked := file("couchtv/soundtracks/itunes-1001.m4a", 30*24*time.Hour)
	heard, unheard := file("couchtv/soundtracks/deezer-2002.mp3", time.Hour), file("couchtv/soundtracks/itunes-1003.m4a", 30*24*time.Hour)
	exec(`INSERT INTO couchtv_titles (title_id, soundtrack_id, soundtrack_source, soundtrack_path, soundtrack_start_ms,
		soundtrack_duration_ms) VALUES ($1, $2, 'auto', $3, 0, 30000)`, movieID, soundtrackID, picked)
	staleSound, freshSound := file("couchtv/sounds/stale.wav", 3*24*time.Hour), file("couchtv/sounds/mixed.wav", 20*time.Hour)
	// a clip rendered before its raw sound was kept out of its package
	usedPlaylist, usedRaw := file(usedClip+"/master.m3u8", 0), file(usedClip+"/sound.f32", 0)

	exec(`INSERT INTO couchtv_airings (id, starts_at, ends_at, kind) VALUES
		('00000000-0000-4000-8000-00000000b001', $1, $2, 'break'), ('00000000-0000-4000-8000-00000000b002', $3, $4, 'break')`,
		now.Add(-3*time.Hour), now.Add(-2*time.Hour), now.Add(time.Hour), now.Add(time.Hour+time.Minute))
	exec(`INSERT INTO couchtv_clips (id, kind, key, state, path, last_used_at) VALUES
		('00000000-0000-4000-8000-00000000c001', 'sting', 'old', 'ready', $1, $3),
		('00000000-0000-4000-8000-00000000c002', 'sting', 'used', 'ready', $2, $3)`, oldClip, usedClip, now.Add(-20*24*time.Hour))
	exec(`INSERT INTO couchtv_breaks (airing_id, kind, state, path, recipe, created_at) VALUES
		('00000000-0000-4000-8000-00000000b001', 'airing', 'ready', $1, '{}', $5),
		('00000000-0000-4000-8000-00000000b002', 'airing', 'composed', $2, '{"clips": [{"clipId": "00000000-0000-4000-8000-00000000c002"}]}', $5),
		(NULL, 'evergreen', 'ready', $3, '{}', $5 - interval '1 day'),
		(NULL, 'evergreen', 'ready', $4, '{}', $5)`, aired, waiting, oldGreen, newGreen, now)

	if err := couchtv.Prune(ctx, couchtv.NewStore(env.pool), env.cfg.DataDir, now); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]bool{aired: false, waiting: true, oldClip: false, usedClip: true,
		oldGreen: false, newGreen: true, staleFrames: false, freshFrames: true, picked: true, heard: true, unheard: false,
		staleSound: false, freshSound: true, usedPlaylist: true, usedRaw: false} {
		if exists(rel) != want {
			t.Errorf("%s kept: %v, want %v", rel, exists(rel), want)
		}
	}
	var clips, evergreens int
	if err := env.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM couchtv_clips WHERE key IN ('old', 'used')),
		(SELECT count(*) FROM couchtv_breaks WHERE kind = 'evergreen')`).Scan(&clips, &evergreens); err != nil {
		t.Fatal(err)
	}
	if clips != 1 || evergreens != 1 {
		t.Errorf("%d clips and %d evergreen breaks left", clips, evergreens)
	}
}

// TestCouchTvComposeJob plans the seeded library and composes its breaks: nothing before a
// render sidecar has published its templates, then every break of the next 24 hours with a
// recipe as long as the break, its clips requested once and queued for rendering (soon
// ones at once, the rest in the night's quiet hours), the
// evergreen break and the standby card; once clips are rendered their breaks are queued for
// assembly and the evergreen break is recomposed with promos; a changed template recomposes
// the breaks that use its rendered clips, once.
func TestCouchTvComposeJob(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	prague, _ := time.LoadLocation("Europe/Prague")
	clock := time.Date(2026, 10, 12, 10, 0, 0, 0, prague)
	now := func() time.Time { return clock }
	jb, set := jobs.NewStore(env.pool), settings.NewStore(env.pool)
	store := couchtv.NewStore(env.pool)
	plan := &couchtv.Scheduler{Store: store, Settings: set, Jobs: jb, Now: now}
	if err := plan.Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	composer := &couchtv.Composer{Store: store, Settings: set, Jobs: jb, Now: now}
	compose := func() {
		t.Helper()
		if err := composer.Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
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

	compose()
	if n := count(`SELECT count(*) FROM couchtv_breaks`); n != 0 {
		t.Fatalf("%d breaks composed before a renderer published its templates", n)
	}
	versions := `{"ident": 1, "sting": 1, "upNext": 1, "promo": 1, "evergreenEnd": 1, "stripPromo": 1, "shortPromo": 1,
		"lineup": 1, "squeeze": 1, "standby": 1}`
	if err := set.Set(ctx, "couchtv_render", json.RawMessage(`{"versions": `+versions+`, "seenAt": "2026-10-12T08:00:00Z"}`)); err != nil {
		t.Fatal(err)
	}

	compose()
	window := []any{clock, clock.Add(24 * time.Hour)}
	breaks := count(`SELECT count(*) FROM couchtv_airings WHERE kind = 'break' AND starts_at >= $1 AND starts_at < $2`, window...)
	if composed := count(`SELECT count(*) FROM couchtv_breaks b JOIN couchtv_airings a ON a.id = b.airing_id
		WHERE b.state = 'composed'`); breaks == 0 || composed != breaks {
		t.Fatalf("%d of %d breaks composed", composed, breaks)
	}
	// every recipe fills its break to the frame with clips back to back
	slotted := 0
	rows, err := env.pool.Query(ctx, `SELECT a.starts_at, a.ends_at, b.recipe FROM couchtv_breaks b
		JOIN couchtv_airings a ON a.id = b.airing_id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var starts, ends time.Time
		var r couchtv.Recipe
		if err := rows.Scan(&starts, &ends, &r); err != nil {
			t.Fatal(err)
		}
		at := 0
		for _, c := range r.Clips {
			if c.At != at {
				t.Fatalf("break at %s: clip at %d, want %d", starts, c.At, at)
			}
			at += c.Frames
		}
		if want := int(ends.Sub(starts).Round(40*time.Millisecond) / (40 * time.Millisecond)); at != want || r.Frames != want {
			t.Errorf("break at %s: %d frames of clips, the break is %d", starts, at, want)
		}
		if len(r.Clips) == 0 || r.Clips[len(r.Clips)-1].Kind != couchtv.ClipUpNext {
			t.Errorf("break at %s does not end on up next: %+v", starts, r.Clips)
		}
		// bulletins: the news on the hour from 06:00, in slots made of whole clips at the
		// break's loudness in whole LU
		onTheHour := ends.In(prague).Minute() == 0 && (ends.In(prague).Hour() >= 7 || ends.In(prague).Hour() == 0)
		hasNews := slices.ContainsFunc(r.Slots, func(s couchtv.RecipeSlot) bool { return s.Kind == couchtv.BulletinNews })
		if onTheHour && r.Frames >= 2000 && !hasNews || !onTheHour && hasNews && ends.In(prague).Hour() != 6 {
			t.Errorf("break at %s ending %s: slots %+v", starts.In(prague), ends.In(prague), r.Slots)
		}
		if hasNews {
			slotted++
		}
		for _, s := range r.Slots {
			edges := 0
			for _, c := range r.Clips {
				if c.At == s.At || c.At+c.Frames == s.At+s.Frames {
					edges++
				}
			}
			if edges != 2 || s.Loudness != math.Round(r.Sound.Loudness) {
				t.Errorf("break at %s: slot %+v over clips %+v", starts, s, r.Clips)
			}
		}
	}
	rows.Close()
	if slotted == 0 {
		t.Error("no break carries the news")
	}
	if n := count(`SELECT count(*) FROM couchtv_breaks WHERE kind = 'evergreen' AND state = 'composed'`); n != 1 {
		t.Errorf("%d evergreen breaks", n)
	}
	if n := count(`SELECT count(*) FROM couchtv_clips WHERE kind = 'standby'`); n != 1 {
		t.Errorf("%d standby cards", n)
	}
	clips := count(`SELECT count(*) FROM couchtv_clips WHERE key NOT LIKE 'seeded-%'`)
	renders := count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'pending'`, couchtv.RenderJob)
	if clips == 0 || renders != clips {
		t.Errorf("%d clips, %d renders queued", clips, renders)
	}
	if n := count(`SELECT count(*) FROM couchtv_clips WHERE kind = 'promo_body'`); n == 0 {
		t.Error("no promo body requested")
	}
	// a render starts at once for what airs any time (the evergreen break) or within three
	// hours, else at 01:00 (Prague) but no later than three hours before its first break
	quiet := time.Date(2026, 10, 13, 1, 0, 0, 0, prague)
	rows, err = env.pool.Query(ctx, `SELECT j.run_at, min(coalesce(a.starts_at, 'epoch')) FROM jobs j
		JOIN couchtv_breaks b ON b.recipe->'clips' @> jsonb_build_array(jsonb_build_object('clipId', j.payload->>'clipId'))
		LEFT JOIN couchtv_airings a ON a.id = b.airing_id
		WHERE j.type = $1 AND j.status = 'pending' GROUP BY j.id`, couchtv.RenderJob)
	if err != nil {
		t.Fatal(err)
	}
	soon, waiting := 0, 0
	for rows.Next() {
		var runAt, airs time.Time
		if err := rows.Scan(&runAt, &airs); err != nil {
			t.Fatal(err)
		}
		want := quiet
		if latest := airs.Add(-3 * time.Hour); latest.Before(want) {
			want = latest
		}
		if airs.Unix() == 0 || want.Before(clock) {
			want = clock
		}
		if !runAt.Equal(want) {
			t.Errorf("the render for a break at %s runs at %s, want %s", airs.In(prague), runAt.In(prague), want.In(prague))
		}
		if want.Equal(clock) {
			soon++
		} else {
			waiting++
		}
	}
	rows.Close()
	if soon == 0 || waiting == 0 {
		t.Errorf("%d renders at once, %d waiting for the quiet hours", soon, waiting)
	}
	// renders queued before the quiet hours existed (due at once) are timed again by the next
	// run, and promo bodies, which no break needs yet, wait for them too
	if _, err := env.pool.Exec(ctx, `UPDATE jobs SET run_at = $2 WHERE type = $1 AND status = 'pending'`,
		couchtv.RenderJob, clock.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	compose()
	bodies := count(`SELECT count(*) FROM jobs j JOIN couchtv_clips c ON c.id::text = j.payload->>'clipId'
		WHERE j.type = $1 AND j.status = 'pending' AND c.kind = 'promo_body' AND j.run_at = $2`, couchtv.RenderJob, quiet)
	if n := count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'pending' AND run_at > $2`,
		couchtv.RenderJob, clock); bodies == 0 || n != waiting+bodies {
		t.Errorf("%d renders timed again for the quiet hours, want %d and %d promo bodies", n, waiting, bodies)
	}

	// a break that leaves the schedule takes its own clips' renders with it
	var gone, upNext string
	if err := env.pool.QueryRow(ctx, `SELECT a.id, rc->>'clipId' FROM couchtv_airings a
		JOIN couchtv_breaks b ON b.airing_id = a.id, jsonb_array_elements(b.recipe->'clips') rc
		WHERE rc->>'kind' = 'upnext' ORDER BY a.starts_at DESC LIMIT 1`).Scan(&gone, &upNext); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`UPDATE couchtv_airings SET state = 'replaced' WHERE id = $1`,
		`DELETE FROM couchtv_breaks WHERE airing_id = $1`} {
		if _, err := env.pool.Exec(ctx, q, gone); err != nil {
			t.Fatal(err)
		}
	}
	compose()
	if n := count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'cancelled' AND payload->>'clipId' = $2`,
		couchtv.RenderJob, upNext); n != 1 {
		t.Errorf("the up next of a break that left the schedule: %d cancelled renders", n)
	}
	renders = count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'pending'`, couchtv.RenderJob)

	// an unchanged schedule composes nothing new
	compose()
	if n := count(`SELECT count(*) FROM couchtv_clips WHERE key NOT LIKE 'seeded-%'`); n != clips {
		t.Errorf("a second run requested %d clips more", n-clips)
	}
	if n := count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'pending'`, couchtv.RenderJob); n != renders {
		t.Errorf("a second run queued %d renders more", n-renders)
	}

	// rendered clips (promo bodies 16 s) queue the breaks that use them for assembly (no break
	// uses the seed's clips)
	if _, err := env.pool.Exec(ctx, `UPDATE couchtv_clips SET state = 'ready', rendered_at = now(), version = 1,
		duration_ms = CASE WHEN duration_ms = 0 THEN 16000 ELSE duration_ms END`); err != nil {
		t.Fatal(err)
	}
	var identID string
	if err := env.pool.QueryRow(ctx, `SELECT id FROM couchtv_clips WHERE kind = 'ident_short' AND key NOT LIKE 'seeded-%'`).Scan(&identID); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(couchtv.RenderPayload{ClipID: identID})
	r := &couchtv.Renderer{Store: store, Jobs: jb, Settings: set}
	if err := r.Render(ctx, &jobs.Job{Payload: payload}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	if n := count(`SELECT count(*) FROM jobs WHERE type = $1`, couchtv.AssembleJob); n == 0 {
		t.Error("no assembly queued")
	}
	compose()
	if n := count(`SELECT count(*) FROM couchtv_breaks b WHERE kind = 'evergreen'
		AND recipe->'clips' @> '[{"kind": "promo_body"}]'`); n != 1 {
		t.Errorf("%d evergreen breaks with promos", n)
	}

	// the sidecar comes back with ident 2: the airing breaks that use a rendered ident 1 are
	// composed again around new idents, and the rest keep their compositions
	usesIdent := `SELECT count(*) FROM couchtv_breaks b WHERE b.kind = 'airing' AND EXISTS (
		SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(b.recipe->'clips') = 'array' THEN b.recipe->'clips' ELSE '[]' END) rc
		JOIN couchtv_clips c ON c.id::text = rc->>'clipId'
		WHERE c.kind IN ('ident_full', 'ident_short') AND c.state = $1)`
	withIdent, composed := count(usesIdent, "ready"), count(`SELECT count(*) FROM couchtv_breaks WHERE kind = 'airing'`)
	if withIdent == 0 {
		t.Fatal("no break uses an ident")
	}
	versions = strings.Replace(versions, `"ident": 1`, `"ident": 2`, 1)
	if err := set.Set(ctx, "couchtv_render", json.RawMessage(`{"versions": `+versions+`, "seenAt": "2026-10-12T09:00:00Z"}`)); err != nil {
		t.Fatal(err)
	}
	compose()
	if old, fresh := count(usesIdent, "ready"), count(usesIdent, "pending"); old != 0 || fresh != withIdent {
		t.Errorf("after the ident changed: %d breaks on the old ident, %d on a new one, want 0 and %d", old, fresh, withIdent)
	}
	if n := count(`SELECT count(*) FROM couchtv_breaks WHERE kind = 'airing'`); n != composed {
		t.Errorf("%d airing breaks, %d before", n, composed)
	}
	clips = count(`SELECT count(*) FROM couchtv_clips WHERE key NOT LIKE 'seeded-%'`)
	compose()
	if n := count(`SELECT count(*) FROM couchtv_clips WHERE key NOT LIKE 'seeded-%'`); n != clips {
		t.Errorf("composing again after the change requested %d clips more", n-clips)
	}
}
