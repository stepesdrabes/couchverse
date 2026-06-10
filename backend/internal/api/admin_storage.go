package api

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"syscall"

	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type AdminStorage struct {
	store   *store.Store
	dataDir string
}

func NewAdminStorage(st *store.Store, dataDir string) *AdminStorage {
	return &AdminStorage{store: st, dataDir: dataDir}
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func (h *AdminStorage) Get(w http.ResponseWriter, r *http.Request) {
	var stat syscall.Statfs_t
	disk := map[string]int64{}
	if err := syscall.Statfs(h.dataDir, &stat); err == nil {
		total := int64(stat.Blocks) * int64(stat.Bsize)
		free := int64(stat.Bavail) * int64(stat.Bsize)
		disk["total"] = total
		disk["free"] = free
		disk["used"] = total - free
	}

	libraries, err := h.store.LibraryUsage(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"disk":      disk,
		"libraries": libraries,
		"cache": map[string]int64{
			"hls":     dirSize(filepath.Join(h.dataDir, "cache", "hls")),
			"images":  dirSize(filepath.Join(h.dataDir, "cache", "images")),
			"uploads": dirSize(filepath.Join(h.dataDir, "cache", "uploads")),
		},
	})
}

func (h *AdminStorage) Overview(w http.ResponseWriter, r *http.Request) {
	counts, err := h.store.Overview(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	pending, err := h.store.PendingJobCount(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	recent, err := h.store.ListJobs(r.Context(), "", 6)
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
	var rows []store.HomeRowConfig
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
