package api

import (
	"net/http"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
	"couchverse/internal/store"
	"couchverse/internal/tmdb"
)

type AdminMetadata struct {
	store    *store.Store
	settings *settings.Store
	jobs     *jobs.Store
}

func NewAdminMetadata(st *store.Store, set *settings.Store, jb *jobs.Store) *AdminMetadata {
	return &AdminMetadata{store: st, settings: set, jobs: jb}
}

// Search proxies a TMDB search so the API key never reaches the browser.
func (h *AdminMetadata) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("kind")
	if q == "" || (kind != "movie" && kind != "series") {
		httpx.BadRequest(w, "q and kind (movie|series) are required")
		return
	}

	key, err := tmdb.APIKey(r.Context(), h.settings)
	if err != nil {
		httpx.Error(w, http.StatusPreconditionFailed, "no_tmdb_key", err.Error())
		return
	}

	results, err := tmdb.New(key).Search(r.Context(), kind, q)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "tmdb_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, results)
}

// Seasons previews a linked show's TMDB seasons for the import picker.
func (h *AdminMetadata) Seasons(w http.ResponseWriter, r *http.Request) {
	title, ok := h.requireTmdbSeries(w, r)
	if !ok {
		return
	}
	key, err := tmdb.APIKey(r.Context(), h.settings)
	if err != nil {
		httpx.Error(w, http.StatusPreconditionFailed, "no_tmdb_key", err.Error())
		return
	}
	seasons, err := tmdb.New(key).SeriesSeasons(r.Context(), *title.TmdbID)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "tmdb_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, seasons)
}

// ImportEpisodes queues the season/episode import job.
func (h *AdminMetadata) ImportEpisodes(w http.ResponseWriter, r *http.Request) {
	title, ok := h.requireTmdbSeries(w, r)
	if !ok {
		return
	}
	var req struct {
		Seasons []int `json:"seasons"`
	}
	_ = httpx.Decode(r, &req) // empty body = all seasons
	jobID, err := h.jobs.EnqueueJobOnce(r.Context(), "import_episodes",
		tmdb.ImportEpisodesPayload{TitleID: title.ID, Seasons: req.Seasons},
		jobs.EnqueueOpts{Priority: 5})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]int64{"jobId": jobID})
}

func (h *AdminMetadata) requireTmdbSeries(w http.ResponseWriter, r *http.Request) (*store.Title, bool) {
	titleID := httpx.UUID(r, "id")
	if titleID == "" {
		httpx.NotFound(w)
		return nil, false
	}
	title, err := h.store.TitleByID(r.Context(), titleID)
	if err != nil {
		httpx.StoreErr(w, err)
		return nil, false
	}
	if title.Kind != "series" {
		httpx.BadRequest(w, "episode import only applies to series")
		return nil, false
	}
	if title.TmdbID == nil {
		httpx.Error(w, http.StatusPreconditionFailed, "no_tmdb_id",
			"link this show to TMDB first (Fetch from TMDB)")
		return nil, false
	}
	return title, true
}

// Apply queues the metadata fetch job for a title.
func (h *AdminMetadata) Apply(w http.ResponseWriter, r *http.Request) {
	titleID := httpx.UUID(r, "id")
	if titleID == "" {
		httpx.NotFound(w)
		return
	}
	var req struct {
		TmdbID int `json:"tmdbId"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.TmdbID <= 0 {
		httpx.BadRequest(w, "tmdbId is required")
		return
	}
	if _, err := h.store.TitleByID(r.Context(), titleID); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	jobID, err := h.jobs.EnqueueJob(r.Context(), "fetch_metadata",
		tmdb.FetchPayload{TitleID: titleID, TmdbID: req.TmdbID}, jobs.EnqueueOpts{Priority: 5})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]int64{"jobId": jobID})
}
