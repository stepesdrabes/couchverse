package couchtv

import (
	"context"
	"log/slog"
	"math"
	"time"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// PlanJob is the job that keeps the schedule planned.
const PlanJob = "couchtv_plan"

// The horizon (plan 5.3): the job keeps planDays of schedule ahead and leaves a
// broadcast day that starts within freeze alone once it is planned, since its breaks
// are being rendered. It runs daily at dailyHour in the channel's zone.
const (
	planDays  = 7
	freeze    = 24 * time.Hour
	dailyHour = 4
)

// PlanPayload names the daily run by its date, so queueing the next one is idempotent
// while the current one runs; a run asked for at startup or by a config change has
// none.
type PlanPayload struct {
	Day string `json:"day,omitempty"`
}

// Scheduler plans the channel: the couchtv_plan job and an admin's replans.
type Scheduler struct {
	Store    *Store
	Settings *settings.Store
	Jobs     *jobs.Store
	// Now is the clock; nil means time.Now (tests set it).
	Now func() time.Time
}

func (s *Scheduler) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Start queues a plan run now: the schedule catches up after downtime and the daily
// runs begin. The schedule is kept while the couchTvEnabled flag is off, so an admin
// can review it before the channel goes on air.
func (s *Scheduler) Start(ctx context.Context) error {
	_, err := s.Jobs.EnqueueJobOnce(ctx, PlanJob, PlanPayload{}, jobs.EnqueueOpts{})
	return err
}

// Handle is the couchtv_plan job: it extends the schedule to planDays ahead, replans
// every day not frozen yet and swaps unprepared programs about to air.
func (s *Scheduler) Handle(ctx context.Context, _ *jobs.Job, report func(int)) error {
	cfg := LoadConfig(ctx, s.Settings)
	now := s.now()
	// the next daily run is queued first, so a failing run cannot end the chain
	if err := s.queueNext(ctx, cfg, now); err != nil {
		return err
	}
	err := s.Store.Locked(ctx, func(st *Store) error {
		lib, err := st.Library(ctx)
		if err != nil {
			return err
		}
		from, err := firstOpenDay(ctx, st, cfg, now)
		if err != nil {
			return err
		}
		if err := replan(ctx, st, cfg, lib, from, now); err != nil {
			return err
		}
		report(80)
		return swap(ctx, st, cfg, lib, now)
	})
	if err != nil {
		return err
	}
	// what was just planned gets prepared, and its breaks composed now rather than at
	// the next hourly run
	if _, err := s.Jobs.EnqueueJobOnce(ctx, PrepareJob, PreparePayload{}, jobs.EnqueueOpts{}); err != nil {
		return err
	}
	_, err = s.Jobs.EnqueueJobOnce(ctx, ComposeJob, ComposePayload{}, jobs.EnqueueOpts{})
	return err
}

func (s *Scheduler) queueNext(ctx context.Context, cfg CouchTvConfig, now time.Time) error {
	l := now.In(cfg.location())
	next := time.Date(l.Year(), l.Month(), l.Day(), dailyHour, 0, 0, 0, l.Location())
	if !next.After(now) {
		next = time.Date(l.Year(), l.Month(), l.Day()+1, dailyHour, 0, 0, 0, l.Location())
	}
	_, err := s.Jobs.EnqueueJobOnce(ctx, PlanJob, PlanPayload{Day: next.Format(time.DateOnly)}, jobs.EnqueueOpts{RunAt: next})
	return err
}

// Replan replans from a broadcast day (its date) to the end of the horizon, keeping
// admin rows, locked rows and whatever already started. The days after it are replanned
// too, so strips stay in order.
func (s *Scheduler) Replan(ctx context.Context, date time.Time) error {
	cfg := LoadConfig(ctx, s.Settings)
	now := s.now()
	return s.Store.Locked(ctx, func(st *Store) error {
		lib, err := st.Library(ctx)
		if err != nil {
			return err
		}
		return replan(ctx, st, cfg, lib, date, now)
	})
}

// firstOpenDay is the first broadcast day the job replans: from today on, a day that
// starts within the freeze keeps its plan once it has one.
func firstOpenDay(ctx context.Context, st *Store, cfg CouchTvConfig, now time.Time) (time.Time, error) {
	date := cfg.dayOf(now)
	for cfg.dayStart(date).Before(now.Add(freeze)) {
		planned, err := st.DayPlanned(ctx, cfg.dayStart(date), cfg.dayStart(nextDate(date)))
		if err != nil || !planned {
			return date, err
		}
		date = nextDate(date)
	}
	return date, nil
}

// lastDay is the last broadcast day the horizon reaches.
func lastDay(cfg CouchTvConfig, now time.Time) time.Time {
	return cfg.dayOf(now.Add(planDays * 24 * time.Hour))
}

func replan(ctx context.Context, st *Store, cfg CouchTvConfig, lib Library, from, now time.Time) error {
	from = civilIn(from, cfg.location())
	// whole days between two dates; rounding absorbs a clock change between them
	days := int(math.Round(lastDay(cfg, now).Sub(from).Hours()/24)) + 1
	if days <= 0 {
		return nil
	}
	start := cfg.dayStart(from)
	history, err := st.History(ctx, start)
	if err != nil {
		return err
	}
	fixed, err := st.Fixed(ctx, start, now)
	if err != nil {
		return err
	}
	cursors, err := st.Cursors(ctx)
	if err != nil {
		return err
	}
	rows := Plan(PlanInput{Config: cfg, Library: lib, History: history, Fixed: fixed, Cursors: cursors, From: from, Days: days, Now: now})
	added, dropped, err := st.Replace(ctx, start, rows)
	if err != nil {
		return err
	}
	slog.Info("couchtv: planned", "from", from.Format(time.DateOnly), "days", days, "added", added, "dropped", dropped)
	return nil
}

func swap(ctx context.Context, st *Store, cfg CouchTvConfig, lib Library, now time.Time) error {
	rows, err := st.Programs(ctx, now, now.Add(freeze))
	if err != nil || len(rows) == 0 {
		return err
	}
	history, err := st.History(ctx, now)
	if err != nil {
		return err
	}
	ends := map[string]time.Time{}
	for _, a := range rows {
		ends[a.ID] = a.EndsAt
	}
	for _, a := range SwapUnready(SwapInput{Config: cfg, Library: lib, History: history, Rows: rows, Now: now}) {
		if err := st.Swap(ctx, a, ends[a.ID]); err != nil {
			return err
		}
		slog.Info("couchtv: swapped an unprepared program", "airingId", a.ID, "startsAt", a.StartsAt, "titleId", a.TitleID)
	}
	return nil
}
