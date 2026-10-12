package couchtv

import (
	"context"
	"net/http"
	"slices"
	"time"

	"couchverse/internal/httpx"
)

// What adminUpdateAiring does to a program.
const (
	ActionLock   = "lock"
	ActionUnlock = "unlock"
	ActionPin    = "pin"
	ActionSwap   = "swap"
	ActionRemove = "remove"
)

// CouchTvAiringEdit is an admin's change to a program in the schedule. Every change but
// lock and unlock replans the program's broadcast day and the days after it, around it.
type CouchTvAiringEdit struct {
	Action    string     `json:"action" enum:"lock,unlock,pin,swap,remove" doc:"lock keeps the program through replanning and unlock hands it back; pin airs titleId (and episodeId) in its place, from startsAt; swap trades places with swapWith; remove takes it off and the planner airs something else."`
	TitleID   string     `json:"titleId,omitempty" required:"false" format:"uuid" doc:"pin: the title to air; the program's own when omitted."`
	EpisodeID string     `json:"episodeId,omitempty" required:"false" format:"uuid" doc:"pin: the episode, for a series; the program's own when the title stays."`
	StartsAt  *time.Time `json:"startsAt,omitempty" required:"false" doc:"pin: when it starts; the program's own start when omitted."`
	SwapWith  string     `json:"swapWith,omitempty" required:"false" format:"uuid" doc:"swap: the other program; each airs at the other's start."`
}

// CouchTvAiringInsert is a program an admin puts on the schedule at a time of their
// choosing; the planner flows the rest of the day around it.
type CouchTvAiringInsert struct {
	TitleID   string    `json:"titleId" format:"uuid"`
	EpisodeID string    `json:"episodeId,omitempty" required:"false" format:"uuid" doc:"The episode, for a series."`
	StartsAt  time.Time `json:"startsAt" doc:"When it starts, within the planned week."`
}

type updateAiringInput struct {
	ID   string `path:"id" format:"uuid"`
	Body CouchTvAiringEdit
}

type insertAiringInput struct{ Body CouchTvAiringInsert }

type airingOutput struct{ Body CouchTvAiring }

// UpdateAiring locks, pins, swaps or removes a program and answers with the broadcast
// days it touched, as replanned.
func (h *Admin) UpdateAiring(ctx context.Context, in *updateAiringInput) (*scheduleOutput, error) {
	var cfg CouchTvConfig
	var days []time.Time
	err := h.scheduler.Edit(ctx, func(st *Store, c CouchTvConfig, now time.Time) (time.Time, error) {
		cfg = c
		e := editor{st: st, cfg: c, now: now}
		a, err := e.program(ctx, in.ID)
		if err != nil {
			return time.Time{}, err
		}
		days = append(days, c.dayOf(a.StartsAt))
		b := in.Body
		switch b.Action {
		case ActionLock, ActionUnlock:
			return time.Time{}, st.SetLocked(ctx, a.ID, b.Action == ActionLock)
		case ActionRemove:
			if err := e.notOnAir(ctx, a.StartsAt); err != nil {
				return time.Time{}, err
			}
			return days[0], st.RemoveProgram(ctx, a)
		case ActionPin:
			if err := e.notOnAir(ctx, a.StartsAt); err != nil {
				return time.Time{}, err
			}
			start := a.StartsAt
			if b.StartsAt != nil {
				start = b.StartsAt.Truncate(time.Second)
			}
			title := b.TitleID
			episode := b.EpisodeID
			if title == "" {
				title = a.TitleID
			}
			if title == a.TitleID && episode == "" {
				episode = a.EpisodeID
			}
			pinned, err := e.airing(ctx, a, title, episode, start)
			if err != nil {
				return time.Time{}, err
			}
			if err := e.place(ctx, pinned, a.ID); err != nil {
				return time.Time{}, err
			}
			days = append(days, c.dayOf(start))
			return slices.MinFunc(days, time.Time.Compare), st.UpdateProgram(ctx, a, pinned)
		case ActionSwap:
			return e.swap(ctx, a, b.SwapWith, &days)
		}
		return time.Time{}, httpx.BadRequestError("unknown action")
	})
	if err != nil {
		return nil, err
	}
	first, last := slices.MinFunc(days, time.Time.Compare), slices.MaxFunc(days, time.Time.Compare)
	return h.schedule(ctx, cfg.dayStart(first), cfg.dayStart(nextDate(last)))
}

// InsertAiring puts a movie or an episode on the schedule at a time and replans its
// broadcast day (and the days after it) around it.
func (h *Admin) InsertAiring(ctx context.Context, in *insertAiringInput) (*airingOutput, error) {
	var id string
	var at time.Time
	err := h.scheduler.Edit(ctx, func(st *Store, c CouchTvConfig, now time.Time) (time.Time, error) {
		e := editor{st: st, cfg: c, now: now}
		row := Airing{Kind: KindProgram, Source: SourceAdmin, State: StatePlanned}
		a, err := e.airing(ctx, row, in.Body.TitleID, in.Body.EpisodeID, in.Body.StartsAt.Truncate(time.Second))
		if err != nil {
			return time.Time{}, err
		}
		if err := e.place(ctx, a); err != nil {
			return time.Time{}, err
		}
		at = a.StartsAt
		id, err = st.InsertProgram(ctx, a)
		return c.dayOf(a.StartsAt), err
	})
	if err != nil {
		return nil, err
	}
	rows, err := h.scheduler.Store.Schedule(ctx, at, at.Add(time.Second))
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.ID == id {
			return &airingOutput{Body: r}, nil
		}
	}
	return nil, httpx.ErrNotFound
}

// editor checks and makes an admin's change inside the planning transaction.
type editor struct {
	st  *Store
	cfg CouchTvConfig
	now time.Time
}

