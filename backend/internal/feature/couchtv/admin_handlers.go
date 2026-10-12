package couchtv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/httpx"
)

// Admin serves the channel's admin operations.
type Admin struct {
	scheduler *Scheduler
}

// CouchTvOverview is the channel as the admin configures it.
type CouchTvOverview struct {
	Config       CouchTvConfig `json:"config"`
	PlannedUntil *time.Time    `json:"plannedUntil" doc:"When the last planned airing ends; null before the first plan."`
}

// CouchTvAiring is one schedule row: a program or the break before the next one.
type CouchTvAiring struct {
	ID            string    `json:"id" format:"uuid"`
	StartsAt      time.Time `json:"startsAt"`
	EndsAt        time.Time `json:"endsAt"`
	Kind          string    `json:"kind" enum:"program,break"`
	TitleID       string    `json:"titleId,omitempty" format:"uuid"`
	TitleName     string    `json:"titleName,omitempty"`
	TitleKind     string    `json:"titleKind,omitempty" enum:"movie,series"`
	EpisodeID     string    `json:"episodeId,omitempty" format:"uuid"`
	SeasonNumber  int       `json:"seasonNumber,omitempty"`
	EpisodeNumber int       `json:"episodeNumber,omitempty"`
	EpisodeName   string    `json:"episodeName,omitempty"`
	MediaFileID   string    `json:"mediaFileId,omitempty" format:"uuid"`
	Slot          string    `json:"slot,omitempty" doc:"The template slot it airs in; absent for fillers and programs an admin inserted."`
	Fill          string    `json:"fill,omitempty" enum:"strip,marathon,reruns,movie,matinee,premiere,saga,encore,filler" doc:"The rule that chose the program; filler fills a gap before a fixed time."`
	Source        string    `json:"source" enum:"auto,admin"`
	Locked        bool      `json:"locked"`
	State         string    `json:"state" enum:"planned,ready,aired,failed,replaced"`
	Ready         bool      `json:"ready" doc:"The program's media is prepared for the channel (couchtv_prepare found it airable); always true for a break."`
	CreditsAtMs   *int      `json:"creditsAtMs" doc:"Where the credits squeeze begins, in ms from the start; null while unknown."`
	Render        string    `json:"render,omitempty" enum:"uncomposed,pending,composed,rendering,ready,failed" doc:"A break's composition: uncomposed, composed and waiting for clips, rendering (assembling), ready to air or failed (the evergreen break airs instead); absent for a program."`
}

// CouchTvSchedule is a stretch of the schedule.
type CouchTvSchedule struct {
	From     time.Time       `json:"from"`
	To       time.Time       `json:"to"`
	TimeZone string          `json:"timeZone" doc:"The channel's time zone, for showing the times as the template has them."`
	Airings  []CouchTvAiring `json:"airings" doc:"The airings overlapping the stretch, in order."`
}

type overviewOutput struct{ Body CouchTvOverview }

// Get reads the channel's config and how far it is planned.
func (h *Admin) Get(ctx context.Context, _ *struct{}) (*overviewOutput, error) {
	until, err := h.scheduler.Store.PlannedUntil(ctx)
	if err != nil {
		return nil, err
	}
	return &overviewOutput{Body: CouchTvOverview{Config: LoadConfig(ctx, h.scheduler.Settings), PlannedUntil: until}}, nil
}

type updateInput struct{ Body CouchTvConfig }

// Update stores a validated config and queues a plan run, which replans every day that
// is not frozen yet with it.
func (h *Admin) Update(ctx context.Context, in *updateInput) (*overviewOutput, error) {
	cfg := in.Body
	if err := cfg.Validate(); err != nil {
		var invalid *ConfigError
		if errors.As(err, &invalid) {
			return nil, httpx.Fail(http.StatusBadRequest, invalid.Code, invalid.Message)
		}
		return nil, err
	}
	if err := h.checkPinned(ctx, cfg.Template); err != nil {
		return nil, err
	}
	stored := loadStored(ctx, h.scheduler.Settings)
	cfg.GuestLink = stored.GuestLink
	body, err := json.Marshal(storedConfig{CouchTvConfig: cfg, GuestLinkHash: stored.GuestLinkHash})
	if err != nil {
		return nil, err
	}
	if err := h.scheduler.Settings.Set(ctx, settingsKey, body); err != nil {
		return nil, err
	}
	if _, err := h.scheduler.Jobs.EnqueueJob(ctx, PlanJob, PlanPayload{}, jobs.EnqueueOpts{}); err != nil {
		return nil, err
	}
	return h.Get(ctx, nil)
}

