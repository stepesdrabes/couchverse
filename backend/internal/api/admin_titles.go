package api

import (
	"net/http"

	"couchverse/internal/artwork"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type AdminTitles struct {
	store   *store.Store
	artwork *artwork.Service
}

func NewAdminTitles(st *store.Store, art *artwork.Service) *AdminTitles {
	return &AdminTitles{store: st, artwork: art}
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
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	t, err := h.store.TitleByID(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
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

	fileIDs := make([]string, len(files))
	for i, f := range files {
		fileIDs[i] = f.ID
	}
	subsByFile, err := h.store.SubtitlesForMediaFiles(r.Context(), fileIDs)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out["subtitlesByFile"] = subsByFile

	art, err := h.store.ArtworkFor(r.Context(), "title", t.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out["artwork"] = art
	httpx.JSON(w, http.StatusOK, out)
}

func (h *AdminTitles) Update(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	var up store.TitleUpdate
	if err := httpx.Decode(r, &up); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if up.Status != nil && !validStatus(*up.Status) {
		httpx.BadRequest(w, "invalid status")
		return
	}
	t, err := h.store.UpdateTitle(r.Context(), id, up)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (h *AdminTitles) Delete(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	if err := h.store.DeleteTitle(r.Context(), id); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if err := h.artwork.DeleteForOwner(r.Context(), "title", id); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *AdminTitles) Bulk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"`
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
		if err == nil {
			for _, id := range req.IDs {
				if err = h.artwork.DeleteForOwner(r.Context(), "title", id); err != nil {
					break
				}
			}
		}
	case "rescan":
		var fileIDs []string
		fileIDs, err = h.store.MediaFileIDsForTitles(r.Context(), req.IDs)
		if err == nil {
			for _, id := range fileIDs {
				if _, err = h.store.EnqueueJobOnce(r.Context(), "probe",
					map[string]string{"mediaFileId": id}, store.EnqueueOpts{}); err != nil {
					break
				}
			}
		}
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
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	se, err := h.store.CreateSeason(r.Context(), id, req.SeasonNumber, req.Name)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, se)
}

func (h *AdminTitles) DeleteSeason(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	if err := h.store.DeleteSeason(r.Context(), id); err != nil {
		httpx.StoreErr(w, err)
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
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	e, err := h.store.CreateEpisode(r.Context(), id, in)
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
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	e, err := h.store.UpdateEpisode(r.Context(), id, up)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, e)
}

func (h *AdminTitles) DeleteEpisode(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	if err := h.store.DeleteEpisode(r.Context(), id); err != nil {
		httpx.StoreErr(w, err)
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
