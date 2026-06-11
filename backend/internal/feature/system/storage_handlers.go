package system

import (
	"net/http"
	"path/filepath"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
)

type AdminStorage struct {
	store   *Store
	jobs    *jobs.Store
	dataDir string
}

func NewAdminStorage(st *Store, jb *jobs.Store, dataDir string) *AdminStorage {
	return &AdminStorage{store: st, jobs: jb, dataDir: dataDir}
}

type storageCategory struct {
	Kind  string `json:"kind"`
	Bytes int64  `json:"bytes"`
}

// Get reports Couchverse's footprint relative to the space available to it,
// not the whole disk. The denominator ("budget") is Couchverse's own usage
// plus the disk's free space - i.e. everything Couchverse could occupy,
// excluding whatever else already lives on the disk. Usage is broken down by
// category (movies/series/music/transcodes/cache) for the segmented bar.
func (h *AdminStorage) Get(w http.ResponseWriter, r *http.Request) {
	diskTotal, free, ok := diskUsage(h.dataDir)
	if !ok {
		diskTotal, free = 0, 0
	}

	byKind, err := h.store.MediaUsageByKind(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	transcodes, err := h.store.TranscodeUsage(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	cache := media.DirSize(filepath.Join(h.dataDir, "cache", "images")) +
		media.DirSize(filepath.Join(h.dataDir, "cache", "uploads")) +
		media.DirSize(filepath.Join(h.dataDir, "cache", "sessions")) +
		media.DirSize(filepath.Join(h.dataDir, "cache", "frames"))

	categories := []storageCategory{
		{Kind: "movies", Bytes: byKind["movies"]},
		{Kind: "series", Bytes: byKind["series"]},
		{Kind: "music", Bytes: byKind["music"]},
		{Kind: "transcodes", Bytes: transcodes},
		{Kind: "cache", Bytes: cache},
	}
	var used int64
	for _, c := range categories {
		used += c.Bytes
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"diskTotal":  diskTotal,
		"free":       free,
		"used":       used,
		"budget":     used + free, // space available to Couchverse
		"categories": categories,
	})
}

func (h *AdminStorage) Overview(w http.ResponseWriter, r *http.Request) {
	counts, err := h.store.Overview(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	pending, err := h.jobs.PendingJobCount(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	recent, err := h.jobs.ListJobs(r.Context(), "", "", 6)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"counts":      counts,
		"pendingJobs": pending,
		"recentJobs":  recent,
	})
}

// HomeRows handlers (admin-curated home page rows)

func (h *AdminStorage) HomeRowsGet(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListHomeRows(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rows)
}

func (h *AdminStorage) HomeRowsPut(w http.ResponseWriter, r *http.Request) {
	var rows []HomeRowConfig
	if err := httpx.Decode(r, &rows); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	for _, row := range rows {
		switch row.Kind {
		case "continue_watching", "recently_added", "genre", "recently_played_music":
		default:
			httpx.BadRequest(w, "unknown row kind "+row.Kind)
			return
		}
		if row.Kind == "genre" && row.GenreID == nil {
			httpx.BadRequest(w, "genre rows need a genreId")
			return
		}
	}
	if err := h.store.ReplaceHomeRows(r.Context(), rows); err != nil {
		httpx.Internal(w, err)
		return
	}
	h.HomeRowsGet(w, r)
}