// checkPinned rejects pinned titles that do not exist or do not suit their slot.
func (h *Admin) checkPinned(ctx context.Context, t CouchTvTemplate) error {
	var ids []string
	for _, s := range t.Slots {
		ids = append(ids, s.Titles...)
	}
	if len(ids) == 0 {
		return nil
	}
	kinds, err := h.scheduler.Store.TitleKinds(ctx, ids)
	if err != nil {
		return err
	}
	for _, s := range t.Slots {
		want := "movie"
		if s.Fill == FillStrip || s.Fill == FillMarathon || s.Fill == FillReruns {
			want = "series"
		}
		for _, id := range s.Titles {
			if kind, ok := kinds[id]; !ok {
				return httpx.Fail(http.StatusBadRequest, ErrPinnedTitle, fmt.Sprintf("slot %q: title %s does not exist", s.Key, id))
			} else if kind != want {
				return httpx.Fail(http.StatusBadRequest, ErrPinnedTitle, fmt.Sprintf("slot %q airs a %s, title %s is a %s", s.Key, want, id, kind))
			}
		}
	}
	return nil
}

type listAiringsInput struct {
	From time.Time `query:"from" doc:"Start of the stretch (RFC 3339); now when omitted."`
	To   time.Time `query:"to" doc:"End of the stretch; a day after from when omitted, at most 15 days after it."`
}

type scheduleOutput struct{ Body CouchTvSchedule }

// maxRange bounds one schedule read.
const maxRange = 15 * 24 * time.Hour

// ListAirings reads a stretch of the schedule.
func (h *Admin) ListAirings(ctx context.Context, in *listAiringsInput) (*scheduleOutput, error) {
	from, to := in.From, in.To
	if from.IsZero() {
		from = h.scheduler.now()
	}
	if to.IsZero() {
		to = from.Add(24 * time.Hour)
	}
	if !to.After(from) || to.Sub(from) > maxRange {
		return nil, httpx.BadRequestError("to must be after from, by at most 15 days")
	}
	return h.schedule(ctx, from, to)
}

func (h *Admin) schedule(ctx context.Context, from, to time.Time) (*scheduleOutput, error) {
	airings, err := h.scheduler.Store.Schedule(ctx, from, to)
	if err != nil {
		return nil, err
	}
	cfg := LoadConfig(ctx, h.scheduler.Settings)
	return &scheduleOutput{Body: CouchTvSchedule{From: from, To: to, TimeZone: cfg.TimeZone, Airings: airings}}, nil
}

type regenerateInput struct {
	Date string `path:"date" format:"date" doc:"The broadcast day, a date in the channel's time zone; it starts at the day's first anchor."`
}

// RegenerateDay replans a broadcast day (and the days after it, so strips stay in
// order) with the current template and library, keeping admin rows, locked rows and
// whatever already started; it answers with the day's schedule.
func (h *Admin) RegenerateDay(ctx context.Context, in *regenerateInput) (*scheduleOutput, error) {
	date, cfg, err := h.scheduler.RegenerateDay(ctx, in.Date)
	switch {
	case errors.Is(err, errNoSuchDay):
		return nil, httpx.NotFoundError()
	case errors.Is(err, errDayOutOfReach):
		return nil, httpx.BadRequestError(err.Error())
	case err != nil:
		return nil, err
	}
	return h.schedule(ctx, cfg.dayStart(date), cfg.dayStart(nextDate(date)))
}
