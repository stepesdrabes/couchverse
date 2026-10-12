// Package jobs runs background work (scans, probes, transcodes) from the
// Postgres-backed queue inside the main process.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"runtime/debug"
	"slices"
	"sync"
	"time"
)

// Handler executes one job. report publishes coarse progress (0-100).
type Handler func(ctx context.Context, job *Job, report func(pct int)) error

type registration struct {
	handler Handler
	// slots limit how many run at once: the type's own, or a pool it shares (Share)
	slots chan struct{}
	// yieldTo are job types that keep this one from being claimed while one of
	// them runs, in any process; backfill says one of a negative priority counts too
	yieldTo  []string
	backfill bool
}

type Runner struct {
	store    *Store
	workers  int
	mu       sync.RWMutex
	handlers map[string]registration
}

func NewRunner(st *Store, workers int) *Runner {
	if workers < 1 {
		workers = 1
	}
	return &Runner{store: st, workers: workers, handlers: map[string]registration{}}
}

// Register adds a handler for a job type with a per-type concurrency cap -
// e.g. transcodes are limited to 1 so probes/scans keep flowing.
func (r *Runner) Register(jobType string, maxConcurrent int, h Handler) {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	slots := make(chan struct{}, maxConcurrent)
	for range maxConcurrent {
		slots <- struct{}{}
	}
	r.mu.Lock()
	r.handlers[jobType] = registration{handler: h, slots: slots}
	r.mu.Unlock()
}

// Share makes the given registered types draw from one pool of n slots, so at
// most n jobs of them run at once in this process, whichever type they are.
func (r *Runner) Share(n int, types ...string) {
	pool := make(chan struct{}, max(n, 1))
	for range cap(pool) {
		pool <- struct{}{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range types {
		if reg, ok := r.handlers[t]; ok {
			reg.slots = pool
			r.handlers[t] = reg
		}
	}
}

// Yield keeps a registered type from being claimed while a job of one of the
// given types runs. Unless backfill, a running job of a negative priority does not
// count, since a backlog of it would hold the type up for as long as it lasts. A job
// claimed over an hour ago counts no more either: its process may be gone. The check
// comes before each claim, so a job of those types that starts later shares the
// machine with at most the one already claimed.
func (r *Runner) Yield(jobType string, backfill bool, to ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if reg, ok := r.handlers[jobType]; ok {
		reg.yieldTo, reg.backfill = to, backfill
		r.handlers[jobType] = reg
	}
}

func (r *Runner) Run(ctx context.Context) {
	r.mu.RLock()
	types := slices.Collect(maps.Keys(r.handlers))
	r.mu.RUnlock()
	if n, err := r.store.ResetRunningJobs(ctx, types); err != nil {
		slog.Error("reset running jobs", "err", err)
	} else if n > 0 {
		slog.Info("requeued orphaned jobs", "count", n)
	}

	var wg sync.WaitGroup
	for i := range r.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.workerLoop(ctx, i)
		}()
	}
	wg.Wait()
}

func (r *Runner) workerLoop(ctx context.Context, id int) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		if !r.runOne(ctx) {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		} else if ctx.Err() != nil {
			return
		}
		_ = id
	}
}

// runOne claims and executes a single job; returns false when the queue is idle.
func (r *Runner) runOne(ctx context.Context) bool {
	types, held := r.acquireSlots()
	types = r.yielding(ctx, types)
	// a slot none of the types left can use goes back
	for slots := range held {
		if !slices.ContainsFunc(types, func(t string) bool { return r.slotsOf(t) == slots }) {
			slots <- struct{}{}
			delete(held, slots)
		}
	}
	if len(types) == 0 {
		return false
	}

	job, err := r.store.ClaimJob(ctx, types)
	if err != nil {
		// ErrNotFound = empty queue; anything else also just backs off a tick
		for slots := range held {
			slots <- struct{}{}
		}
		return false
	}

	// keep only the claimed type's slot
	claimed := r.slotsOf(job.Type)
	for slots := range held {
		if slots != claimed {
			slots <- struct{}{}
		}
	}
	defer func() { claimed <- struct{}{} }()

	r.execute(ctx, job)
	return true
}

func (r *Runner) execute(ctx context.Context, job *Job) {
	slog.Info("job start", "id", job.ID, "type", job.Type, "attempt", job.Attempts)

	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// watch for admin cancellation while the handler runs
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				status, err := r.store.JobStatus(context.WithoutCancel(jobCtx), job.ID)
				if err == nil && status == "cancelled" {
					cancel()
					return
				}
			}
		}
	}()

	r.mu.RLock()
	reg := r.handlers[job.Type]
	r.mu.RUnlock()

	err := func() (err error) {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("panic: %v\n%s", p, debug.Stack())
			}
		}()
		report := func(pct int) {
			_ = r.store.SetJobProgress(context.WithoutCancel(jobCtx), job.ID, pct)
		}
		return reg.handler(jobCtx, job, report)
	}()

	cancel()
	<-watcherDone

	// use a fresh context: the worker ctx may be shutting down
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer finishCancel()

	switch {
	case err == nil:
		if e := r.store.CompleteJob(finishCtx, job.ID); e != nil {
			slog.Error("complete job", "id", job.ID, "err", e)
		}
		slog.Info("job done", "id", job.ID, "type", job.Type)
	case errors.Is(err, context.Canceled):
		// cancelled by admin (status already set) or shutdown (reset at next start)
		slog.Info("job cancelled/interrupted", "id", job.ID, "type", job.Type)
	default:
		slog.Warn("job failed", "id", job.ID, "type", job.Type, "err", err)
		if e := r.store.FailJob(finishCtx, job, err); e != nil {
			slog.Error("fail job", "id", job.ID, "err", e)
		}
	}
}

// acquireSlots offers every registered type with a free slot, holding one slot
// per pool: the types sharing a pool are offered together, so the claim picks
// among them by priority.
func (r *Runner) acquireSlots() ([]string, map[chan struct{}]struct{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var types []string
	held := map[chan struct{}]struct{}{}
	for jobType, reg := range r.handlers {
		if _, ok := held[reg.slots]; !ok {
			select {
			case <-reg.slots:
				held[reg.slots] = struct{}{}
			default:
				continue
			}
		}
		types = append(types, jobType)
	}
	return types, held
}

// yielding drops the types that yield to a running job.
func (r *Runner) yielding(ctx context.Context, types []string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.DeleteFunc(types, func(jobType string) bool {
		reg := r.handlers[jobType]
		if len(reg.yieldTo) == 0 {
			return false
		}
		running, err := r.store.AnyRunning(ctx, reg.yieldTo, reg.backfill)
		return err != nil || running
	})
}

func (r *Runner) slotsOf(jobType string) chan struct{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handlers[jobType].slots
}
