package library

import (
	"net/http"
	"os"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/httpx"
)

type AdminLibraries struct {
	store *Store
	jobs  *jobs.Store
}

func NewAdminLibraries(st *Store, jb *jobs.Store) *AdminLibraries {
	return &AdminLibraries{store: st, jobs: jb}
}

func (h *AdminLibraries) List(w http.ResponseWriter, r *http.Request) {
	libs, err := h.store.ListLibraries(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, libs)
}

func (h *AdminLibraries) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
		Path string `json:"path"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if req.Name == "" || req.Path == "" || (req.Kind != "movies" && req.Kind != "series" && req.Kind != "music") {
		httpx.BadRequest(w, "name, path and kind (movies|series|music) are required")
		return
	}
	if info, err := os.Stat(req.Path); err != nil || !info.IsDir() {
		httpx.BadRequest(w, "path does not exist or is not a directory")
		return
	}
	lib, err := h.store.CreateLibrary(r.Context(), req.Name, req.Kind, req.Path, false)
	if err != nil {
		httpx.Error(w, http.StatusConflict, "conflict", "a library with this path already exists")
		return
	}
	httpx.JSON(w, http.StatusCreated, lib)
}

func (h *AdminLibraries) Delete(w http.ResponseWriter, r *http.Request) {
	lib, err := h.store.LibraryByID(r.Context(), httpx.ID(r, "id"))
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if lib.Managed {
		httpx.BadRequest(w, "managed libraries cannot be removed")
		return
	}
	if err := h.store.DeleteLibrary(r.Context(), lib.ID); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *AdminLibraries) Scan(w http.ResponseWriter, r *http.Request) {
	lib, err := h.store.LibraryByID(r.Context(), httpx.ID(r, "id"))
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	jobID, err := h.jobs.EnqueueJobOnce(r.Context(), "scan_library",
		ScanPayload{LibraryID: lib.ID}, jobs.EnqueueOpts{Priority: 10})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]any{"jobId": jobID})
}

// ScanAll enqueues a scan for every library.
func (h *AdminLibraries) ScanAll(w http.ResponseWriter, r *http.Request) {
	libs, err := h.store.ListLibraries(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	for _, lib := range libs {
		if _, err := h.jobs.EnqueueJobOnce(r.Context(), "scan_library",
			ScanPayload{LibraryID: lib.ID}, jobs.EnqueueOpts{Priority: 10}); err != nil {
			httpx.Internal(w, err)
			return
		}
	}
	httpx.JSON(w, http.StatusAccepted, map[string]any{"libraries": len(libs)})
}
