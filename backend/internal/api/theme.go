package api

import (
	"encoding/json"
	"net/http"

	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

// defaultAccent is a Netflix-like red used until an admin picks one.
const defaultAccent = "#e50914"

type Theme struct {
	store *store.Store
}

func NewTheme(st *store.Store) *Theme {
	return &Theme{store: st}
}

// Get is public so the accent applies on the login screen too.
func (h *Theme) Get(w http.ResponseWriter, r *http.Request) {
	accent := defaultAccent
	if raw, err := h.store.Setting(r.Context(), "appearance"); err == nil && raw != nil {
		var a struct {
			Accent string `json:"accent"`
		}
		if json.Unmarshal(raw, &a) == nil && a.Accent != "" {
			accent = a.Accent
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"accent": accent})
}
