package catalog

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/httpx"
)

type AdminHandlers struct {
	store   *Store
	jobs    *jobs.Store
	artwork *artwork.Service
}

func NewAdminHandlers(st *Store, jb *jobs.Store, art *artwork.Service) *AdminHandlers {
	return &AdminHandlers{store: st, jobs: jb, artwork: art}
}

func (h *AdminHandlers) Library(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := LibraryFilter{
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

func (h *AdminHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var in TitleInput
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

func (h *AdminHandlers) Get(w http.ResponseWriter, r *http.Request) {
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
	if len(t.Translations) > 0 {
		out["translations"] = t.Translations // raw per-language jsonb for the editor
	}
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

	art, err := h.artwork.Store.ArtworkFor(r.Context(), "title", t.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out["artwork"] = art
	httpx.JSON(w, http.StatusOK, out)
}

func (h *AdminHandlers) Update(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	var up TitleUpdate
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

// SetTranslation saves a manually-edited name/overview for one language, so
// admins can localize titles that were not fetched from TMDB.
func (h *AdminHandlers) SetTranslation(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	lang := chi.URLParam(r, "lang")
	if id == "" || len(lang) < 2 || len(lang) > 5 {
		httpx.NotFound(w)
		return
	}
	var req struct {
		Name     string `json:"name"`
		Overview string `json:"overview"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if err := h.store.SetTitleTranslationText(r.Context(), id, lang, req.Name, req.Overview); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *AdminHandlers) Delete(w http.ResponseWriter, r *http.Request) {
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

func (h *AdminHandlers) Bulk(w http.ResponseWriter, r *http.Request) {
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
				if _, err = h.jobs.EnqueueJobOnce(r.Context(), "probe",
					map[string]string{"mediaFileId": id}, jobs.EnqueueOpts{}); err != nil {
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

func (h *AdminHandlers) CreateSeason(w http.ResponseWriter, r *http.Request) {
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

func (h *AdminHandlers) DeleteSeason(w http.ResponseWriter, r *http.Request) {
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

func (h *AdminHandlers) CreateEpisode(w http.ResponseWriter, r *http.Request) {
	var in EpisodeInput
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

func (h *AdminHandlers) UpdateEpisode(w http.ResponseWriter, r *http.Request) {
	var up EpisodeUpdate
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

// EpisodeTranslations returns an episode's raw translations for the editor.
func (h *AdminHandlers) EpisodeTranslations(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	tr, err := h.store.EpisodeTranslations(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if len(tr) == 0 {
		tr = json.RawMessage("{}")
	}
	httpx.JSON(w, http.StatusOK, tr)
}

// SetEpisodeTranslation saves a manually-edited name/overview for one language.
func (h *AdminHandlers) SetEpisodeTranslation(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	lang := chi.URLParam(r, "lang")
	if id == "" || len(lang) < 2 || len(lang) > 5 {
		httpx.NotFound(w)
		return
	}
	var req struct {
		Name     string `json:"name"`
		Overview string `json:"overview"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if err := h.store.SetEpisodeTranslation(r.Context(), id, lang, req.Name, req.Overview); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *AdminHandlers) DeleteEpisode(w http.ResponseWriter, r *http.Request) {
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
