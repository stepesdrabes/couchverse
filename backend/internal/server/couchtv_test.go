package server_test

import (
	"context"
	"encoding/json"
	"slices"
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

	// the pilot gets prepared (its HEVC source copied, an H.264 rung): an unprepared
	// episode about to air gives way to it
	if _, err := env.pool.Exec(ctx, `INSERT INTO transcode_variants (media_file_id, name, mode, status, format) VALUES
		($1, 'source', 'copy', 'ready', 'fmp4'), ($1, '720p', 'transcode', 'ready', 'fmp4')`, hdrFileID); err != nil {
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

// TestCouchTvComposeJob plans the seeded library and composes its breaks: nothing before a
// render sidecar has published its templates, then every break of the next 48 hours with a
// recipe as long as the break, its clips requested once and queued for rendering, the
// evergreen break and the standby card; once clips are rendered their breaks are queued for
// assembly and the evergreen break is recomposed with promos.
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
	window := []any{clock, clock.Add(48 * time.Hour)}
	breaks := count(`SELECT count(*) FROM couchtv_airings WHERE kind = 'break' AND starts_at >= $1 AND starts_at < $2`, window...)
	if composed := count(`SELECT count(*) FROM couchtv_breaks b JOIN couchtv_airings a ON a.id = b.airing_id
		WHERE b.state = 'composed'`); breaks == 0 || composed != breaks {
		t.Fatalf("%d of %d breaks composed", composed, breaks)
	}
	// every recipe fills its break to the frame with clips back to back
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
	}
	rows.Close()
	if n := count(`SELECT count(*) FROM couchtv_breaks WHERE kind = 'evergreen' AND state = 'composed'`); n != 1 {
		t.Errorf("%d evergreen breaks", n)
	}
	if n := count(`SELECT count(*) FROM couchtv_clips WHERE kind = 'standby'`); n != 1 {
		t.Errorf("%d standby cards", n)
	}
	clips := count(`SELECT count(*) FROM couchtv_clips`)
	renders := count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'pending'`, couchtv.RenderJob)
	if clips == 0 || renders != clips {
		t.Errorf("%d clips, %d renders queued", clips, renders)
	}
	if n := count(`SELECT count(*) FROM couchtv_clips WHERE kind = 'promo_body'`); n == 0 {
		t.Error("no promo body requested")
	}

	// an unchanged schedule composes nothing new
	compose()
	if n := count(`SELECT count(*) FROM couchtv_clips`); n != clips {
		t.Errorf("a second run requested %d clips more", n-clips)
	}
	if n := count(`SELECT count(*) FROM jobs WHERE type = $1 AND status = 'pending'`, couchtv.RenderJob); n != renders {
		t.Errorf("a second run queued %d renders more", n-renders)
	}

	// rendered clips (promo bodies 16 s) queue the breaks that use them for assembly
	if _, err := env.pool.Exec(ctx, `UPDATE couchtv_clips SET state = 'ready', rendered_at = now(),
		duration_ms = CASE WHEN duration_ms = 0 THEN 16000 ELSE duration_ms END`); err != nil {
		t.Fatal(err)
	}
	var identID string
	if err := env.pool.QueryRow(ctx, `SELECT id FROM couchtv_clips WHERE kind = 'ident_short'`).Scan(&identID); err != nil {
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
}
