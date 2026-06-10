package catalog

import (
	"log/slog"
	"net/http"

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

func (h *Progress) Put(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	var req struct {
		TitleID         *string `json:"titleId"`
		EpisodeID       *string `json:"episodeId"`
		PositionSeconds int     `json:"positionSeconds"`
		DurationSeconds int     `json:"durationSeconds"`
		WatchedSeconds  int     `json:"watchedSeconds"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if (req.TitleID == nil) == (req.EpisodeID == nil) {
		httpx.BadRequest(w, "exactly one of titleId or episodeId is required")
		return
	}
	if err := h.store.UpsertProgress(r.Context(), user.ID, req.TitleID, req.EpisodeID,
		req.PositionSeconds, req.DurationSeconds); err != nil {
		httpx.Internal(w, err)
		return
	}
	if req.WatchedSeconds > 0 {
		// best effort - analytics must never fail the beacon
		if err := h.analytics.RecordWatch(r.Context(), user.ID, req.TitleID, req.EpisodeID,
			min(req.WatchedSeconds, maxWatchedDelta)); err != nil {
			slog.Warn("record watch time", "err", err)
		}
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Progress) ContinueWatching(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	items, err := h.store.ContinueWatching(r.Context(), user.ID, 20)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Progress) WatchlistGet(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	items, err := h.store.Watchlist(r.Context(), user.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Progress) WatchlistPut(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	titleID := httpx.UUID(r, "titleId")
	if titleID == "" {
		httpx.NotFound(w)
		return
	}
	if err := h.store.WatchlistAdd(r.Context(), user.ID, titleID); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Progress) WatchlistDelete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	titleID := httpx.UUID(r, "titleId")
	if titleID == "" {
		httpx.NotFound(w)
		return
	}
	if err := h.store.WatchlistRemove(r.Context(), user.ID, titleID); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
