package catalog

import (
	"context"
	"log/slog"
	"time"

	"couchverse/internal/feature/analytics"
	"couchverse/internal/feature/auth"
	"couchverse/internal/httpx"
)

// maxWatchedDelta bounds one beacon's watch-time contribution (missed-beacon
// tolerance / abuse bound); the player reports roughly every 10 seconds.
const maxWatchedDelta = 600

type Progress struct {
	store     *Store
	analytics *analytics.Store
}

func NewProgress(st *Store, an *analytics.Store) *Progress {
	return &Progress{store: st, analytics: an}
}

// ProgressReport is the player's periodic beacon for one title or episode.
type ProgressReport struct {
	TitleID         *string `json:"titleId,omitempty" format:"uuid" doc:"Set for a movie; exactly one of titleId and episodeId."`
	EpisodeID       *string `json:"episodeId,omitempty" format:"uuid" doc:"Set for an episode; exactly one of titleId and episodeId."`
	PositionSeconds int     `json:"positionSeconds" minimum:"0"`
	DurationSeconds int     `json:"durationSeconds" minimum:"0"`
	WatchedSeconds  int     `json:"watchedSeconds,omitempty" minimum:"0" doc:"Seconds actually played since the previous report (feeds analytics)."`
	// clients replay what was watched offline once they are back
	WatchedAt *time.Time `json:"watchedAt,omitempty" doc:"When the position was reached, for a report replayed after watching offline; it does not replace a position saved later. Absent means now."`
}

type progressInput struct {
	Body ProgressReport
}

func (h *Progress) Put(ctx context.Context, in *progressInput) (*struct{}, error) {
	user := auth.UserFrom(ctx)
	req := in.Body
	if (req.TitleID == nil) == (req.EpisodeID == nil) {
		return nil, httpx.BadRequestError("exactly one of titleId or episodeId is required")
	}
	if err := h.store.UpsertProgress(ctx, user.ID, req.TitleID, req.EpisodeID,
		req.PositionSeconds, req.DurationSeconds, req.WatchedAt); err != nil {
		return nil, err
	}
	if req.WatchedSeconds > 0 {
		// best effort - analytics must never fail the beacon
		if err := h.analytics.RecordWatch(ctx, user.ID, req.TitleID, req.EpisodeID,
			min(req.WatchedSeconds, maxWatchedDelta)); err != nil {
			slog.Warn("record watch time", "err", err)
		}
	}
	return nil, nil
}

type continueOutput struct{ Body []ContinueItem }

func (h *Progress) ContinueWatching(ctx context.Context, _ *struct{}) (*continueOutput, error) {
	items, err := h.store.ContinueWatching(ctx, auth.UserFrom(ctx).ID, 20)
	if err != nil {
		return nil, err
	}
	return &continueOutput{Body: items}, nil
}

type cardsOutput struct{ Body []CardItem }

func (h *Progress) WatchlistGet(ctx context.Context, _ *struct{}) (*cardsOutput, error) {
	items, err := h.store.Watchlist(ctx, auth.UserFrom(ctx).ID)
	if err != nil {
		return nil, err
	}
	return &cardsOutput{Body: items}, nil
}

type watchlistInput struct {
	TitleID string `path:"titleId" format:"uuid"`
}

func (h *Progress) WatchlistPut(ctx context.Context, in *watchlistInput) (*struct{}, error) {
	return nil, h.store.WatchlistAdd(ctx, auth.UserFrom(ctx).ID, in.TitleID)
}

func (h *Progress) WatchlistDelete(ctx context.Context, in *watchlistInput) (*struct{}, error) {
	return nil, h.store.WatchlistRemove(ctx, auth.UserFrom(ctx).ID, in.TitleID)
}
