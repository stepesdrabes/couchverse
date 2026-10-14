package server_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// TestCouchTvLongRendersWait: the render sidecar has one worker and does not leave a job it has
// begun, so a long render (a full ident, a trailer's body) claimed when a bulletin's render may be
// asked for before it is over goes back in the queue for the next quiet hours, without a failure
// or an attempt counted and with its clip still pending; one claimed in time, and every render
// that takes a few minutes, goes on, as do all of them with the bulletins off.
func TestCouchTvLongRendersWait(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	prague, _ := time.LoadLocation("Europe/Prague")
	clock := time.Date(2026, 10, 14, 10, 0, 0, 0, prague)
	jb, set, store := jobs.NewStore(env.pool), settings.NewStore(env.pool), couchtv.NewStore(env.pool)
	r := &couchtv.Renderer{Store: store, Jobs: jb, Settings: set, Now: func() time.Time { return clock }}

	type state struct {
		jobStatus, clipState string
		runAt                time.Time
		attempts             int
	}
	// render queues a clip of the kind with its render, claims it as the sidecar does and renders
	n := 0
	render := func(kind string) (state, error) {
		t.Helper()
		n++
		key := kind + "-" + strconv.Itoa(n)
		id, _, err := store.RequestClip(ctx, kind, key, couchtv.Spec{}, 100, clock)
		if err != nil {
			t.Fatal(err)
		}
		// a spec that is no object cannot be rendered: a render that goes on fails its clip at once
		if _, err := env.pool.Exec(ctx, `UPDATE couchtv_clips SET spec = '[]' WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := jb.EnqueueJob(ctx, couchtv.RenderJob, couchtv.RenderPayload{ClipID: id}, jobs.EnqueueOpts{}); err != nil {
			t.Fatal(err)
		}
		job, err := jb.ClaimJob(ctx, []string{couchtv.RenderJob})
		if err != nil {
			t.Fatal(err)
		}
		renderErr := r.Render(ctx, job, func(int) {})
		var s state
		if err := env.pool.QueryRow(ctx, `SELECT j.status, j.run_at, j.attempts, c.state FROM jobs j, couchtv_clips c
			WHERE j.id = $1 AND c.id = $2`, job.ID, id).Scan(&s.jobStatus, &s.runAt, &s.attempts, &s.clipState); err != nil {
			t.Fatal(err)
		}
		return s, renderErr
	}
	goesOn := func(name, kind string) {
		t.Helper()
		if s, err := render(kind); err == nil || s.clipState != "failed" || s.jobStatus != "running" {
			t.Errorf("%s: %+v, %v: the render did not go on", name, s, err)
		}
	}
	waits := func(name, kind string, until time.Time) {
		t.Helper()
		s, err := render(kind)
		if err != nil || s.jobStatus != "pending" || s.clipState != "pending" || s.attempts != 0 || !s.runAt.Equal(until) {
			t.Errorf("%s: %+v, %v: want it queued again at %s, no attempt counted", name, s, err, until.In(prague))
		}
	}

	nextNight := time.Date(2026, 10, 15, 1, 0, 0, 0, prague)
	waits("an ident by day", couchtv.ClipIdentFull, nextNight)
	waits("a trailer by day", couchtv.ClipTrailerBody, nextNight)
	goesOn("a promo body by day", couchtv.ClipPromoBody)

	clock = time.Date(2026, 10, 15, 2, 0, 0, 0, prague)
	goesOn("an ident at two", couchtv.ClipIdentFull)
	goesOn("a trailer at two", couchtv.ClipTrailerBody)

	clock = time.Date(2026, 10, 15, 4, 0, 0, 0, prague)
	waits("an ident at four, which would run into the news of half past five", couchtv.ClipIdentFull, time.Date(2026, 10, 16, 1, 0, 0, 0, prague))
	goesOn("a trailer at four", couchtv.ClipTrailerBody)

	// with the bulletins off, nothing waits for them
	cfg := couchtv.DefaultConfig()
	cfg.Bulletins.Enabled = false
	if status, body, _ := env.request(t, env.clients["admin"], "PUT", "/admin/couchtv", cfg); status != http.StatusOK {
		t.Fatalf("saving the config: %d %s", status, body)
	}
	goesOn("an ident at four with the bulletins off", couchtv.ClipIdentFull)
}
