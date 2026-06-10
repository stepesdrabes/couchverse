package jobs

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/httpx"
)

type AdminJobs struct {
	store *Store
}

func NewAdminJobs(st *Store) *AdminJobs {
	return &AdminJobs{store: st}
}

func (h *AdminJobs) MountAdmin(r chi.Router) {
	r.Get("/jobs", h.List)
	r.Post("/jobs/{id}/retry", h.Retry)
	r.Post("/jobs/{id}/cancel", h.Cancel)
}

func (h *AdminJobs) List(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.store.ListJobs(r.Context(), r.URL.Query().Get("status"),
		r.URL.Query().Get("mediaFileId"), httpx.QueryInt(r, "limit", 50))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, jobs)
}

func (h *AdminJobs) Retry(w http.ResponseWriter, r *http.Request) {
	if err := h.store.RetryJob(r.Context(), httpx.ID(r, "id")); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *AdminJobs) Cancel(w http.ResponseWriter, r *http.Request) {
	if err := h.store.CancelJob(r.Context(), httpx.ID(r, "id")); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
