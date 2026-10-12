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
	slots   chan struct{} // per-type concurrency limiter
	// yieldTo are job types that keep this one from being claimed while one of
	// them runs, in any process
	yieldTo []string
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

// Yield keeps a registered type from being claimed while a job of one of the
// given types runs. The check comes before each claim, so a job of those types
// that starts later shares the machine with at most the one already claimed.
func (r *Runner) Yield(jobType string, to ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if reg, ok := r.handlers[jobType]; ok {
		reg.yieldTo = to
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
	types, acquired := r.acquireSlots()
	types = r.yielding(ctx, types, acquired)
	if len(types) == 0 {
		return false
	}
	releaseAll := func() {
		for jobType := range acquired {
			r.release(jobType)
		}
	}

	job, err := r.store.ClaimJob(ctx, types)
	if err != nil {
		// ErrNotFound = empty queue; anything else also just backs off a tick
		releaseAll()
		return false
	}

	// keep only the claimed type's slot
	for jobType := range acquired {
		if jobType != job.Type {
			r.release(jobType)
		}
	}
	defer r.release(job.Type)

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

// acquireSlots grabs one slot per registered type that has capacity.
func (r *Runner) acquireSlots() ([]string, map[string]struct{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var types []string
	acquired := map[string]struct{}{}
	for jobType, reg := range r.handlers {
		select {
		case <-reg.slots:
			types = append(types, jobType)
			acquired[jobType] = struct{}{}
		default:
		}
	}
	return types, acquired
}

// yielding drops the types that yield to a running job, giving back their slots.
func (r *Runner) yielding(ctx context.Context, types []string, acquired map[string]struct{}) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.DeleteFunc(types, func(jobType string) bool {
		to := r.handlers[jobType].yieldTo
		if len(to) == 0 {
			return false
		}
		if running, err := r.store.AnyRunning(ctx, to); err == nil && !running {
			return false
		}
		delete(acquired, jobType)
		r.handlers[jobType].slots <- struct{}{}
		return true
	})
}

func (r *Runner) release(jobType string) {
	r.mu.RLock()
	reg, ok := r.handlers[jobType]
	r.mu.RUnlock()
	if ok {
		reg.slots <- struct{}{}
	}
}
