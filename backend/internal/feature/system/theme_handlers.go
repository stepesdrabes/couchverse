package system

import (
	"context"
	"encoding/json"
	"net/http"

	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

// defaultAccent is a Netflix-like red used until an admin picks one.
const defaultAccent = "#e50914"

type Theme struct {
	settings *settings.Store
}

func NewTheme(set *settings.Store) *Theme {
	return &Theme{settings: set}
}

// accent reads the configured accent colour (shared by the theme + favicon).
func (h *Theme) accent(ctx context.Context) string {
	accent := defaultAccent
	if raw, err := h.settings.Get(ctx, "appearance"); err == nil && raw != nil {
		var a struct {
			Accent string `json:"accent"`
		}
		if json.Unmarshal(raw, &a) == nil && a.Accent != "" {
			accent = a.Accent
		}
	}
	return accent
}

// Get is public so the accent applies on the login screen too.
func (h *Theme) Get(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"accent": h.accent(r.Context())})
}
