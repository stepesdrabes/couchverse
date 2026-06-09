package api

import (
	"net/http"

	"couchverse/internal/auth"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type Progress struct {
	store *store.Store
}

func NewProgress(st *store.Store) *Progress {
	return &Progress{store: st}
}

func (h *Progress) Put(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	var req struct {
		TitleID         *int64 `json:"titleId"`
		EpisodeID       *int64 `json:"episodeId"`
		PositionSeconds int    `json:"positionSeconds"`
		DurationSeconds int    `json:"durationSeconds"`
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
	if err := h.store.WatchlistAdd(r.Context(), user.ID, httpx.ID(r, "titleId")); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Progress) WatchlistDelete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	if err := h.store.WatchlistRemove(r.Context(), user.ID, httpx.ID(r, "titleId")); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
