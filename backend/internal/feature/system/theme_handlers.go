package system

import (
	"context"
	"encoding/json"

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
	if raw, err := h.settings.Get(ctx, keyAppearance); err == nil && raw != nil {
		var a AppearanceSettings
		if json.Unmarshal(raw, &a) == nil && a.Accent != "" {
			accent = a.Accent
		}
	}
	return accent
}

type ThemeInfo struct {
	Accent string `json:"accent" doc:"Accent colour as a CSS hex value such as #e50914."`
}

type themeOutput struct{ Body ThemeInfo }

// Get is public so the accent applies on the login screen too.
func (h *Theme) Get(ctx context.Context, _ *struct{}) (*themeOutput, error) {
	return &themeOutput{Body: ThemeInfo{Accent: h.accent(ctx)}}, nil
}