// program reads a program row to change.
func (e editor) program(ctx context.Context, id string) (Airing, error) {
	a, err := e.st.Airing(ctx, id)
	if err != nil {
		return Airing{}, err
	}
	if a.Kind != KindProgram {
		return Airing{}, httpx.BadRequestError("only a program can be changed; its breaks follow it")
	}
	return *a, nil
}

// notOnAir refuses a change at t once what is on air runs up to it.
func (e editor) notOnAir(ctx context.Context, t time.Time) error {
	reaches, err := e.st.OnAirReaches(ctx, t, e.now)
	if err != nil {
		return err
	}
	if reaches {
		return httpx.Fail(http.StatusConflict, "on_air", "the program is on air or comes right after what is")
	}
	return nil
}

// airing is row airing a title (and episode) from start, as an admin's program.
func (e editor) airing(ctx context.Context, row Airing, titleID, episodeID string, start time.Time) (Airing, error) {
	p, err := e.st.ProgramMedia(ctx, titleID, episodeID)
	if err != nil {
		return Airing{}, err
	}
	switch {
	case !p.EpisodeOK && p.Kind == "series":
		return Airing{}, httpx.BadRequestError("a series airs one of its episodes: episodeId is required and must be the series'")
	case !p.EpisodeOK:
		return Airing{}, httpx.BadRequestError("a movie takes no episodeId")
	case p.Excluded:
		return Airing{}, httpx.Fail(http.StatusConflict, "excluded", "the title is not on Couch TV")
	case !p.Published:
		return Airing{}, httpx.Fail(http.StatusConflict, "not_published", "the title is not published")
	case p.Media.FileID == "" || p.Media.onAir() <= 0:
		return Airing{}, httpx.Fail(http.StatusConflict, "no_media", "nothing of it can play yet")
	}
	a := row
	a.Fill = adminFill(row.Fill, row.TitleID, titleID, p.Kind == "movie")
	a.TitleID, a.EpisodeID, a.MediaFileID = titleID, episodeID, p.Media.FileID
	a.StartsAt, a.EndsAt, a.CreditsAt = start, start.Add(p.Media.onAir()), p.Media.CreditsAt
	a.Source = SourceAdmin
	return a, nil
}

// place checks that a program can air where it is: in the planned week, after what is
// on air, and a shortest break clear of every program replanning keeps (those with
// the except ids are being changed with it).
func (e editor) place(ctx context.Context, a Airing, except ...string) error {
	if !a.StartsAt.After(e.now) || e.cfg.dayOf(a.StartsAt).After(lastDay(e.cfg, e.now)) {
		return httpx.Fail(http.StatusBadRequest, "outside_horizon", "a program can only be placed between now and the end of the planned week")
	}
	if err := e.notOnAir(ctx, a.StartsAt); err != nil {
		return err
	}
	gap := time.Duration(e.cfg.BreakMinSeconds) * time.Second
	clash, err := e.st.FixedBetween(ctx, a.StartsAt.Add(-gap), a.EndsAt.Add(gap), e.now, except)
	if err != nil {
		return err
	}
	if len(clash) > 0 {
		return overlap()
	}
	return nil
}

func overlap() error {
	return httpx.Fail(http.StatusConflict, "overlap", "it would run into a program that stays (an admin's, a locked one or one on air), or leave less than the shortest break")
}

// swap airs a's program at b's start and b's at a's.
func (e editor) swap(ctx context.Context, a Airing, otherID string, days *[]time.Time) (time.Time, error) {
	if otherID == "" || otherID == a.ID {
		return time.Time{}, httpx.BadRequestError("swapWith names the other program")
	}
	b, err := e.program(ctx, otherID)
	if err != nil {
		return time.Time{}, err
	}
	*days = append(*days, e.cfg.dayOf(b.StartsAt))
	for _, x := range []Airing{a, b} {
		if err := e.notOnAir(ctx, x.StartsAt); err != nil {
			return time.Time{}, err
		}
	}
	a2, err := e.airing(ctx, a, b.TitleID, b.EpisodeID, a.StartsAt)
	if err != nil {
		return time.Time{}, err
	}
	b2, err := e.airing(ctx, b, a.TitleID, a.EpisodeID, b.StartsAt)
	if err != nil {
		return time.Time{}, err
	}
	gap := time.Duration(e.cfg.BreakMinSeconds) * time.Second
	if a2.StartsAt.Before(b2.EndsAt.Add(gap)) && b2.StartsAt.Before(a2.EndsAt.Add(gap)) {
		return time.Time{}, overlap()
	}
	for _, x := range []Airing{a2, b2} {
		if err := e.place(ctx, x, a.ID, b.ID); err != nil {
			return time.Time{}, err
		}
	}
	if err := e.st.UpdateProgram(ctx, a, a2); err != nil {
		return time.Time{}, err
	}
	return slices.MinFunc(*days, time.Time.Compare), e.st.UpdateProgram(ctx, b, b2)
}

// adminFill is the fill an admin's program takes in a row: the row's own while the
// title stays (another episode of a strip restarts the strip there), else the plain
// rerun of its kind, or the rotation for a movie in prime time. Another title under a
// strip's, premiere's or saga's fill would move that strip, call a rerun a premiere or
// skip ahead in a franchise.
func adminFill(rowFill, rowTitle, titleID string, movie bool) string {
	if titleID == rowTitle && rowFill != "" {
		return rowFill
	}
	if rowFill == FillFiller {
		return FillFiller
	}
	if !movie {
		return FillReruns
	}
	if rowFill == FillMovie || rowFill == FillPremiere || rowFill == FillSaga {
		return FillMovie
	}
	return FillMatinee
}
