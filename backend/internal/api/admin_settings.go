package api

import (
	"encoding/json"
	"net/http"

	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type AdminSettings struct {
	store *store.Store
}

func NewAdminSettings(st *store.Store) *AdminSettings {
	return &AdminSettings{store: st}
}

func (h *AdminSettings) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.AllSettings(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, settings)
}

// Put merges the posted keys into settings.
func (h *AdminSettings) Put(w http.ResponseWriter, r *http.Request) {
	var req map[string]json.RawMessage
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	for key, value := range req {
		if err := h.store.SetSetting(r.Context(), key, value); err != nil {
			httpx.Internal(w, err)
			return
		}
	}
	h.Get(w, r)
}
