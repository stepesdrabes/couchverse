package api

import (
	"net/http"

	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type AdminJobs struct {
	store *store.Store
}

func NewAdminJobs(st *store.Store) *AdminJobs {
	return &AdminJobs{store: st}
}

func (h *AdminJobs) List(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.store.ListJobs(r.Context(), r.URL.Query().Get("status"), httpx.QueryInt(r, "limit", 50))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, jobs)
}

func (h *AdminJobs) Retry(w http.ResponseWriter, r *http.Request) {
	if err := h.store.RetryJob(r.Context(), httpx.ID(r, "id")); err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *AdminJobs) Cancel(w http.ResponseWriter, r *http.Request) {
	if err := h.store.CancelJob(r.Context(), httpx.ID(r, "id")); err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
