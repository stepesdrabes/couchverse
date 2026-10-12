package server_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"couchverse/internal/feature/jobs"
)

// TestRunnerSharedSlots runs two job types that share one slot on two workers: they never
// run at once, and the claim takes the higher priority whichever type it is.
func TestRunnerSharedSlots(t *testing.T) {
	env := newTestEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	st := jobs.NewStore(env.pool)

	var mu sync.Mutex
	running, most := 0, 0
	var order []string
	work := func(name string) jobs.Handler {
		return func(context.Context, *jobs.Job, func(int)) error {
			mu.Lock()
			running++
			most = max(most, running)
			order = append(order, name)
			mu.Unlock()
			// longer than a worker's 2 s poll, so the other worker looks while it runs
			time.Sleep(2500 * time.Millisecond)
			mu.Lock()
			running--
			mu.Unlock()
			return nil
		}
	}
	r := jobs.NewRunner(st, 2)
	r.Register("test_encode", 1, work("encode"))
	r.Register("test_analyze", 1, work("analyze"))
	r.Share(1, "test_encode", "test_analyze")
	for _, j := range []struct {
		kind     string
		priority int
	}{{"test_analyze", -10}, {"test_analyze", -10}, {"test_encode", 10}} {
		if _, err := st.EnqueueJob(ctx, j.kind, struct{}{}, jobs.EnqueueOpts{Priority: j.priority}); err != nil {
			t.Fatal(err)
		}
	}
	go r.Run(ctx)

	deadline := time.Now().Add(30 * time.Second)
	for {
		var done int
		if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE type LIKE 'test_%' AND status = 'done'`).Scan(&done); err != nil {
			t.Fatal(err)
		}
		if done == 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of 3 jobs done", done)
		}
		time.Sleep(100 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if most != 1 {
		t.Errorf("%d jobs of the shared pool ran at once", most)
	}
	if order[0] != "encode" {
		t.Errorf("claimed in the order %v, want the encode first", order)
	}
}

// TestRunnerYield holds a type back while a job it yields to runs, whatever that job's
// priority, and lets it go once that job was claimed over an hour ago (its process is
// gone).
func TestRunnerYield(t *testing.T) {
	env := newTestEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	st := jobs.NewStore(env.pool)
	r := jobs.NewRunner(st, 1)
	r.Register("test_analyze", 1, func(context.Context, *jobs.Job, func(int)) error { return nil })
	r.Yield("test_analyze", true, "test_render")

	// a render in another process: running, at a negative priority, claimed just now
	var render int64
	if err := env.pool.QueryRow(ctx, `INSERT INTO jobs (type, status, priority, claimed_at)
		VALUES ('test_render', 'running', -1000, now()) RETURNING id`).Scan(&render); err != nil {
		t.Fatal(err)
	}
	analysis, err := st.EnqueueJob(ctx, "test_analyze", struct{}{}, jobs.EnqueueOpts{})
	if err != nil {
		t.Fatal(err)
	}
	status := func() string {
		t.Helper()
		var s string
		if err := env.pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1`, analysis).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	go r.Run(ctx)
	time.Sleep(3 * time.Second)
	if s := status(); s != "pending" {
		t.Fatalf("the analysis ran next to a render: %s", s)
	}

	if _, err := env.pool.Exec(ctx, `UPDATE jobs SET claimed_at = now() - interval '2 hours' WHERE id = $1`, render); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); status() != "done"; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the analysis is still %s behind a render claimed two hours ago", status())
		}
	}
}
