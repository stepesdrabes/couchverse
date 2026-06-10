package api

import (
	"encoding/json"
	"net/http"

	"couchverse/internal/httpx"
	"couchverse/internal/store"
	"couchverse/internal/transcode"
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
		// the transcoder silently ignores values it doesn't know — reject them here
		if key == "transcode" {
			var ts transcode.Settings
			if err := json.Unmarshal(value, &ts); err != nil {
				httpx.BadRequest(w, "invalid transcode settings")
				return
			}
			if err := ts.Validate(); err != nil {
				httpx.BadRequest(w, err.Error())
				return
			}
		}
		if err := h.store.SetSetting(r.Context(), key, value); err != nil {
			httpx.Internal(w, err)
			return
		}
	}
	h.Get(w, r)
}
