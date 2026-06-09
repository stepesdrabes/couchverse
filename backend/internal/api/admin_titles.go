package api

import (
	"errors"
	"net/http"

	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type AdminTitles struct {
	store *store.Store
}

func NewAdminTitles(st *store.Store) *AdminTitles {
	return &AdminTitles{store: st}
}

func (h *AdminTitles) Library(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.LibraryFilter{
		Kind:     q.Get("type"),
		Status:   q.Get("status"),
		Query:    q.Get("q"),
		Sort:     q.Get("sort"),
		Page:     httpx.QueryInt(r, "page", 1),
		PageSize: httpx.QueryInt(r, "pageSize", 50),
	}
	items, total, err := h.store.ListLibrary(r.Context(), f)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
		"page":  f.Page,
	})
}

func (h *AdminTitles) Create(w http.ResponseWriter, r *http.Request) {
	var in store.TitleInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if in.Name == "" || (in.Kind != "movie" && in.Kind != "series") {
		httpx.BadRequest(w, "name and kind (movie|series) are required")
		return
	}
	t, err := h.store.CreateTitle(r.Context(), in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
}

func (h *AdminTitles) Get(w http.ResponseWriter, r *http.Request) {
	t, err := h.store.TitleByID(r.Context(), httpx.ID(r, "id"))
	if err != nil {
		respondStoreErr(w, err)
		return
	}
	out := map[string]any{"title": t}
	if t.Kind == "series" {
		seasons, err := h.store.SeasonsWithEpisodes(r.Context(), t.ID)
		if err != nil {
			httpx.Internal(w, err)
			return
		}
		out["seasons"] = seasons
	}
	files, err := h.store.MediaFilesForTitle(r.Context(), t.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out["mediaFiles"] = files

	art, err := h.store.ArtworkFor(r.Context(), "title", t.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out["artwork"] = art
	httpx.JSON(w, http.StatusOK, out)
}

func (h *AdminTitles) Update(w http.ResponseWriter, r *http.Request) {
	var up store.TitleUpdate
	if err := httpx.Decode(r, &up); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if up.Status != nil && !validStatus(*up.Status) {
		httpx.BadRequest(w, "invalid status")
		return
	}
	t, err := h.store.UpdateTitle(r.Context(), httpx.ID(r, "id"), up)
	if err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (h *AdminTitles) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteTitle(r.Context(), httpx.ID(r, "id")); err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *AdminTitles) Bulk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
	}
	if err := httpx.Decode(r, &req); err != nil || len(req.IDs) == 0 {
		httpx.BadRequest(w, "ids and action are required")
		return
	}

	var err error
	switch req.Action {
	case "publish":
		err = h.store.SetTitlesStatus(r.Context(), req.IDs, "published")
	case "hide":
		err = h.store.SetTitlesStatus(r.Context(), req.IDs, "hidden")
	case "draft":
		err = h.store.SetTitlesStatus(r.Context(), req.IDs, "draft")
	case "delete":
		err = h.store.DeleteTitles(r.Context(), req.IDs)
	default:
		httpx.BadRequest(w, "unknown action")
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *AdminTitles) CreateSeason(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SeasonNumber int    `json:"seasonNumber"`
		Name         string `json:"name"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	se, err := h.store.CreateSeason(r.Context(), httpx.ID(r, "id"), req.SeasonNumber, req.Name)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, se)
}

func (h *AdminTitles) DeleteSeason(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteSeason(r.Context(), httpx.ID(r, "id")); err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *AdminTitles) CreateEpisode(w http.ResponseWriter, r *http.Request) {
	var in store.EpisodeInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	e, err := h.store.CreateEpisode(r.Context(), httpx.ID(r, "id"), in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

func (h *AdminTitles) UpdateEpisode(w http.ResponseWriter, r *http.Request) {
	var up store.EpisodeUpdate
	if err := httpx.Decode(r, &up); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	e, err := h.store.UpdateEpisode(r.Context(), httpx.ID(r, "id"), up)
	if err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, e)
}

func (h *AdminTitles) DeleteEpisode(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteEpisode(r.Context(), httpx.ID(r, "id")); err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func validStatus(s string) bool {
	switch s {
	case "draft", "processing", "published", "hidden":
		return true
	}
	return false
}

func respondStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, httpx.ErrNotFound) {
		httpx.NotFound(w)
		return
	}
	httpx.Internal(w, err)
}
